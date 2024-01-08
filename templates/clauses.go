package templates

import (
	"fmt"
	"strings"

	"github.com/joshsziegler/squirrel/parser"
)

// WherePKs returns the where clause for the table provided, such as:
// "WHERE id=:id" or "WHERE artist=:artist, album=:album"
func WherePKs(t *parser.Table) string {
	res := ""
	pks := t.PrimaryKeys()
	for i, col := range pks {
		if i > 0 {
			res += ", "
		}
		res += fmt.Sprintf("%s=:%s", col.SQLName(), col.SQLName())
	}
	return res
}

// InsertColumns returns the column list for INSERT or VALUES, depending on the last param.
func InsertColumns(t *parser.Table, value bool) string {
	res := ""
	for _, col := range t.Columns {
		if col.DBGenerated() {
			continue // skip
		}
		if res != "" {
			res += ", "
		}
		if value {
			res += ":"
		}
		res += col.SQLName()
	}
	return res
}

// UpdateColumns returns the column list for UPDATE or VALUES, depending on the last param.
// TODO: Update update_at if used!
func UpdateColumns(t *parser.Table, value bool) string {
	res := ""
	for _, col := range t.Columns {
		if col.DBGenerated() {
			continue // skip
		}
		if res != "" {
			res += ", "
		}
		if value {
			res += ":"
		}
		res += col.SQLName()
	}
	return res
}

// UpsertUpdateColumns returns the column list for DO UPDATE SET.
// TODO: Update update_at if used!
func UpsertUpdateColumns(t *parser.Table) string {
	res := ""
	for _, col := range t.Columns {
		if col.DBGenerated() {
			continue // skip
		}
		if res != "" {
			res += ", "
		}
		res += fmt.Sprintf("%s=EXCLUDED.%s", col.SQLName(), col.SQLName())
	}
	return res
}

// UpsertConflictColumns returns the column list for an UPSERT's ON CONFLICT clause.
//
// TODO: Exclude auto-increment PKs such as row_id or ID.
//
// From the SQLite Docs:
//
//	The UPSERT processing happens only for uniqueness constraints. A "uniqueness constraint" is an
//	explicit UNIQUE or PRIMARY KEY constraint within the CREATE TABLE statement, or a unique
//	index. UPSERT does not intervene for failed NOT NULL, CHECK, or foreign key constraints or for
//	constraints that are implemented using triggers.
//		~ https://www.sqlite.org/lang_upsert.html
func UpsertConflictColumns(t *parser.Table) string {
	cols := []string{}
	pks := t.PrimaryKeys()
	for _, col := range pks {
		if !col.AutoIncrement() { // Do not include auto-incrementing columns (e.g. rowid)
			cols = append(cols, col.SQLName())
		}
	}
	for _, col := range t.Columns {
		if col.Unique {
			cols = append(cols, col.SQLName())
		}
	}
	return strings.Join(cols, ", ")
}
