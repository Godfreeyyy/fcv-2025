package querybuilder

type JoinType int

const (
	JoinTypeInner JoinType = iota + 1
	JoinTypeLeft
	JoinTypeRight
)

func (j JoinType) ToString() string {
	switch j {
	case JoinTypeInner:
		return "INNER JOIN"
	case JoinTypeLeft:
		return "LEFT JOIN"
	case JoinTypeRight:
		return "RIGHT JOIN"
	default:
		return ""
	}
}

type join struct {
	joinType JoinType
	table    string
	alias    string
	on       string
}
