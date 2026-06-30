package parser

import (
	"fmt"
	"strings"

	"github.com/joshsziegler/squirrel/name"
)

type Table struct {
	Strict      bool // Strict is true if enabled. Defaults to false.
	SchemaName  string
	sqlName     string
	goName      string
	Temp        bool
	IfNotExists bool
	Columns     []Column
	ForeignKeys []*ForeignKey // ForeignKeys defined on this table (inline single-column and table-level, possibly composite).
	// UniqueConstraints holds every UNIQUE constraint on the table, whether declared inline on a
	// column or as a table-level constraint, and whether single- or multi-column. Use
	// SingleColumnUnique to test individual-column uniqueness.
	UniqueConstraints []UniqueConstraint
	Comment           string // Comment at the end of the CREATE TABLE definition if provided.
}

// UniqueConstraint is a table-level UNIQUE constraint over one or more columns.
type UniqueConstraint struct {
	Name    string   // Name from a CONSTRAINT <name> prefix, or "" if unnamed.
	Columns []string // The constrained columns, in the order declared.
}

func (t *Table) GoName() string  { return t.goName }
func (t *Table) SQLName() string { return t.sqlName }
func (t *Table) SetSQLName(s string) {
	t.sqlName = s
	t.goName = name.ToGo(s)
}

// InternalUse returns true if this table is to be used by SQLite only.
//
// From SQLite Docs (https://www.sqlite.org/lang_createtable.html):
//
// Table names that begin with "sqlite_" are reserved for internal use. It is an
// error to attempt to create a table with a name that starts with "sqlite_".
func (t *Table) InternalUse() bool {
	return strings.HasPrefix(t.SQLName(), "sqlite_")
}

// SetPrimaryKeys takes a slice of column name(s) and verifies they exist and sets their metadata accordingly.
// The columns MUST be defined by the time this method is called.
func (t *Table) SetPrimaryKeys(colNames []string) error {
	if len(colNames) < 1 {
		return fmt.Errorf("must provide at least one column name for primary key(s)")
	}
	makeCompositePK := len(colNames) > 1
	// Make the error message friendlier by storing the columns we DO find in the same order they are specified
	foundCols := []string{}
	for _, colName := range colNames {
		for i, col := range t.Columns {
			if colName == col.SQLName() {
				if makeCompositePK {
					t.Columns[i].CompositePrimaryKey = true
				} else {
					t.Columns[i].PrimaryKey = true
				}
				foundCols = append(foundCols, col.SQLName())
				break // stop searching for this column
			}
		}
	}
	if len(colNames) != len(foundCols) {
		return fmt.Errorf("could not find all columns for the composite primary key (%v); found (%v)", colNames, foundCols)
	}
	return nil
}

// AddForeignKey validates a parsed foreign key and adds it to the table. Every local column named
// by the foreign key MUST be defined by the time this method is called, and the number of local and
// referenced columns must match.
func (t *Table) AddForeignKey(fk *ForeignKey) error {
	if len(fk.LocalColumns) != len(fk.Columns) {
		return fmt.Errorf("foreign key has %d local column(s) but %d referenced column(s): %v -> %v",
			len(fk.LocalColumns), len(fk.Columns), fk.LocalColumns, fk.Columns)
	}
	for _, localColumn := range fk.LocalColumns {
		if !t.hasColumn(localColumn) {
			return fmt.Errorf("foreign key references unknown column %q", localColumn)
		}
	}
	t.ForeignKeys = append(t.ForeignKeys, fk)
	return nil
}

// AddUniqueConstraint records a table-level UNIQUE constraint. Every named column MUST be defined
// by the time this is called. A single-column constraint sets that column's Unique flag (equivalent
// to an inline UNIQUE); a multi-column constraint is appended to UniqueConstraints.
func (t *Table) AddUniqueConstraint(name string, colNames []string) error {
	if len(colNames) < 1 {
		return fmt.Errorf("UNIQUE constraint must name at least one column")
	}
	for _, colName := range colNames {
		if !t.hasColumn(colName) {
			return fmt.Errorf("UNIQUE constraint references unknown column %q", colName)
		}
	}
	t.UniqueConstraints = append(t.UniqueConstraints, UniqueConstraint{Name: name, Columns: colNames})
	return nil
}

// SingleColumnUnique reports whether the named column has a single-column UNIQUE constraint.
func (t *Table) SingleColumnUnique(colName string) bool {
	for _, uc := range t.UniqueConstraints {
		if len(uc.Columns) == 1 && uc.Columns[0] == colName {
			return true
		}
	}
	return false
}

// hasColumn returns true if the table has a column with the given SQL name.
func (t *Table) hasColumn(sqlName string) bool {
	for i := range t.Columns {
		if t.Columns[i].SQLName() == sqlName {
			return true
		}
	}
	return false
}

// PrimaryKeys returns the column(s).
//
//	WARNING:: SQLite Docs (https://www.sqlite.org/lang_createtable.html):
//
//	According to the SQL standard, PRIMARY KEY should always imply NOT NULL. Unfortunately, due
//	to a bug in some early versions, this is not the case in SQLite. Unless the column is an
//	INTEGER PRIMARY KEY or the table is a WITHOUT ROWID table or a STRICT table or the column is
//	declared NOT NULL, SQLite allows NULL values in a PRIMARY KEY column. SQLite could be fixed
//	to conform to the standard, but doing so might break legacy applications. Hence, it has been
//	decided to merely document the fact that SQLite allows NULLs in most PRIMARY KEY columns.
func (t *Table) PrimaryKeys() []*Column {
	pks := []*Column{}
	for i, col := range t.Columns {
		if col.PrimaryKey {
			return []*Column{&col} // TODO: Should we continue to search in case of an error where multiple are defined?
		}
		if col.CompositePrimaryKey {
			pks = append(pks, &t.Columns[i])
		}
	}
	return pks
}

// PrimaryKeyAutoIncrements returns true if there is a single Primary Key column
// -- it is not a composite PK -- and it auto-increments (e.g. rowid, or ID).
func (t *Table) PrimaryKeyAutoIncrements() bool {
	pks := t.PrimaryKeys()
	return len(pks) == 1 && pks[0].AutoIncrement()
}
