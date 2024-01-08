package parser

import (
	"fmt"

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
	Comment     string // Comment at the end of the CREATE TABLE definition if provided.
}

func (t *Table) GoName() string  { return t.goName }
func (t *Table) SQLName() string { return t.sqlName }
func (t *Table) SetSQLName(s string) {
	t.sqlName = s
	t.goName = name.ToGo(s)
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

// PrimaryKeys returns the column(s).
func (t *Table) PrimaryKeys() []*Column {
	pks := []*Column{}
	for _, col := range t.Columns {
		if col.PrimaryKey {
			return []*Column{&col} // TODO: Should we continue to search in case of an error where multiple are defined?
		}
		if col.CompositePrimaryKey {
			pks = append(pks, &col)
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
