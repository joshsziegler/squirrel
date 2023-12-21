package main

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		wantErr bool
		want    []*Table
	}{
		{"basic table",
			`CREATE TABLE users ( name TEXT PRIMARY KEY )`, false,
			[]*Table{
				{
					Name: "users",
					Columns: []Column{
						{Name: "name", Type: "string", PrimaryKey: true, Nullable: true},
					},
				},
			},
		},
		{"basic table without nullable primary key",
			`CREATE TABLE users ( name TEXT PRIMARY KEY NOT NULL)`, false,
			[]*Table{
				{
					Name: "users",
					Columns: []Column{
						{Name: "name", Type: "string", PrimaryKey: true, Nullable: false},
					},
				},
			},
		},
		{"numeric ID primary key, but nullable unique name",
			`CREATE TABLE users (
				id INT PRIMARY KEY NOT NULL AUTOINCREMENT,
				name TEXT UNIQUE
			)`,
			false,
			[]*Table{
				{
					Name: "users",
					Columns: []Column{
						{Name: "id", Type: "int64", PrimaryKey: true, Nullable: false},
						{Name: "name", Type: "string", PrimaryKey: false, Nullable: true, Unique: true},
					},
				},
			},
		},
		{"integer number of nodes and semicolon at end of definition",
			`CREATE TABLE jobs (
				id INT PRIMARY KEY NOT NULL AUTOINCREMENT,
				num_nodes INTEGER
			);`,
			false,
			[]*Table{
				{
					Name: "jobs",
					Columns: []Column{
						{Name: "id", Type: "int64", PrimaryKey: true, Nullable: false},
						{Name: "num_nodes", Type: "int64", PrimaryKey: false, Nullable: true},
					},
				},
			},
		},
		{"quotes table name and comments at end of the CREATE TABLE line",
			`CREATE TABLE "foo" ( -- Hello world!
				id INT PRIMARY KEY NOT NULL AUTOINCREMENT
			);`,
			false,
			[]*Table{
				{
					Name:    "foo",
					Comment: "Hello world!",
					Columns: []Column{
						{Name: "id", Type: "int64", PrimaryKey: true, Nullable: false},
					},
				},
			},
		},
		{"bugs due to spacing between comment delimiter and comment text",
			`CREATE TABLE comments (
				foo TEXT, --no space between delimiter and first word
				bar TEXT,-- no space after comma ending the column definition
				baz TEXT--No space on either side
			);`,
			false,
			[]*Table{
				{
					Name: "comments",
					Columns: []Column{
						{Name: "foo", Type: "string", Nullable: true, Comment: "no space between delimiter and first word"},
						{Name: "bar", Type: "string", Nullable: true, Comment: "no space after comma ending the column definition"},
						{Name: "baz", Type: "string", Nullable: true, Comment: "No space on either side"},
					},
				},
			},
		},
		{"comments at end of a column line",
			`CREATE TABLE bars (
				name TEXT NOT NULL UNIQUE, -- name of the bar
				open INTEGER,
				close INTEGER -- Hour (1-24) the bar closes if known
			);`,
			false,
			[]*Table{
				{
					Name: "bars",
					Columns: []Column{
						{Name: "name", Type: "string", PrimaryKey: false, Nullable: false, Unique: true, Comment: "name of the bar"},
						{Name: "open", Type: "int64", PrimaryKey: false, Nullable: true, Unique: false},
						// FIXME: Lexer/Parser turns this comment from "(1-24)" to "( 1-24 )"
						{Name: "close", Type: "int64", PrimaryKey: false, Nullable: true, Unique: false, Comment: "Hour ( 1-24 ) the bar closes if known"},
					},
				},
			},
		},
		{"every data type supported by STRICT",
			`CREATE TABLE animals (
				name TEXT PRIMARY KEY NOT NULL,
				age INT NOT NULL,
				weight REAL NOT NULL,
				height INTEGER NOT NULL,
				last_seen DATETIME,
				photo BLOB,
				data ANY
			);`,
			false,
			[]*Table{
				{
					Name: "animals",
					Columns: []Column{
						{Name: "name", Type: "string", PrimaryKey: true, Nullable: false},
						{Name: "age", Type: "int64", Nullable: false},
						{Name: "weight", Type: "float64", Nullable: false},
						{Name: "height", Type: "int64", Nullable: false},
						{Name: "last_seen", Type: "time.Time", Nullable: true},
						{Name: "photo", Type: "[]byte", Nullable: true},
						{Name: "data", Type: "[]byte", Nullable: true},
					},
				},
			},
		},
		{"invalid column type 'INTERGER'",
			`CREATE TABLE jobs (
				id TEXT UNIQUE NOT NULL,
				user_id INTERGER NOT NULL
			);`,
			true,
			[]*Table{},
		},
		{"a Foreign Key specified with inline REFERENCES",
			`CREATE TABLE people (
				id		INT NOT NULL PRIMARY KEY,
				name	TEXT NOT NULL, -- Name may not be unique!
				spouse	INT REFERENCES people(id) -- Husband or Wife within this table
			);`,
			false,
			[]*Table{
				{
					Name: "people",
					Columns: []Column{
						{Name: "id", Type: "int64", PrimaryKey: true, Nullable: false},
						{Name: "name", Type: "string", Nullable: false, Comment: "Name may not be unique!"},
						{Name: "spouse", Type: "int64", Nullable: true, ForeignKey: &ForeignKey{Table: "people", ColumnName: "id"}, Comment: "Husband or Wife within this table"},
					},
				},
			},
		},
		{"default value as a constant string",
			`CREATE TABLE product (
				id				INT		NOT NULL	PRIMARY KEY,
				name			TEXT	NOT NULL	UNIQUE,
				type			TEXT	NOT NULL	DEFAULT "software",
				description		TEXT	NOT NULL	DEFAULT "", -- Empty string as the default
				discontinued	BOOL	NOT NULL	DEFAULT FALSE,
				on_sale			BOOLEAN				DEFAULT 1, -- true using integer notation
				magic			BOOL				DEFAULT TRUE,
				stolen			BOOL				DEFAULT false,
				intelligent		BOOL				DEFAULT 0
			);`,
			false,
			[]*Table{
				{
					Name: "product",
					Columns: []Column{
						{Name: "id", Type: "int64", PrimaryKey: true, Nullable: false},
						{Name: "name", Type: "string", Nullable: false, Unique: true},
						{Name: "type", Type: "string", Nullable: false, DefaultString: sql.NullString{Valid: true, String: "software"}},
						{Name: "description", Type: "string", Nullable: false, DefaultString: sql.NullString{Valid: true, String: ""}, Comment: "Empty string as the default"},
						{Name: "discontinued", Type: "bool", Nullable: false, DefaultBool: sql.NullBool{Valid: true, Bool: false}},
						{Name: "on_sale", Type: "bool", Nullable: true, DefaultBool: sql.NullBool{Valid: true, Bool: true}, Comment: "true using integer notation"},
						{Name: "magic", Type: "bool", Nullable: true, DefaultBool: sql.NullBool{Valid: true, Bool: true}},
						{Name: "stolen", Type: "bool", Nullable: true, DefaultBool: sql.NullBool{Valid: true, Bool: false}},
						{Name: "intelligent", Type: "bool", Nullable: true, DefaultBool: sql.NullBool{Valid: true, Bool: false}},
					},
				},
			},
		},
		{"a Foreign Key specified with inline REFERENCES and ON DELETE, ON UPDATE, and both ...",
			`CREATE TABLE boxes (
				id		INTEGER NOT NULL PRIMARY KEY,
				name	TEXT NOT NULL UNIQUE
			);
			CREATE TABLE franchises (
				name	TEXT NOT NULL PRIMARY KEY
			);
			CREATE TABLE toys (
				id		INTEGER NOT NULL PRIMARY KEY,
				name	TEXT NOT NULL,
				box_id	INTEGER NOT NULL REFERENCES boxes (id) ON DELETE CASCADE,
				franchise_name	TEXT NOT NULL REFERENCES franchises(name) ON UPDATE CASCADE ON DELETE SET NULL
			)`,
			false,
			[]*Table{
				{
					Name: "boxes",
					Columns: []Column{
						{Name: "id", Type: "int64", PrimaryKey: true, Nullable: false},
						{Name: "name", Type: "string", Nullable: false, Unique: true},
					},
				},
				{
					Name: "franchises",
					Columns: []Column{
						{Name: "name", Type: "string", PrimaryKey: true, Nullable: false},
					},
				},
				{
					Name: "toys",
					Columns: []Column{
						{Name: "id", Type: "int64", PrimaryKey: true, Nullable: false},
						{Name: "name", Type: "string", Nullable: false, Unique: false},
						{Name: "box_id", Type: "int64", Nullable: false, ForeignKey: &ForeignKey{Table: "boxes", ColumnName: "id", OnDelete: Cascade}},
						{Name: "franchise_name", Type: "string", Nullable: false, ForeignKey: &ForeignKey{Table: "franchises", ColumnName: "name", OnUpdate: Cascade, OnDelete: SetNull}},
					},
				},
			},
		},
		{"a Foreign Key specified with without specifying a column name",
			`CREATE TABLE users (
				user_id	INTEGER NOT NULL PRIMARY KEY,
				name	TEXT NOT NULL
			);
			CREATE TABLE groups (
				group_name	TEXT NOT NULL PRIMARY KEY
			);
			CREATE TABLE user_group (
				user_id		INTEGER NOT NULL REFERENCES users, -- Column name is implied by omitting it
				group_name	TEXT NOT NULL REFERENCES groups
			);
			`,
			false,
			[]*Table{
				{
					Name: "users",
					Columns: []Column{
						{Name: "user_id", Type: "int64", PrimaryKey: true, Nullable: false},
						{Name: "name", Type: "string", Nullable: false},
					},
				},
				{
					Name: "groups",
					Columns: []Column{
						{Name: "group_name", Type: "string", PrimaryKey: true, Nullable: false},
					},
				},
				{
					Name: "user_group",
					Columns: []Column{
						{Name: "user_id", Type: "int64", Nullable: false, ForeignKey: &ForeignKey{Table: "users", ColumnName: "user_id"}, Comment: "Column name is implied by omitting it"},
						{Name: "group_name", Type: "string", Nullable: false, ForeignKey: &ForeignKey{Table: "groups", ColumnName: "group_name"}},
					},
				},
			},
		},
		{"named-constraint-with-multi-column-primary-key",
			`CREATE TABLE IF NOT EXISTS "albums" (
			 artist        TEXT NOT NULL,
			 album_title   TEXT NOT NULL,
			 year          INT NOT NULL,
			 CONSTRAINT author_book PRIMARY KEY (artist, album_title) --
			)
			`,
			false,
			[]*Table{
				{
					Name:        "albums",
					IfNotExists: true,
					Columns: []Column{
						{Name: "artist", Type: "string", PrimaryKey: false, Nullable: false},
						{Name: "album_title", Type: "string", PrimaryKey: false, Nullable: false},
						{Name: "year", Type: "int64", PrimaryKey: false, Nullable: false},
					},
				},
			},
		},
		{"unnamed-unique-constraint",
			`CREATE TABLE IF NOT EXISTS "players" (
			 server        INT NOT NULL,
			 character_name   TEXT NOT NULL,
			 UNIQUE (server, character_name) -- Names must be unique PER SERVER
			)
			`,
			false,
			[]*Table{
				{
					Name:        "players",
					IfNotExists: true,
					Columns: []Column{
						{Name: "server", Type: "int64", PrimaryKey: false, Nullable: false},
						{Name: "character_name", Type: "string", PrimaryKey: false, Nullable: false},
					},
				},
			},
		},
		{"foreign-key-on-own-line-to-single-column",
			`CREATE TABLE "albums" (
			 artist TEXT NOT NULL,
			 name   TEXT NOT NULL,
			 year   INT,
			 FOREIGN KEY (artist) REFERENCES artist (name) ON DELETE CASCADE-- Link back to the artist table and delete if artist is deleted
			)
			`,
			false,
			[]*Table{
				{
					Name: "albums",
					Columns: []Column{
						{Name: "artist", Type: "string", PrimaryKey: false, Nullable: false},
						{Name: "name", Type: "string", PrimaryKey: false, Nullable: false},
						{Name: "year", Type: "int64", PrimaryKey: false, Nullable: true},
					},
				},
			},
		},
		{"foreign key on own line",
			` CREATE TABLE job_extended_attrs (
				fk_job_id TEXT NOT NULL,
				auto_extend INTEGER NOT NULL,
				PRIMARY KEY (fk_job_id),
				FOREIGN KEY(fk_job_id) REFERENCES jobsCache(id) ON DELETE CASCADE
			)
		`,
			false,
			[]*Table{
				{
					Name: "job_extended_attrs",
					Columns: []Column{
						{Name: "fk_job_id", Type: "string", PrimaryKey: false, Nullable: false},
						{Name: "auto_extend", Type: "int64", Nullable: false},
					},
				},
			},
		},
		{"indices-on-simple-table",
			`CREATE TABLE users (
				name TEXT NOT NULL PRIMARY KEY,
				email TEXT NOT NULL,
				role TEXT NOT NULL
			)
			CREATE INDEX idx_users_email ON users(email);
			CREATE INDEX idx_users_role ON users(role);
		`,
			false,
			[]*Table{
				{
					Name: "users",
					Columns: []Column{
						{Name: "name", Type: "string", PrimaryKey: true, Nullable: false},
						{Name: "email", Type: "string"},
						{Name: "role", Type: "string"},
					},
				},
			},
		},
		{"datetime with default value of now",
			`CREATE TABLE "login_attempts" (
				id   INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
				ip   TEXT NOT NULL,
				time DATETIME NOT NULL DEFAULT (datetime('now'))
			)
		`,
			false,
			[]*Table{
				{
					Name: "login_attempts",
					Columns: []Column{
						{Name: "id", Type: "int64", PrimaryKey: true, Nullable: false},
						{Name: "ip", Type: "string", Nullable: false},
						{Name: "time", Type: "time.Time", Nullable: false},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.sql)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got, "%+v", got)
		})
	}
}
