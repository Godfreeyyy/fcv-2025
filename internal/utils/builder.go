package querybuilder

import (
	"fmt"
	"strings"
)

type QueryBuilder interface {
	Select(cols ...string) QueryBuilder
	From(table string) QueryBuilder
	Into(table string) QueryBuilder
	Where(clause string, args ...interface{}) QueryBuilder

	Or(clause string, args ...interface{}) QueryBuilder
	And(clause string, args ...interface{}) QueryBuilder

	AndGroup(fn func(qb QueryBuilder)) QueryBuilder
	OrGroup(fn func(qb QueryBuilder)) QueryBuilder

	OrderBy(col string, asc bool) QueryBuilder
	GroupBy(cols ...string) QueryBuilder
	Join(joinType JoinType, table, alias, on string) QueryBuilder

	Insert(cols ...string) QueryBuilder

	Values(values ...interface{}) QueryBuilder

	Update(table string, data UpdateData) QueryBuilder
	Delete(table string) QueryBuilder
	Build() (string, []interface{})

	DoNothing() QueryBuilder
	DoUpdate(cols ...string) QueryBuilder
	Set(doUpdate map[string]interface{}) QueryBuilder
	SetExclude(cols ...string) QueryBuilder
	OnConflict(cols ...string) QueryBuilder

	getConditions() []Condition
}

type queryBuilder struct {
	table         string
	cols          []string
	conditions    []Condition
	joins         []join
	values        InsertRows
	updateData    UpdateData
	groupBy       []string
	orderBy       []string
	isDelete      bool
	isInsert      bool
	onConflictSet map[string]interface{}
	setCols       []string
	excludeCols   []string
	onConflict    []string
	schema        string
}

func (q *queryBuilder) DoNothing() QueryBuilder {
	q.onConflictSet = nil
	return q
}

func (q *queryBuilder) DoUpdate(cols ...string) QueryBuilder {
	q.setCols = cols
	return q
}
func (q *queryBuilder) Set(doUpdate map[string]interface{}) QueryBuilder {
	q.onConflictSet = doUpdate
	return q
}

func (q *queryBuilder) SetExclude(cols ...string) QueryBuilder {
	q.excludeCols = cols
	return q
}

func (q *queryBuilder) OnConflict(cols ...string) QueryBuilder {
	q.onConflict = cols
	return q
}

func (q *queryBuilder) getConditions() []Condition {
	return q.conditions
}

func (q *queryBuilder) Select(cols ...string) QueryBuilder {
	q.cols = append(q.cols, cols...)
	return q
}

func (q *queryBuilder) Columns(cols ...string) QueryBuilder {
	q.cols = append(q.cols, cols...)
	return q
}

func (q *queryBuilder) Insert(cols ...string) QueryBuilder {
	q.cols = cols
	return q
}

func (q *queryBuilder) Values(values ...interface{}) QueryBuilder {
	q.values = append(q.values, values)
	return q
}

func (q *queryBuilder) Update(table string, data UpdateData) QueryBuilder {
	q.table = table
	q.updateData = data
	return q
}

func (q *queryBuilder) Delete(table string) QueryBuilder {
	q.table = table
	return q
}

func (q *queryBuilder) Or(clause string, args ...interface{}) QueryBuilder {
	cond := Condition{
		condType: CondTypeOr,
		clause:   clause,
		args:     args,
	}
	q.conditions = append(q.conditions, cond)

	return q
}

func (q *queryBuilder) And(clause string, args ...interface{}) QueryBuilder {
	cond := Condition{
		condType: CondTypeAnd,
		clause:   clause,
		args:     args,
	}
	q.conditions = append(q.conditions, cond)

	return q
}

func (q *queryBuilder) group(condType CondType, fn func(qb QueryBuilder)) QueryBuilder {
	rawQueryBuilder := NewQueryBuilder(q.schema)
	fn(rawQueryBuilder)
	q.conditions = append(q.conditions, Condition{
		condType:   condType,
		subCond:    rawQueryBuilder.getConditions(),
		isSubGroup: true,
	})
	return q

}

func (q *queryBuilder) AndGroup(fn func(qb QueryBuilder)) QueryBuilder {
	return q.group(CondTypeAnd, fn)
}

func (q *queryBuilder) OrGroup(fn func(qb QueryBuilder)) QueryBuilder {
	return q.group(CondTypeOr, fn)
}

func (q *queryBuilder) OrderBy(col string, asc bool) QueryBuilder {
	orderVector := "ASC"
	if !asc {
		orderVector = "DESC"
	}
	q.orderBy = append(q.orderBy, fmt.Sprintf("%s %s", col, orderVector))
	return q
}

func (q *queryBuilder) GroupBy(cols ...string) QueryBuilder {
	q.groupBy = append(q.groupBy, cols...)
	return q
}

func (q *queryBuilder) From(table string) QueryBuilder {
	q.table = table
	return q
}

func (q *queryBuilder) Into(table string) QueryBuilder {
	q.table = table
	return q
}

func (q *queryBuilder) Where(clause string, args ...interface{}) QueryBuilder {
	cond := Condition{
		condType: CondTypeAnd,
		clause:   clause,
		args:     args,
	}
	q.conditions = append(q.conditions, cond)

	return q
}

func (q *queryBuilder) Join(joinType JoinType, table, alias, on string) QueryBuilder {
	q.joins = append(q.joins, join{
		joinType: joinType,
		table:    table,
		alias:    alias,
		on:       on,
	})
	return q
}

