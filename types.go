package main

import (
	"database/sql"
	"fmt"
	"io"
	"strings"
)

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

func NewTokens(tokens []string) *Tokens {
	return &Tokens{
		i:      0,
		tokens: tokens,
		length: len(tokens),
	}
}

type Tokens struct {
	tokens []string
	i      int
	length int
}

// Next token or an empty string if there are no more tokens.
// This does not "take" the token, so further calls will return the same token.
func (t *Tokens) Next() string {
	if t.i < t.length {
		return t.tokens[t.i]
	}
	return ""
}

// NextN at the N next tokens concatenated together.
// This does not "take" the token, so further calls will return the same tokens.
// If there are not n more tokens, returns the remainder (e.g. two tokens returned when three were requested).
func (t *Tokens) NextN(n int) string {
	s := make([]string, 0)
	for j := 0; j < n; j++ {
		if t.i+j >= t.length {
			break
		}
		s = append(s, t.tokens[t.i+j])
	}
	return strings.Join(s, " ")
}

// Peek at the nth token from the current index or return an empty string if it does not exist.
// This does not "take" the token, so further calls will return the same token.
func (t *Tokens) Peek(n int) string {
	j := t.i + n
	if j < t.length {
		return t.tokens[j]
	}
	return ""
}

// Take the next token (i.e. claim/consume it).
// Does nothing if there are no more tokens.
func (t *Tokens) Take() string {
	token := ""
	if t.i < t.length {
		token = t.tokens[t.i]
		t.i += 1
	}
	return token
}

// Take the next N tokens (i.e. claim/consume them).
// Stops when there are no more tokens to take.
func (t *Tokens) TakeN(n int) {
	j := t.i + n
	if j < t.length {
		t.i += n
	}
	return
}

// Return the last token (opposite of Take()).
func (t *Tokens) Return() {
	if t.i == 0 {
		fmt.Println("Error: Tried returning past zero")
		return
	}
	t.i -= 1
}

// Return the last N token (opposite of TakeN()).
func (t *Tokens) ReturnN(n int) {
	if t.i-n < 0 {
		fmt.Println("Error: Tried returning past zero")
		return
	}
	t.i -= n
}

// Datatype is the domain-type representing out SQLite-to-Go type mapping.
type Datatype string

const (
	UNKNOWN  Datatype = "Unknown"
	INT               = "int"
	BOOL              = "bool"
	FLOAT             = "float"
	TEXT              = "text"
	BLOB              = "blob"
	DATETIME          = "datetime"
)

// ToGo converts this domain type to the equivalent Go type.
// Uses Int64 for all integers and Float64 for all floats.
func (t Datatype) ToGo(nullable bool) string {
	switch t {
	case INT:
		if nullable {
			return "sql.NullInt64"
		}
		return "int64"
	case BOOL:
		if nullable {
			return "sql.NullBool"
		}
		return "bool"
	case FLOAT:
		if nullable {
			return "sql.NullFloat64"
		}
		return "float64"
	case TEXT:
		if nullable {
			return "sql.NullString"
		}
		return "string"
	case BLOB:
		return "[]byte" // TODO: Is this the right Go type for unmarshalling from SQL?
	case DATETIME:
		if nullable {
			return "sql.NullTime"
		}
		return "time.Time"
	default: // Unknown datatype
		panic("unknown datatype")
	}
}

// FromSQL to internal domain type.
//
// SQLite's STRICT data types (i.e. INT, INTEGER, REAL, TEXT, BLOB, ANY).
// SQLite Docs: https://www.sqlite.org/datatype3.html
// mattn/go-sqlit3 Docs: https://pkg.go.dev/github.com/mattn/go-sqlite3#hdr-Supported_Types
func FromSQL(s string) (Datatype, error) {
	switch s {
	case "INT", "INTEGER":
		return INT, nil
	case "BOOL", "BOOLEAN": // SQLite does not have a bool or boolean type and represents them as integers (0 and 1) internally.
		return BOOL, nil
	case "FLOAT": // TODO: This isn't a valid SQLite type.
		fallthrough
	case "REAL":
		return FLOAT, nil
	case "TEXT":
		return TEXT, nil
	case "BLOB":
		fallthrough
	case "ANY":
		return BLOB, nil
	case "DATETIME":
		fallthrough
	case "TIMESTAMP":
		return DATETIME, nil
	default:
		// TODO: IF strict, return an error. Otherwise use "ANY"
		return "", fmt.Errorf("\"%s\" is not a valid SQLite column type", s)
	}
}

type Table struct {
	SchemaName  string
	Name        string
	Temp        bool
	IfNotExists bool
	Columns     []Column
	Comment     string // Comment at the end of the CREATE TABLE definition if provided.
}

func (t *Table) ORM(w ShortWriter) {
	goName := ToGoName(t.Name)
	if t.Comment != "" {
		w.F("// %s %s\n", goName, t.Comment)
	}
	w.F("type %s struct {\n", goName)
	for _, c := range t.Columns {
		c.ORM(w)
	}
	w.N(`    _exists, _deleted bool // DB Metadata `)
	w.N("}")

	w.N("// Exists in the database.")
	w.F("func (x *%s) Exists() bool {\n", goName)
	w.N("    return x._exists")
	w.N("}")

	w.N("// Deleted when this row has been marked for deletion from the database.")
	w.F("func (x *%s) Deleted() bool {", goName)
	w.N("	return x._deleted")
	w.N("}")

	// Handle Insert, Update, Upsert, Delete
	w.F("func (x *%s) Insert(ctx context.Context, dbc DB) error {\n", goName)
	w.N(`    if x._exists {`)
	w.N(`        return merry.Wrap(ErrInsertAlreadyExists)`)
	w.N(`    } else if x.deleted {`)
	w.N(`        return merry.Wrap(ErrInsertMarkedForDeletion)`)
	w.N(`    }`)
	w.N("    res, err := dbc.NamedExecContext(ctx, `")
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
	w.N("id, err := res.LastInsertId()")
	w.N("if err != nil {")
	w.N("	return err")
	w.N("}")
	w.N("x.ID = id")
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
	w.N("func (x *LoginAttempt) Delete(ctx context.Context, dbc DB) error {")
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

type Column struct {
	Name       string
	Type       Datatype
	PrimaryKey bool
	Nullable   bool
	Unique     bool
	Comment    string // Comment at the end of this column definition if provided.

	ForeignKey *ForeignKey // ForeignKey or null.

	// Default which can be a constant or expression and is type-dependent.
	// TODO: Handle expressions
	DefaultString sql.NullString
	DefaultInt    sql.NullInt64
	DefaultBool   sql.NullBool
}

// ORM
// TODO: Add comment about defaults?
func (c *Column) ORM(w ShortWriter) {
	comment := ""
	if c.Comment != "" {
		comment = fmt.Sprintf(" // %s", c.Comment)
	}
	w.F("    %s %s%s\n", ToGoName(c.Name), c.Type.ToGo(c.Nullable), comment)
}

type ForeignKey struct {
	Table      string
	ColumnName string
	OnUpdate   OnFkAction // OnUpdate action to take (e.g. none, Set Null, Set Default, etc.)
	OnDelete   OnFkAction // OnDelete action to take (e.g. none, Set Null, Set Default, etc.)
}
