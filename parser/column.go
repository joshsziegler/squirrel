package parser

import (
	"database/sql"

	"github.com/joshsziegler/squirrel/name"
)

type Column struct {
	sqlName             string
	goName              string
	Type                Datatype
	PrimaryKey          bool // True if this column is the one and only primary key (typically defined inline with the column).
	CompositePrimaryKey bool // True if this column is part of a composite primary key.
	autoIncrement       bool // AutoIncrement is true if the this column explicitly specified AUTOINCREMENT. Use AutoIncrement()!
	Nullable            bool
	Unique              bool
	Comment             string      // Comment at the end of this column definition if provided.
	ForeignKey          *ForeignKey // ForeignKey or null.

	// Default which can be a constant or expression and is type-dependent.
	// TODO: Handle expressions
	DefaultString sql.NullString
	DefaultInt    sql.NullInt64
	DefaultBool   sql.NullBool
}

func (t *Column) GoName() string  { return t.goName }
func (t *Column) SQLName() string { return t.sqlName }
func (t *Column) SetSQLName(s string) {
	t.sqlName = s
	t.goName = name.ToGo(s)
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

// AutoIncrement is true if the column explicitly defined or SQLite's deems it to be a row_id alias.
//
// My understanding of the docs is that any column that is both a PK and type INTEGER will be auto-
// incremented, regardless of whether it specifies "AUTOINCREMENT". The keyword only affects how the
// ID is chosen.
//
// SQLite Docs (https://www.sqlite.org/autoinc.html):
//
//  1. The AUTOINCREMENT keyword imposes extra CPU, memory, disk space, and disk I/O overhead and
//     should be avoided if not strictly needed. It is usually not needed.
//  2. In SQLite, a column with type INTEGER PRIMARY KEY is an alias for the ROWID (except in
//     WITHOUT ROWID tables) which is always a 64-bit signed integer.
//  3. On an INSERT, if the ROWID or INTEGER PRIMARY KEY column is not explicitly given a value,
//     then it will be filled automatically with an unused integer, usually one more than the
//     largest ROWID currently in use. This is true regardless of whether or not the AUTOINCREMENT
//     keyword is used.
//  4. If the AUTOINCREMENT keyword appears after INTEGER PRIMARY KEY, that changes the automatic
//     ROWID assignment algorithm to prevent the reuse of ROWIDs over the lifetime of the database
//     In other words, the purpose of AUTOINCREMENT is to prevent the reuse of ROWIDs from
//     previously deleted rows.
func (c *Column) AutoIncrement() bool {
	return c.PrimaryKey && c.Type == INT
}
