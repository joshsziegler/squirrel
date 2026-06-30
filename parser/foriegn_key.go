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

// ForeignKey represents a single foreign-key constraint, which may span one or more columns.
// Both inline (single-column) and table-level (possibly composite) foreign keys use this type, and
// are stored on the Table rather than on individual Columns. LocalColumns and Columns are paired
// positionally: LocalColumns[i] in this table references Columns[i] in the foreign Table.
type ForeignKey struct {
	Name         string     // Name from a CONSTRAINT <name> prefix (table-level only), or "" if unnamed.
	Table        string     // Table is the referenced (foreign) table.
	LocalColumns []string   // LocalColumns are the column(s) in THIS table, in order.
	Columns      []string   // Columns are the referenced column(s) in Table, paired positionally with LocalColumns.
	OnUpdate     OnFkAction // OnUpdate action to take (e.g. none, Set Null, Set Default, etc.)
	OnDelete     OnFkAction // OnDelete action to take (e.g. none, Set Null, Set Default, etc.)
}

// Composite returns true if this foreign key spans more than one column.
func (fk *ForeignKey) Composite() bool {
	return len(fk.LocalColumns) > 1
}
