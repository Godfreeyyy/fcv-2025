package querybuilder

type InsertRows [][]interface{} // multiple Rows

func (rows InsertRows) Columns() []string {
	if len(rows) == 0 {
		return []string{}
	}
	firstRow := rows[0]
	columns := make([]string, len(firstRow))
	//for i := range firstRow {
	//	columns[i] = nil
	//}
	return columns
}
