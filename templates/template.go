package templates

import (
	"fmt"
	"io"
	"strings"

	"github.com/joshsziegler/squirrel/parser"
)

func Header(writer io.Writer, pkgName string) {
	w := NewShortWriter(writer)

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

// Table converts a Table to its Go-access-layer.
func Table(writer io.Writer, t *parser.Table) {
	w := NewShortWriter(writer)

	w.F("// %s represents a row from '%s'\n", t.GoName(), t.SQLName())
	if t.Comment != "" {
		w.F("// Schema Comment: %s\n", t.Comment)
	}
	if len(t.PrimaryKeys()) < 1 {
		w.N("//")
		w.N("// Update, Save, Upsert, and Delete methods not provided because we were unable to determine a PK")
	}
	w.F("type %s struct {\n", t.GoName())
	for _, c := range t.Columns {
		columnToGo(w, &c)
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

	if len(t.PrimaryKeys()) > 0 {
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

		// UPSERT
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
		w.F("		DO UPDATE SET %s`, x)\n", UpsertUpdateColumns(t))
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
}

// columnToGo converts a Column to its Go-ORM layer.
// Adds a comment about whether this column is Unique, has a Default, and the SQL comment
func columnToGo(w *ShortWriter, c *parser.Column) {
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

	w.F("	%s %s %s\n", c.GoName(), c.Type.ToGo(c.Nullable), comment)
}
