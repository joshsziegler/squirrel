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

type Table struct {
	SchemaName  string
	Name        string
	Temp        bool
	IfNotExists bool
	Columns     []Column
	Comment     string // Comment at the end of the CREATE TABLE definition if provided.
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
	Type       string
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

type ColumnInt struct {
	Column
	Default int64
}

type ForeignKey struct {
	Table      string
	ColumnName string
	OnUpdate   OnFkAction // OnUpdate action to take (e.g. none, Set Null, Set Default, etc.)
	OnDelete   OnFkAction // OnDelete action to take (e.g. none, Set Null, Set Default, etc.)
}
