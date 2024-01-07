package main

import (
	"database/sql"
	"fmt"
	"strings"
)

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
	sqlName     string
	goName      string
	Temp        bool
	IfNotExists bool
	Columns     []Column
	Comment     string // Comment at the end of the CREATE TABLE definition if provided.
	primaryKeys []string
}

func (t *Table) GoName() string  { return t.goName }
func (t *Table) SQLName() string { return t.sqlName }
func (t *Table) SetSQLName(name string) {
	t.sqlName = name
	t.goName = ToGoName(name)
}

func (t *Table) SetPrimaryKeys(colNames []string) error {
	if len(t.primaryKeys) > 0 {
		return fmt.Errorf("table cannot have more than one primary key definition")
	}
	if len(colNames) < 1 {
		return fmt.Errorf("must provide at least one column name for primary key(s)")
	}
	t.primaryKeys = colNames
	return nil
}

// PrimaryKeys returns the column name(s).
func (t *Table) PrimaryKeys() []string {
	if len(t.primaryKeys) > 0 {
		return t.primaryKeys
	}
	for _, col := range t.Columns {
		if col.PrimaryKey {
			return []string{col.sqlName}
		}
	}
	return []string{} // None found
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
	sqlName    string
	goName     string
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

func (t *Column) GoName() string  { return t.goName }
func (t *Column) SQLName() string { return t.sqlName }
func (t *Column) SetSQLName(name string) {
	t.sqlName = name
	t.goName = ToGoName(name)
}

// DBGenerated returns true if the database creates this column, such as row IDs, or
// created/updates_at metadata.
func (c *Column) DBGenerated() bool {
	switch {
	case c.PrimaryKey:
		return true
	case c.sqlName == "created_at":
		return true
	case c.sqlName == "updated_at":
		return true
	default:
		return false
	}
}

type ForeignKey struct {
	Table    string
	Column   string
	OnUpdate OnFkAction // OnUpdate action to take (e.g. none, Set Null, Set Default, etc.)
	OnDelete OnFkAction // OnDelete action to take (e.g. none, Set Null, Set Default, etc.)
}

// Template holds all data needed to execute out template.
type Template struct {
	PackageName string
	Tables      []*Table
}
