package parser

type OnFkAction int // OnFkAction represent all possible actions to take on a Foreign KEY (DELETE|UPDATE)

const (
	NoAction OnFkAction = iota
	SetNull
	SetDefault
	Cascade
	Restrict
)

func (a OnFkAction) String() string {
	switch a {
	case NoAction:
		return "NO ACTION"
	case SetNull:
		return "SET NULL"
	case SetDefault:
		return "SET DEFAULT"
	case Cascade:
		return "CASCADE"
	case Restrict:
		return "RESTRICT"
	default:
		return "UNDEFINED"
	}
}

type ForeignKey struct {
	Table    string
	Column   string
	OnUpdate OnFkAction // OnUpdate action to take (e.g. none, Set Null, Set Default, etc.)
	OnDelete OnFkAction // OnDelete action to take (e.g. none, Set Null, Set Default, etc.)
}
