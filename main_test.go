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
		{
			"basic table",
			`CREATE TABLE users ( name TEXT PRIMARY KEY )`, false,
			[]*Table{
				{
					sqlName: "users",
					goName:  "User",
					Columns: []Column{
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: true, Nullable: true},
					},
				},
			},
		},
		{
			"basic table without nullable primary key",
			`CREATE TABLE users ( name TEXT PRIMARY KEY NOT NULL)`, false,
			[]*Table{
				{
					sqlName: "users",
					goName:  "User",
					Columns: []Column{
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: true, Nullable: false},
					},
				},
			},
		},
		{
			"numeric ID primary key, but nullable unique name",
			`CREATE TABLE users (
				id INT PRIMARY KEY NOT NULL AUTOINCREMENT,
				name TEXT UNIQUE
			)`,
			false,
			[]*Table{
				{
					sqlName: "users",
					goName:  "User",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: false, Nullable: true, Unique: true},
					},
				},
			},
		},
		{
			"integer number of nodes and semicolon at end of definition",
			`CREATE TABLE jobs (
				id INT PRIMARY KEY NOT NULL AUTOINCREMENT,
				num_nodes INTEGER
			);`,
			false,
			[]*Table{
				{
					sqlName: "jobs",
					goName:  "Job",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "num_nodes", goName: "NumNode", Type: INT, PrimaryKey: false, Nullable: true},
					},
				},
			},
		},
		{
			"quotes table name and comments at end of the CREATE TABLE line",
			`CREATE TABLE "foo" ( -- Hello world!
				id INT PRIMARY KEY NOT NULL AUTOINCREMENT
			);`,
			false,
			[]*Table{
				{
					sqlName: "foo",
					goName:  "Foo",
					Comment: "Hello world!",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
					},
				},
			},
		},
		{
			"bugs due to spacing between comment delimiter and comment text",
			`CREATE TABLE comments (
				foo TEXT, --no space between delimiter and first word
				bar TEXT,-- no space after comma ending the column definition
				baz TEXT--No space on either side
			);`,
			false,
			[]*Table{
				{
					sqlName: "comments",
					goName:  "Comment",
					Columns: []Column{
						{sqlName: "foo", goName: "Foo", Type: TEXT, Nullable: true, Comment: "no space between delimiter and first word"},
						{sqlName: "bar", goName: "Bar", Type: TEXT, Nullable: true, Comment: "no space after comma ending the column definition"},
						{sqlName: "baz", goName: "Baz", Type: TEXT, Nullable: true, Comment: "No space on either side"},
					},
				},
			},
		},
		{
			"comments at end of a column line",
			`CREATE TABLE bars (
				name TEXT NOT NULL UNIQUE, -- name of the bar
				open INTEGER,
				close INTEGER -- Hour (1-24) the bar closes if known
			);`,
			false,
			[]*Table{
				{
					sqlName: "bars",
					goName:  "Bar",
					Columns: []Column{
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: false, Nullable: false, Unique: true, Comment: "name of the bar"},
						{sqlName: "open", goName: "Open", Type: INT, PrimaryKey: false, Nullable: true, Unique: false},
						// FIXME: Lexer/Parser turns this comment from "(1-24)" to "( 1-24 )"
						{sqlName: "close", goName: "Close", Type: INT, PrimaryKey: false, Nullable: true, Unique: false, Comment: "Hour ( 1-24 ) the bar closes if known"},
					},
				},
			},
		},
		{
			"every data type supported by STRICT",
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
					sqlName: "animals",
					goName:  "Animal",
					Columns: []Column{
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: true, Nullable: false},
						{sqlName: "age", goName: "Age", Type: INT, Nullable: false},
						{sqlName: "weight", goName: "Weight", Type: FLOAT, Nullable: false},
						{sqlName: "height", goName: "Height", Type: INT, Nullable: false},
						{sqlName: "last_seen", goName: "LastSeen", Type: DATETIME, Nullable: true},
						{sqlName: "photo", goName: "Photo", Type: BLOB, Nullable: true},
						{sqlName: "data", goName: "Datum", Type: BLOB, Nullable: true},
					},
				},
			},
		},
		{
			"invalid column type 'INTERGER'",
			`CREATE TABLE jobs (
				id TEXT UNIQUE NOT NULL,
				user_id INTERGER NOT NULL
			);`,
			true,
			[]*Table{},
		},
		{
			"a Foreign Key specified with inline REFERENCES",
			`CREATE TABLE people (
				id		INT NOT NULL PRIMARY KEY,
				name	TEXT NOT NULL, -- Name may not be unique!
				spouse	INT REFERENCES people(id) -- Husband or Wife within this table
			);`,
			false,
			[]*Table{
				{
					sqlName: "people",
					goName:  "Person",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "name", goName: "Name", Type: TEXT, Nullable: false, Comment: "Name may not be unique!"},
						{sqlName: "spouse", goName: "Spouse", Type: INT, Nullable: true, ForeignKey: &ForeignKey{Table: "people", Column: "id"}, Comment: "Husband or Wife within this table"},
					},
				},
			},
		},
		{
			"default value as a constant string",
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
					sqlName: "product",
					goName:  "Product",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "name", goName: "Name", Type: TEXT, Nullable: false, Unique: true},
						{sqlName: "type", goName: "Type", Type: TEXT, Nullable: false, DefaultString: sql.NullString{Valid: true, String: "software"}},
						{sqlName: "description", goName: "Description", Type: TEXT, Nullable: false, DefaultString: sql.NullString{Valid: true, String: ""}, Comment: "Empty string as the default"},
						{sqlName: "discontinued", goName: "Discontinued", Type: BOOL, Nullable: false, DefaultBool: sql.NullBool{Valid: true, Bool: false}},
						{sqlName: "on_sale", goName: "OnSale", Type: BOOL, Nullable: true, DefaultBool: sql.NullBool{Valid: true, Bool: true}, Comment: "true using integer notation"},
						{sqlName: "magic", goName: "Magic", Type: BOOL, Nullable: true, DefaultBool: sql.NullBool{Valid: true, Bool: true}},
						{sqlName: "stolen", goName: "Stolen", Type: BOOL, Nullable: true, DefaultBool: sql.NullBool{Valid: true, Bool: false}},
						{sqlName: "intelligent", goName: "Intelligent", Type: BOOL, Nullable: true, DefaultBool: sql.NullBool{Valid: true, Bool: false}},
					},
				},
			},
		},
		{
			"a Foreign Key specified with inline REFERENCES and ON DELETE, ON UPDATE, and both ...",
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
					sqlName: "boxes",
					goName:  "Box",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "name", goName: "Name", Type: TEXT, Nullable: false, Unique: true},
					},
				},
				{
					sqlName: "franchises",
					goName:  "Franchise",
					Columns: []Column{
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: true, Nullable: false},
					},
				},
				{
					sqlName: "toys",
					goName:  "Toy",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "name", goName: "Name", Type: TEXT, Nullable: false, Unique: false},
						{sqlName: "box_id", goName: "BoxID", Type: INT, Nullable: false, ForeignKey: &ForeignKey{Table: "boxes", Column: "id", OnDelete: Cascade}},
						{sqlName: "franchise_name", goName: "FranchiseName", Type: TEXT, Nullable: false, ForeignKey: &ForeignKey{Table: "franchises", Column: "name", OnUpdate: Cascade, OnDelete: SetNull}},
					},
				},
			},
		},
		{
			"a Foreign Key specified with without specifying a column name",
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
					sqlName: "users",
					goName:  "User",
					Columns: []Column{
						{sqlName: "user_id", goName: "UserID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "name", goName: "Name", Type: TEXT, Nullable: false},
					},
				},
				{
					sqlName: "groups",
					goName:  "Group",
					Columns: []Column{
						{sqlName: "group_name", goName: "GroupName", Type: TEXT, PrimaryKey: true, Nullable: false},
					},
				},
				{
					sqlName: "user_group",
					goName:  "UserGroup",
					Columns: []Column{
						{sqlName: "user_id", goName: "UserID", Type: INT, Nullable: false, ForeignKey: &ForeignKey{Table: "users", Column: "user_id"}, Comment: "Column name is implied by omitting it"},
						{sqlName: "group_name", goName: "GroupName", Type: TEXT, Nullable: false, ForeignKey: &ForeignKey{Table: "groups", Column: "group_name"}},
					},
				},
			},
		},
		{
			"named-constraint-with-multi-column-primary-key",
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
					sqlName:     "albums",
					goName:      "Album",
					IfNotExists: true,
					Columns: []Column{
						{sqlName: "artist", goName: "Artist", Type: TEXT, PrimaryKey: false, Nullable: false},
						{sqlName: "album_title", goName: "AlbumTitle", Type: TEXT, PrimaryKey: false, Nullable: false},
						{sqlName: "year", goName: "Year", Type: INT, PrimaryKey: false, Nullable: false},
					},
				},
			},
		},
		{
			"unnamed-unique-constraint",
			`CREATE TABLE IF NOT EXISTS "players" (
			 server        INT NOT NULL,
			 character_name   TEXT NOT NULL,
			 UNIQUE (server, character_name) -- Names must be unique PER SERVER
			)
			`,
			false,
			[]*Table{
				{
					sqlName:     "players",
					goName:      "Player",
					IfNotExists: true,
					Columns: []Column{
						{sqlName: "server", goName: "Server", Type: INT, PrimaryKey: false, Nullable: false},
						{sqlName: "character_name", goName: "CharacterName", Type: TEXT, PrimaryKey: false, Nullable: false},
					},
				},
			},
		},
		{
			"foreign-key-on-own-line-to-single-column",
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
					sqlName: "albums",
					goName:  "Album",
					Columns: []Column{
						{sqlName: "artist", goName: "Artist", Type: TEXT, PrimaryKey: false, Nullable: false},
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: false, Nullable: false},
						{sqlName: "year", goName: "Year", Type: INT, PrimaryKey: false, Nullable: true},
					},
				},
			},
		},
		{
			"foreign key on own line",
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
					sqlName: "job_extended_attrs",
					goName:  "JobExtendedAttr",
					Columns: []Column{
						{sqlName: "fk_job_id", goName: "FkJobID", Type: TEXT, PrimaryKey: false, Nullable: false},
						{sqlName: "auto_extend", goName: "AutoExtend", Type: INT, Nullable: false},
					},
				},
			},
		},
		{
			"indices-on-simple-table",
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
					sqlName: "users",
					goName:  "User",
					Columns: []Column{
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: true, Nullable: false},
						{sqlName: "email", goName: "Email", Type: TEXT},
						{sqlName: "role", goName: "Role", Type: TEXT},
					},
				},
			},
		},
		{
			"datetime with default value of now",
			`CREATE TABLE "login_attempts" (
				id   INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
				ip   TEXT NOT NULL,
				time DATETIME NOT NULL DEFAULT (datetime('now'))
			)
		`,
			false,
			[]*Table{
				{
					sqlName: "login_attempts",
					goName:  "LoginAttempt",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "ip", goName: "IP", Type: TEXT, Nullable: false},
						{sqlName: "time", goName: "Time", Type: DATETIME, Nullable: false},
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
