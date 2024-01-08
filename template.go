package main

import (
	"fmt"
	"io"
	"strings"
)

// ShortWriter allows simplified use of fmt.Fprintln() and fmt.Fprintf with an
// io.Writer to make the code easier to read, especially large blocks.
type ShortWriter struct {
	w io.Writer
}

// N ~ Newline
func (x *ShortWriter) N(s string) {
	fmt.Fprintln(x.w, s)
}

// F ~ Format
func (x *ShortWriter) F(format string, a ...any) {
	fmt.Fprintf(x.w, format, a...)
}

// WherePKs returns the where clause for the table provided, such as:
// "WHERE id=:id" or "WHERE artist=:artist, album=:album"
func WherePKs(t *Table) string {
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
func InsertColumns(t *Table, value bool) string {
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
func UpdateColumns(t *Table, value bool) string {
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
func UpsertConflictColumns(t *Table) string {
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

func Header(w ShortWriter, pkgName string) {
	w.F("package %s\n\n", pkgName)
	w.N(`
import (
	"context"
	"database/sql"
	"errors"
	"time"
)
`)
	w.N(`
// DB is the common interface for database operations and works with sqlx.DB, and sqlx.Tx.
// Use this IFF the function or method only performs one SQL operation to provide flexibility to the caller.
// If the function or method performs two or more SQL operations, use TX instead.
// This forces the caller to use a transaction and indicates that it's necessary to maintain data consistency.
type DB interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Get(dest interface{}, query string, args ...interface{}) error
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	NamedExec(query string, arg interface{}) (sql.Result, error)
	NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	Rebind(query string) string
	Select(dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
}

// TX is the common interface for database operations requiring a transaction and works with sqlx.Tx.
// Use this IFF the function or method performs two or more SQL operations.
// This forces the caller to use a transaction and indicates that it's necessary to maintain data consistency.
// If the function or method performs only one SQL operation, use DB instead.
type TX interface {
	DB
	Commit() error
	Rollback() error
}

var (
	ErrInsertAlreadyExists		= errors.New("cannot insert because the row already exists")
	ErrInsertMarkedForDeletion	= errors.New("cannot insert because the row has been deleted")
	ErrUpdateDoesNotExist		= errors.New("cannot update because the row does not exist")
	ErrUpdateMarkedForDeletion	= errors.New("cannot update because the row has been deleted")
	ErrUpsertMarkedForDeletion	= errors.New("cannot upsert because the row has been deleted")
)
`)
}

// TableToGo converts a Table to its Go-ORM layer.
func TableToGo(w ShortWriter, t *Table) {
	w.F("// %s represents a row from '%s'\n", t.GoName(), t.SQLName())
	if t.Comment != "" {
		w.F("// Schema Comment: %s\n", t.Comment)
	}
	w.F("type %s struct {\n", t.GoName())
	for _, c := range t.Columns {
		ColumnToGo(w, &c)
	}
	w.N(`	_exists, _deleted bool // In-memory-only metadata on this row's status in the DB`)
	w.N("}")

	w.N("// Exists in the database.")
	w.F("func (x *%s) Exists() bool {\n", t.GoName())
	w.N("	return x._exists")
	w.N("}")

	w.N("// Deleted from the database.")
	w.F("func (x *%s) Deleted() bool {\n", t.GoName())
	w.N("	return x._deleted")
	w.N("}")

	// INSERT
	w.N("// Insert this row into the database, returning an error on conflicts.")
	w.N("// Use Upsert if a conflict should not result in an error.")
	w.F("func (x *%s) Insert(ctx context.Context, dbc DB) error {\n", t.GoName())
	w.N(`	switch {`)
	w.N(`	case x._exists:`)
	w.N(`		return ErrInsertAlreadyExists`)
	w.N(`	case x._deleted:`)
	w.N(`		return ErrInsertMarkedForDeletion`)
	w.N(`	}`)
	if t.PrimaryKeyAutoIncrements() {
		w.N("	res, err := dbc.NamedExecContext(ctx, `")
	} else {
		w.N("	_, err := dbc.NamedExecContext(ctx, `")
	}
	w.F("		INSERT INTO %s (%s)\n", t.SQLName(), InsertColumns(t, false))
	w.F("		VALUES (%s)`, x)\n", InsertColumns(t, true))
	w.N("	if err != nil {")
	w.N("		return err")
	w.N("	}")
	if t.PrimaryKeyAutoIncrements() {
		w.N("	id, err := res.LastInsertId()")
		w.N("	if err != nil {")
		w.N("		return err")
		w.N("	}")
		w.N("	x.ID = id")
	}
	w.N("	x._exists = true")
	w.N("	return nil")
	w.N(`}`)

	if len(t.PrimaryKeys()) < 1 {
		w.N("// Update and Save methods not provided because we were unable to determine a PK\n\n")
	} else {
		// UPDATE
		w.N("// Update this row in the database.")
		w.F("func (x *%s) Update(ctx context.Context, dbc DB) error {\n", t.GoName())
		w.N("	switch {")
		w.N("	case !x._exists: // doesn't exist")
		w.N("		return ErrUpdateDoesNotExist")
		w.N("	case x._deleted: // deleted")
		w.N("		return ErrUpdateMarkedForDeletion")
		w.N("	}")
		w.N("	// update with primary key")
		w.N("	_, err := dbc.NamedExecContext(ctx, `")
		w.F("			UPDATE %s (%s)\n", t.SQLName(), UpdateColumns(t, false))
		w.F("			VALUES (%s)\n", UpdateColumns(t, true))
		w.F("			WHERE %s`, x)\n", WherePKs(t))
		w.N("	if err != nil {")
		w.N("		return err")
		w.N("	}")
		w.N("	return nil")
		w.N("}")

		// SAVE
		w.N("// Save this row to the database, either using Insert or Update.")
		w.F("func (x *%s) Save(ctx context.Context, dbc DB) error {", t.GoName())
		w.N("	if x.Exists() {")
		w.N("		return x.Update(ctx, dbc)")
		w.N("	}")
		w.N("	return x.Insert(ctx, dbc)")
		w.N("}")
	}

	// UPSERT FIXME: Not Done
	w.N("// Upsert this row to the database.")
	w.F("func (x *%s) Upsert(ctx context.Context, dbc DB) error {\n", t.GoName())
	w.N("	switch {")
	w.N("	case x._deleted: // deleted")
	w.N("		return ErrUpsertMarkedForDeletion")
	w.N("	}")
	w.N("	_, err := dbc.NamedExecContext(ctx, `")
	w.F("		INSERT INTO %s (%s)\n", t.SQLName(), InsertColumns(t, false))
	w.F("		VALUES (%s)\n", InsertColumns(t, true))
	w.F("		ON CONFLICT (%s)\n", UpsertConflictColumns(t))
	w.N("		DO UPDATE SET ip = EXCLUDED.ip, time=EXCLUDED.time`, x)") // TODO: Update this line
	w.N("	if err != nil {")
	w.N("		return err")
	w.N("	}")
	w.N("	// set exists")
	w.N("	x._exists = true")
	w.N("	return nil")
	w.N("}")

	// DELETE
	w.N("// Delete this row from the database.")
	w.F("func (x *%s) Delete(ctx context.Context, dbc DB) error {\n", t.GoName())
	w.N("	switch {")
	w.N("	case !x._exists: // doesn't exist")
	w.N("		return nil")
	w.N("	case x._deleted:")
	w.N("		return nil")
	w.N("	}")
	w.N("	_, err := dbc.NamedExecContext(ctx, `")
	w.F("			DELETE FROM %s\n", t.SQLName())
	w.F("			WHERE %s`, x)\n", WherePKs(t))
	w.N("	if err != nil {")
	w.N("		return err")
	w.N("	}")
	w.N("	x._deleted = true")
	w.N("	return nil")
	w.N("}")
}

// ColumnToGo converts a Column to its Go-ORM layer.
// Adds a comment about whether this column is Unique, has a Default, and the SQL comment
func ColumnToGo(w ShortWriter, c *Column) {
	commentParts := []string{}
	if c.PrimaryKey {
		commentParts = append(commentParts, "PK")
	}
	if c.Unique {
		commentParts = append(commentParts, "Unique")
	}
	if c.ForeignKey != nil {
		commentParts = append(commentParts, fmt.Sprintf("FK: %s.%s", c.ForeignKey.Table, c.ForeignKey.Column))
	}
	switch {
	case c.DefaultBool.Valid:
		commentParts = append(commentParts, fmt.Sprintf("Default: %v", c.DefaultBool.Bool))
	case c.DefaultString.Valid:
		commentParts = append(commentParts, fmt.Sprintf("Default: %v", c.DefaultString.String))
	case c.DefaultInt.Valid:
		commentParts = append(commentParts, fmt.Sprintf("Default: %v", c.DefaultInt.Int64))
	}
	if c.Comment != "" {
		commentParts = append(commentParts, fmt.Sprintf("-- %s", c.Comment))
	}
	comment := ""
	if len(commentParts) > 0 {
		comment = fmt.Sprintf("// %s", strings.Join(commentParts, ", "))
	}

	w.F("	%s %s %s\n", ToGoName(c.SQLName()), c.Type.ToGo(c.Nullable), comment)
}