func buildCondition(condition []Condition) (string, []interface{}) {
	parts := make([]string, 0)
	args := make([]interface{}, 0)
	var clause string

	for i, cond := range condition {
		if i > 0 {
			parts = append(parts, cond.condType.ToString())
		}
		if cond.isSubGroup && len(cond.subCond) > 0 {
			clause, args = buildCondition(cond.subCond)
			parts = append(parts, fmt.Sprintf("(%s)", clause))
			args = append(args, cond.args...)
			args = append(args, args...)
			continue
		}

		parts = append(parts, cond.clause)
		args = append(args, cond.args...)
	}

	return strings.Join(parts, " "), args
}

func (q *queryBuilder) Build() (string, []interface{}) {
	isInsert := len(q.values) > 0
	isUpdate := len(q.updateData) > 0
	isDelete := q.table != "" && len(q.conditions) > 0 && q.isDelete
	isSelect := len(q.cols) > 0
	operationOrder := []string{"insert", "update", "delete", "select"}
	opByCond := map[string]bool{
		"insert": isInsert,
		"update": isUpdate,
		"delete": isDelete,
		"select": isSelect,
	}
	for _, op := range operationOrder {
		if opByCond[op] {
			switch op {
			case "insert":
				return q.buildInsert()
			case "update":
				return q.buildUpdate()
			case "delete":
				return q.buildDelete()
			}
		}
	}

	return q.buildSelect()
}

func (q *queryBuilder) buildSelect() (string, []interface{}) {
	query := fmt.Sprintf("SELECT %s FROM %s.%s", strings.Join(q.cols, ", "), q.schema, q.table)
	if len(q.joins) > 0 {
		for _, j := range q.joins {
			query += fmt.Sprintf(" %s %s %s ON %s", j.joinType.ToString(), j.table, j.alias, j.on)
		}
	}

	var args []interface{}

	if len(q.conditions) > 0 {
		condition, condArgs := buildCondition(q.conditions)
		query += fmt.Sprintf(" WHERE %s", condition)
		args = append(args, condArgs...)
	}

	if len(q.groupBy) > 0 {
		query += fmt.Sprintf(" GROUP BY %s", strings.Join(q.groupBy, ", "))
	}

	if len(q.orderBy) > 0 {
		query += fmt.Sprintf(" ORDER BY %s", strings.Join(q.orderBy, ", "))
	}

	return query, args
}

func (q *queryBuilder) buildInsert() (string, []interface{}) {
	var (
		valueTuples = make([]string, len(q.values))
		firsRows    = q.values[0]
		numOfParam  = len(firsRows)
		args        = make([]interface{}, 0)
		colValStack = make([]string, 0, numOfParam)
	)
	query := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES ", q.schema, q.table, strings.Join(q.cols, ", "))

	if len(q.values) == 0 {
		goto InvalidQuery
	}

	if numOfParam == 0 || (len(q.cols) != numOfParam) {
		goto InvalidQuery
	}

	for i, row := range q.values {
		if len(row) != numOfParam {
			goto InvalidQuery
		}
		for _, val := range row {
			args = append(args, val)
			colValStack = append(colValStack, "?")
			valueTuples[i] = fmt.Sprintf("(%s)", strings.Join(colValStack, ", "))
		}
		colValStack = colValStack[:0]
	}

	query += strings.Join(valueTuples, ", ")

	if len(q.onConflict) > 0 {
		query += fmt.Sprintf(" ON CONFLICT (%s) ", strings.Join(q.onConflict, ", "))
		isOnConflictMode := (q.onConflictSet != nil || len(q.onConflictSet) > 0) || len(q.excludeCols) > 0
		if !isOnConflictMode {
			query += " DO NOTHING "
			return query, args
		}
		query += " DO UPDATE SET "
		for _, col := range q.setCols {
			query += fmt.Sprintf("%s = ?, ", col)
			if v, ok := q.onConflictSet[col]; ok {
				args = append(args, v)
			} else {
				goto InvalidQuery
			}
		}

		for _, excludeCol := range q.excludeCols {
			query += fmt.Sprintf("%s = EXCLUDED.%s, ", excludeCol, excludeCol)
		}

		query = strings.TrimSuffix(query, ", ")
	}
	return query, args

InvalidQuery:
	return "", nil
}

func (q *queryBuilder) buildUpdate() (string, []interface{}) {
	if len(q.cols) != len(q.updateData) {
		fmt.Println("columns and update data mismatch")
	}
	base := fmt.Sprintf("UPDATE %s.%s SET ", q.schema, q.table)
	setClause := make([]string, 0, len(q.updateData))
	args := make([]interface{}, 0, len(q.updateData))
	for col, val := range q.updateData {
		setClause = append(setClause, fmt.Sprintf("%s = ?", col))
		args = append(args, val)
	}
	query := base + strings.Join(setClause, ", ")

	for _, j := range q.joins {
		query += fmt.Sprintf(" %s %s %s ON %s", j.joinType.ToString(), j.table, j.alias, j.on)
	}

	if len(q.conditions) > 0 {
		condition, condArgs := buildCondition(q.conditions)
		query += fmt.Sprintf(" WHERE %s", condition)
		args = append(args, condArgs...)
	}

	if len(q.groupBy) > 0 {
		query += fmt.Sprintf(" GROUP BY %s", strings.Join(q.groupBy, ", "))
	}

	if len(q.orderBy) > 0 {
		query += fmt.Sprintf(" ORDER BY %s", strings.Join(q.orderBy, ", "))
	}

	return query, args
}

func (q *queryBuilder) buildDelete() (string, []interface{}) {
	panic("implement me")
}

func NewQueryBuilder(schema string) QueryBuilder {
	return &queryBuilder{
		schema: schema,
	}
}

func init() {

}
