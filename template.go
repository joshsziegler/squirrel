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

func Header(w ShortWriter, pkgName string) {
	w.F("package %s\n\n", pkgName)
	w.N(`
import (
	"context"
	"database/sql"
	"time"

	"github.com/ansel1/merry/v2"
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
	ErrAlreadyExists           = merry.New("already exists", merry.NoCaptureStack())
	ErrDoesNotExist            = merry.New("does not exist", merry.NoCaptureStack())
	ErrMarkedForDeletion       = merry.New("marked for deletion", merry.NoCaptureStack())
	ErrInsertAlreadyExists     = merry.Prepend(ErrAlreadyExists, "cannot insert", merry.NoCaptureStack())
	ErrInsertMarkedForDeletion = merry.Prepend(ErrMarkedForDeletion, "cannot insert", merry.NoCaptureStack())
	ErrUpdateDoesNotExist      = merry.Prepend(ErrDoesNotExist, "cannot update", merry.NoCaptureStack())
	ErrUpdateMarkedForDeletion = merry.Prepend(ErrMarkedForDeletion, "cannot update", merry.NoCaptureStack())
	ErrUpsertMarkedForDeletion = merry.Prepend(ErrMarkedForDeletion, "cannot upsert", merry.NoCaptureStack())
)
`)
}

// TableToGo converts a Table to its Go-ORM layer.
func TableToGo(w ShortWriter, t *Table) {
	goName := ToGoName(t.Name)
	if t.Comment != "" {
		w.F("// %s %s\n", goName, t.Comment)
	}
	w.F("type %s struct {\n", goName)
	for _, c := range t.Columns {
		ColumnToGo(w, &c)
	}
	w.N(`    _exists, _deleted bool // DB Metadata `)
	w.N("}")

	w.N("// Exists in the database.")
	w.F("func (x *%s) Exists() bool {\n", goName)
	w.N("    return x._exists")
	w.N("}")

	w.N("// Deleted from the database.")
	w.F("func (x *%s) Deleted() bool {\n", goName)
	w.N("	return x._deleted")
	w.N("}")

	// Handle Insert, Update, Upsert, Delete
	pk := t.PrimaryKey()
	w.F("func (x *%s) Insert(ctx context.Context, dbc DB) error {\n", goName)
	w.N(`    switch {`)
	w.N(`    case x._exists:`)
	w.N(`        return merry.Wrap(ErrInsertAlreadyExists)`)
	w.N(`    case x._deleted:`)
	w.N(`        return merry.Wrap(ErrInsertMarkedForDeletion)`)
	w.N(`    }`)
	if pk != nil && pk.Name == "ID" { // TODO: Handle auto-generated 'rowid'
		w.N("    res, err := dbc.NamedExecContext(ctx, `")
	} else {
		w.N("    _, err := dbc.NamedExecContext(ctx, `")
	}
	w.F("        INSERT INTO %s (", t.Name)
	for i, col := range t.Columns {
		if i > 0 {
			w.F(", %s", col.Name)
		} else {
			w.F("%s", col.Name)
		}
	}
	w.N(")")
	w.F("        VALUES (")
	for i, col := range t.Columns {
		if i > 0 {
			w.F(", :%s", col.Name)
		} else {
			w.F(":%s", col.Name)
		}
	}
	w.N(")`, x)")
	w.N("if err != nil {")
	w.N("	return err")
	w.N("}")
	if pk != nil && pk.Name == "ID" { // TODO: Handle auto-generated 'rowid'
		w.N("id, err := res.LastInsertId()")
		w.N("if err != nil {")
		w.N("	return err")
		w.N("}")
		w.N("x.ID = id")
	}
	w.N("x._exists = true")
	w.N("return nil")
	w.N(`}`)

	w.N("// Update this row in the database.")
	w.F("func (x *%s) Update(ctx context.Context, dbc DB) error {\n", goName)
	w.N("switch {")
	w.N("case !x._exists: // doesn't exist")
	w.N("	return merry.Wrap(ErrUpdateDoesNotExist)")
	w.N("case x._deleted: // deleted")
	w.N("	return ErrUpdateMarkedForDeletion")
	w.N("}")
	w.N("// update with primary key")
	w.N("_, err := dbc.NamedExecContext(ctx, `")
	w.F("        UPDATE %s (", t.Name)
	for i, col := range t.Columns { // TODO: Exclude DB-generated fields
		if col.PrimaryKey {
			continue // skip
		}
		if i > 0 {
			w.F(", %s", col.Name)
		} else {
			w.F("%s", col.Name)
		}
	}
	w.N(")")
	w.F("        VALUES (")
	for i, col := range t.Columns { // TODO: Exclude DB-generated fields
		if col.PrimaryKey {
			continue // skip
		}
		if i > 0 {
			w.F(", :%s", col.Name)
		} else {
			w.F(":%s", col.Name)
		}
	}
	w.N(")")
	w.N("        WHERE id = :id`, x)") // TODO: How to determine PK, or composite PK?
	w.N("if err != nil {")
	w.N("	return err")
	w.N("}")
	w.N("return nil")
	w.N("}")

	w.N("// Save this row to the database, either using Insert or Update.")
	w.F("func (x *%s) Save(ctx context.Context, dbc DB) error {", goName)
	w.N("    if x.Exists() {")
	w.N("        return x.Update(ctx, dbc)")
	w.N("    }")
	w.N("    return x.Insert(ctx, dbc)")
	w.N("}")

	// TODO: Add UPSERT

	w.N("// Delete this row from the database.")
	w.F("func (x *%s) Delete(ctx context.Context, dbc DB) error {\n", goName)
	w.N("switch {")
	w.N("case !x._exists: // doesn't exist")
	w.N("	return nil")
	w.N("case x._deleted:")
	w.N("	return nil")
	w.N("}")
	w.N("_, err := dbc.NamedExecContext(ctx, `")
	w.F("        DELETE FROM %s\n", t.Name)
	w.F("        WHERE id = :id`, x)\n") // TODO: How to determine PK, or composite PK?
	w.N("if err != nil {")
	w.N("	return err")
	w.N("}")
	w.N("x._deleted = true")
	w.N("return nil")
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

	w.F("    %s %s %s\n", ToGoName(c.Name), c.Type.ToGo(c.Nullable), comment)
}
