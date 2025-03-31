package querybuilder

type CondType int

const (
	CondTypeAnd CondType = iota + 1
	CondTypeOr
)

func (c CondType) ToString() string {
	switch c {
	case CondTypeAnd:
		return "AND"
	case CondTypeOr:
		return "OR"
	default:
		return ""
	}
}

type Condition struct {
	condType   CondType
	clause     string
	args       []interface{}
	subCond    []Condition
	isSubGroup bool
}
