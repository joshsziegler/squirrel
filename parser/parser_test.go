package parser

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
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: false, Nullable: true},
					},
					UniqueConstraints: []UniqueConstraint{{Columns: []string{"name"}}},
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
			"quoted column names",
			`CREATE TABLE widgets (
				"id"    INTEGER NOT NULL PRIMARY KEY,
				"total" INTEGER NOT NULL,
				"label" TEXT NOT NULL
			);`,
			false,
			[]*Table{
				{
					sqlName: "widgets",
					goName:  "Widget",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "total", goName: "Total", Type: INT, Nullable: false},
						{sqlName: "label", goName: "Label", Type: TEXT, Nullable: false},
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
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: false, Nullable: false, Comment: "name of the bar"},
						{sqlName: "open", goName: "Open", Type: INT, PrimaryKey: false, Nullable: true},
						{sqlName: "close", goName: "Close", Type: INT, PrimaryKey: false, Nullable: true, Comment: "Hour (1-24) the bar closes if known"},
					},
					UniqueConstraints: []UniqueConstraint{{Columns: []string{"name"}}},
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
			"invalid column type 'INTERGER' defaults to BLOB when STRICT is off",
			`CREATE TABLE jobs (
				id TEXT UNIQUE NOT NULL,
				user_id INTERGER NOT NULL
			);`,
			false,
			[]*Table{
				{
					sqlName: "jobs",
					goName:  "Job",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: TEXT, Nullable: false},
						{sqlName: "user_id", goName: "UserID", Type: BLOB, Nullable: false},
					},
					UniqueConstraints: []UniqueConstraint{{Columns: []string{"id"}}},
				},
			},
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
						{sqlName: "spouse", goName: "Spouse", Type: INT, Nullable: true, Comment: "Husband or Wife within this table"},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "people", LocalColumns: []string{"spouse"}, Columns: []string{"id"}},
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
						{sqlName: "name", goName: "Name", Type: TEXT, Nullable: false},
						{sqlName: "type", goName: "Type", Type: TEXT, Nullable: false, DefaultString: sql.NullString{Valid: true, String: "software"}},
						{sqlName: "description", goName: "Description", Type: TEXT, Nullable: false, DefaultString: sql.NullString{Valid: true, String: ""}, Comment: "Empty string as the default"},
						{sqlName: "discontinued", goName: "Discontinued", Type: BOOL, Nullable: false, DefaultBool: sql.NullBool{Valid: true, Bool: false}},
						{sqlName: "on_sale", goName: "OnSale", Type: BOOL, Nullable: true, DefaultBool: sql.NullBool{Valid: true, Bool: true}, Comment: "true using integer notation"},
						{sqlName: "magic", goName: "Magic", Type: BOOL, Nullable: true, DefaultBool: sql.NullBool{Valid: true, Bool: true}},
						{sqlName: "stolen", goName: "Stolen", Type: BOOL, Nullable: true, DefaultBool: sql.NullBool{Valid: true, Bool: false}},
						{sqlName: "intelligent", goName: "Intelligent", Type: BOOL, Nullable: true, DefaultBool: sql.NullBool{Valid: true, Bool: false}},
					},
					UniqueConstraints: []UniqueConstraint{{Columns: []string{"name"}}},
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
						{sqlName: "name", goName: "Name", Type: TEXT, Nullable: false},
					},
					UniqueConstraints: []UniqueConstraint{{Columns: []string{"name"}}},
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
						{sqlName: "name", goName: "Name", Type: TEXT, Nullable: false},
						{sqlName: "box_id", goName: "BoxID", Type: INT, Nullable: false},
						{sqlName: "franchise_name", goName: "FranchiseName", Type: TEXT, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "boxes", LocalColumns: []string{"box_id"}, Columns: []string{"id"}, OnDelete: Cascade},
						{Table: "franchises", LocalColumns: []string{"franchise_name"}, Columns: []string{"name"}, OnUpdate: Cascade, OnDelete: SetNull},
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
						{sqlName: "user_id", goName: "UserID", Type: INT, Nullable: false, Comment: "Column name is implied by omitting it"},
						{sqlName: "group_name", goName: "GroupName", Type: TEXT, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "users", LocalColumns: []string{"user_id"}, Columns: []string{"user_id"}},
						{Table: "groups", LocalColumns: []string{"group_name"}, Columns: []string{"group_name"}},
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
						{sqlName: "artist", goName: "Artist", Type: TEXT, PrimaryKey: false, CompositePrimaryKey: true, Nullable: false},
						{sqlName: "album_title", goName: "AlbumTitle", Type: TEXT, PrimaryKey: false, CompositePrimaryKey: true, Nullable: false},
						{sqlName: "year", goName: "Year", Type: INT, PrimaryKey: false, Nullable: false},
					},
					PrimaryKeyName: "author_book",
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
					UniqueConstraints: []UniqueConstraint{{Columns: []string{"server", "character_name"}}},
				},
			},
		},
		{
			"single-column table-level UNIQUE is recorded as a constraint",
			`CREATE TABLE t (
				id		INTEGER NOT NULL PRIMARY KEY,
				email	TEXT NOT NULL,
				UNIQUE (email)
			)`,
			false,
			[]*Table{
				{
					sqlName: "t",
					goName:  "T",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "email", goName: "Email", Type: TEXT, Nullable: false},
					},
					UniqueConstraints: []UniqueConstraint{{Columns: []string{"email"}}},
				},
			},
		},
		{
			"named single-column UNIQUE constraint retains its name",
			`CREATE TABLE t (
				id		INTEGER NOT NULL PRIMARY KEY,
				email	TEXT NOT NULL,
				CONSTRAINT uc_email UNIQUE (email)
			)`,
			false,
			[]*Table{
				{
					sqlName: "t",
					goName:  "T",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "email", goName: "Email", Type: TEXT, Nullable: false},
					},
					UniqueConstraints: []UniqueConstraint{{Name: "uc_email", Columns: []string{"email"}}},
				},
			},
		},
		{
			"multiple UNIQUE constraints, single- and multi-column",
			`CREATE TABLE memberships (
				id		INTEGER NOT NULL PRIMARY KEY,
				org_id	INTEGER NOT NULL,
				user_id	INTEGER NOT NULL,
				slug	TEXT NOT NULL,
				UNIQUE (slug),
				UNIQUE (org_id, user_id)
			)`,
			false,
			[]*Table{
				{
					sqlName: "memberships",
					goName:  "Membership",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "org_id", goName: "OrgID", Type: INT, Nullable: false},
						{sqlName: "user_id", goName: "UserID", Type: INT, Nullable: false},
						{sqlName: "slug", goName: "Slug", Type: TEXT, Nullable: false},
					},
					UniqueConstraints: []UniqueConstraint{
						{Columns: []string{"slug"}},
						{Columns: []string{"org_id", "user_id"}},
					},
				},
			},
		},
		{
			"named multi-column UNIQUE constraint retains its name",
			`CREATE TABLE memberships (
				id		INTEGER NOT NULL PRIMARY KEY,
				org_id	INTEGER NOT NULL,
				user_id	INTEGER NOT NULL,
				CONSTRAINT uc_org_user UNIQUE (org_id, user_id)
			)`,
			false,
			[]*Table{
				{
					sqlName: "memberships",
					goName:  "Membership",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "org_id", goName: "OrgID", Type: INT, Nullable: false},
						{sqlName: "user_id", goName: "UserID", Type: INT, Nullable: false},
					},
					UniqueConstraints: []UniqueConstraint{{Name: "uc_org_user", Columns: []string{"org_id", "user_id"}}},
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
					ForeignKeys: []*ForeignKey{
						{Table: "artist", LocalColumns: []string{"artist"}, Columns: []string{"name"}, OnDelete: Cascade},
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
						{sqlName: "fk_job_id", goName: "FkJobID", Type: TEXT, PrimaryKey: true, Nullable: false},
						{sqlName: "auto_extend", goName: "AutoExtend", Type: INT, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "jobsCache", LocalColumns: []string{"fk_job_id"}, Columns: []string{"id"}, OnDelete: Cascade},
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
		{
			"sqlite sequence table",
			`CREATE TABLE sqlite_sequence ( name , seq ) ;`,
			false,
			[]*Table{
				{
					sqlName: "sqlite_sequence",
					goName:  "SqliteSequence",
					Columns: []Column{
						{sqlName: "name", goName: "Name", Type: BLOB, Nullable: true},
						{sqlName: "seq", goName: "Seq", Type: BLOB, Nullable: true},
					},
				},
			},
		},
		{
			"default bool with parenthesis",
			`CREATE TABLE posts (
				title TEXT PRIMARY KEY NOT NULL,
				public BOOL DEFAULT (FALSE)
			)`,
			false,
			[]*Table{
				{
					sqlName: "posts",
					goName:  "Post",
					Columns: []Column{
						{sqlName: "title", goName: "Title", Type: TEXT, PrimaryKey: true, Nullable: false},
						{sqlName: "public", goName: "Public", Type: BOOL, Nullable: true, DefaultBool: sql.NullBool{Valid: true, Bool: false}},
					},
				},
			},
		},
		{
			"bug: unrecognized table constraint due to two 'ON' clauses",
			`CREATE TABLE ip_login_attempts (
				id   INTEGER PRIMARY KEY AUTOINCREMENT,
				ip   TEXT NOT NULL,
				time DATETIME NOT NULL DEFAULT (datetime('now')),
				FOREIGN KEY (ip) REFERENCES ip_login_summary (ip) ON UPDATE CASCADE ON DELETE CASCADE
			);`,
			false,
			[]*Table{
				{
					sqlName: "ip_login_attempts",
					goName:  "IPLoginAttempt",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: true},
						{sqlName: "ip", goName: "IP", Type: TEXT, Nullable: false},
						{sqlName: "time", goName: "Time", Type: DATETIME, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "ip_login_summary", LocalColumns: []string{"ip"}, Columns: []string{"ip"}, OnUpdate: Cascade, OnDelete: Cascade},
					},
				},
			},
		},
		{
			"bug: unrecognized table constraint due to two 'ON' clauses",
			`CREATE TABLE ip_login_summary (
				ip                 TEXT PRIMARY KEY,
				total_attempts     INTEGER DEFAULT 0,
				locked             BOOLEAN DEFAULT 0,
				lockout_time       DATETIME DEFAULT NULL,
				last_attempt_time  DATETIME DEFAULT NULL
			);
			CREATE TABLE ip_login_attempts (
				id   INTEGER PRIMARY KEY AUTOINCREMENT,
				ip   TEXT NOT NULL REFERENCES ip_login_summary (ip) ON UPDATE CASCADE ON DELETE CASCADE,
				time DATETIME NOT NULL DEFAULT (datetime('now'))
			);`,
			false,
			[]*Table{
				{
					sqlName: "ip_login_summary",
					goName:  "IPLoginSummary",
					Columns: []Column{
						{sqlName: "ip", goName: "IP", Type: TEXT, PrimaryKey: true, Nullable: true},
						{sqlName: "total_attempts", goName: "TotalAttempt", Type: INT, Nullable: true, DefaultInt: sql.NullInt64{Valid: true, Int64: 0}},
						{sqlName: "locked", goName: "Locked", Type: BOOL, Nullable: true, DefaultBool: sql.NullBool{Valid: true, Bool: false}},
						{sqlName: "lockout_time", goName: "LockoutTime", Type: DATETIME, Nullable: true},
						{sqlName: "last_attempt_time", goName: "LastAttemptTime", Type: DATETIME, Nullable: true},
					},
				},
				{
					sqlName: "ip_login_attempts",
					goName:  "IPLoginAttempt",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: true},
						{sqlName: "ip", goName: "IP", Type: TEXT, Nullable: false},
						{sqlName: "time", goName: "Time", Type: DATETIME, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "ip_login_summary", LocalColumns: []string{"ip"}, Columns: []string{"ip"}, OnUpdate: Cascade, OnDelete: Cascade},
					},
				},
			},
		},
		{
			"BUG: Parser fails on multiline 'CREATE INDEX'",
			`CREATE TABLE accounts (
				id				   INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
				name			   TEXT NOT NULL,
				type			   INTEGER NOT NULL,
				total			   INTEGER NOT NULL DEFAULT 0,
				total_used		   INTEGER NOT NULL DEFAULT 0,
				deactivated        BOOLEAN NOT NULL DEFAULT FALSE
			);
			CREATE INDEX idx_account_name_unique_active
			ON account (name)
			WHERE deactivated = FALSE;
			`,
			false,
			[]*Table{
				{
					sqlName: "accounts",
					goName:  "Account",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: false, Nullable: false},
						{sqlName: "type", goName: "Type", Type: INT, PrimaryKey: false, Nullable: false},
						{sqlName: "total", goName: "Total", Type: INT, PrimaryKey: false, Nullable: false, DefaultInt: sql.NullInt64{Valid: true, Int64: 0}},
						{sqlName: "total_used", goName: "TotalUsed", Type: INT, PrimaryKey: false, Nullable: false, DefaultInt: sql.NullInt64{Valid: true, Int64: 0}},
						{sqlName: "deactivated", goName: "Deactivated", Type: BOOL, PrimaryKey: false, Nullable: false, DefaultBool: sql.NullBool{Valid: true, Bool: false}},
					},
				},
			},
		},
		{
			"BUG: Parser fails on 'CREATE UNIQUE INDEX'",
			`CREATE TABLE accounts (
				id				   INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
				name			   TEXT NOT NULL,
				type			   INTEGER NOT NULL,
				total			   INTEGER NOT NULL DEFAULT 0,
				total_used		   INTEGER NOT NULL DEFAULT 0,
				deactivated        BOOLEAN NOT NULL DEFAULT FALSE
			);
			CREATE UNIQUE INDEX idx_account_name_unique_active ON account (name) WHERE deactivated = FALSE;
			`,
			false,
			[]*Table{
				{
					sqlName: "accounts",
					goName:  "Account",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: false, Nullable: false},
						{sqlName: "type", goName: "Type", Type: INT, PrimaryKey: false, Nullable: false},
						{sqlName: "total", goName: "Total", Type: INT, PrimaryKey: false, Nullable: false, DefaultInt: sql.NullInt64{Valid: true, Int64: 0}},
						{sqlName: "total_used", goName: "TotalUsed", Type: INT, PrimaryKey: false, Nullable: false, DefaultInt: sql.NullInt64{Valid: true, Int64: 0}},
						{sqlName: "deactivated", goName: "Deactivated", Type: BOOL, PrimaryKey: false, Nullable: false, DefaultBool: sql.NullBool{Valid: true, Bool: false}},
					},
				},
			},
		},
		{
			"BUG: partial index with empty-string literal in WHERE clause",
			`CREATE TABLE shared_services (
				id				INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
				source			TEXT NOT NULL,
				source_key		TEXT NOT NULL
			);
			CREATE UNIQUE INDEX idx_shared_services_source_key ON shared_services (source, source_key) WHERE source != '';
			`,
			false,
			[]*Table{
				{
					sqlName: "shared_services",
					goName:  "SharedService",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "source", goName: "Source", Type: TEXT, Nullable: false},
						{sqlName: "source_key", goName: "SourceKey", Type: TEXT, Nullable: false},
					},
				},
			},
		},
		{
			"inline Foreign Key with ON DELETE/UPDATE RESTRICT and NO ACTION",
			`CREATE TABLE parents (
				id		INTEGER NOT NULL PRIMARY KEY,
				name	TEXT NOT NULL UNIQUE
			);
			CREATE TABLE children (
				id				INTEGER NOT NULL PRIMARY KEY,
				parent_id		INTEGER NOT NULL REFERENCES parents (id) ON DELETE RESTRICT,
				guardian_id		INTEGER NOT NULL REFERENCES parents (id) ON DELETE NO ACTION,
				sponsor_id		INTEGER NOT NULL REFERENCES parents (id) ON UPDATE RESTRICT ON DELETE NO ACTION
			)`,
			false,
			[]*Table{
				{
					sqlName: "parents",
					goName:  "Parent",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "name", goName: "Name", Type: TEXT, Nullable: false},
					},
					UniqueConstraints: []UniqueConstraint{{Columns: []string{"name"}}},
				},
				{
					sqlName: "children",
					goName:  "Child",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "parent_id", goName: "ParentID", Type: INT, Nullable: false},
						{sqlName: "guardian_id", goName: "GuardianID", Type: INT, Nullable: false},
						{sqlName: "sponsor_id", goName: "SponsorID", Type: INT, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "parents", LocalColumns: []string{"parent_id"}, Columns: []string{"id"}, OnDelete: Restrict},
						{Table: "parents", LocalColumns: []string{"guardian_id"}, Columns: []string{"id"}, OnDelete: NoAction},
						{Table: "parents", LocalColumns: []string{"sponsor_id"}, Columns: []string{"id"}, OnUpdate: Restrict, OnDelete: NoAction},
					},
				},
			},
		},
		{
			"table-level Foreign Key clause with ON DELETE RESTRICT and ON UPDATE NO ACTION",
			`CREATE TABLE artist (
				name	TEXT NOT NULL PRIMARY KEY
			);
			CREATE TABLE track (
				id		INTEGER NOT NULL PRIMARY KEY,
				artist	TEXT NOT NULL,
				FOREIGN KEY (artist) REFERENCES artist (name) ON UPDATE NO ACTION ON DELETE RESTRICT
			)`,
			false,
			[]*Table{
				{
					sqlName: "artist",
					goName:  "Artist",
					Columns: []Column{
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: true, Nullable: false},
					},
				},
				{
					sqlName: "track",
					goName:  "Track",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "artist", goName: "Artist", Type: TEXT, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "artist", LocalColumns: []string{"artist"}, Columns: []string{"name"}, OnUpdate: NoAction, OnDelete: Restrict},
					},
				},
			},
		},
		{
			"table-level Foreign Key clause with quoted column names",
			`CREATE TABLE artist (
				name	TEXT NOT NULL PRIMARY KEY
			);
			CREATE TABLE track (
				id		INTEGER NOT NULL PRIMARY KEY,
				artist	TEXT NOT NULL,
				FOREIGN KEY ("artist") REFERENCES artist ("name") ON DELETE CASCADE
			)`,
			false,
			[]*Table{
				{
					sqlName: "artist",
					goName:  "Artist",
					Columns: []Column{
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: true, Nullable: false},
					},
				},
				{
					sqlName: "track",
					goName:  "Track",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "artist", goName: "Artist", Type: TEXT, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "artist", LocalColumns: []string{"artist"}, Columns: []string{"name"}, OnDelete: Cascade},
					},
				},
			},
		},
		{
			"inline Foreign Key with MATCH and DEFERRABLE clauses",
			`CREATE TABLE parents (
				id		INTEGER NOT NULL PRIMARY KEY
			);
			CREATE TABLE children (
				id			INTEGER NOT NULL PRIMARY KEY,
				parent_id	INTEGER NOT NULL REFERENCES parents (id) ON DELETE CASCADE MATCH SIMPLE DEFERRABLE INITIALLY DEFERRED
			)`,
			false,
			[]*Table{
				{
					sqlName: "parents",
					goName:  "Parent",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
					},
				},
				{
					sqlName: "children",
					goName:  "Child",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "parent_id", goName: "ParentID", Type: INT, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "parents", LocalColumns: []string{"parent_id"}, Columns: []string{"id"}, OnDelete: Cascade},
					},
				},
			},
		},
		{
			"table-level Foreign Key clause with MATCH and NOT DEFERRABLE clauses",
			`CREATE TABLE artist (
				name	TEXT NOT NULL PRIMARY KEY
			);
			CREATE TABLE track (
				id		INTEGER NOT NULL PRIMARY KEY,
				artist	TEXT NOT NULL,
				FOREIGN KEY (artist) REFERENCES artist (name) MATCH FULL ON UPDATE CASCADE NOT DEFERRABLE
			)`,
			false,
			[]*Table{
				{
					sqlName: "artist",
					goName:  "Artist",
					Columns: []Column{
						{sqlName: "name", goName: "Name", Type: TEXT, PrimaryKey: true, Nullable: false},
					},
				},
				{
					sqlName: "track",
					goName:  "Track",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "artist", goName: "Artist", Type: TEXT, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "artist", LocalColumns: []string{"artist"}, Columns: []string{"name"}, OnUpdate: Cascade},
					},
				},
			},
		},
		{
			"table-level composite Foreign Key",
			`CREATE TABLE parents (
				first_name	TEXT NOT NULL,
				last_name	TEXT NOT NULL,
				PRIMARY KEY (first_name, last_name)
			);
			CREATE TABLE children (
				id			INTEGER NOT NULL PRIMARY KEY,
				parent_first	TEXT NOT NULL,
				parent_last		TEXT NOT NULL,
				FOREIGN KEY (parent_first, parent_last) REFERENCES parents (first_name, last_name) ON DELETE CASCADE
			)`,
			false,
			[]*Table{
				{
					sqlName: "parents",
					goName:  "Parent",
					Columns: []Column{
						{sqlName: "first_name", goName: "FirstName", Type: TEXT, CompositePrimaryKey: true, Nullable: false},
						{sqlName: "last_name", goName: "LastName", Type: TEXT, CompositePrimaryKey: true, Nullable: false},
					},
				},
				{
					sqlName: "children",
					goName:  "Child",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "parent_first", goName: "ParentFirst", Type: TEXT, Nullable: false},
						{sqlName: "parent_last", goName: "ParentLast", Type: TEXT, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "parents", LocalColumns: []string{"parent_first", "parent_last"}, Columns: []string{"first_name", "last_name"}, OnDelete: Cascade},
					},
				},
			},
		},
		{
			"table with both a composite and a single-column Foreign Key",
			`CREATE TABLE parents (
				first_name	TEXT NOT NULL,
				last_name	TEXT NOT NULL,
				PRIMARY KEY (first_name, last_name)
			);
			CREATE TABLE schools (
				id		INTEGER NOT NULL PRIMARY KEY
			);
			CREATE TABLE children (
				id				INTEGER NOT NULL PRIMARY KEY,
				parent_first	TEXT NOT NULL,
				parent_last		TEXT NOT NULL,
				school_id		INTEGER NOT NULL REFERENCES schools (id) ON DELETE RESTRICT,
				FOREIGN KEY (parent_first, parent_last) REFERENCES parents (first_name, last_name) ON DELETE CASCADE
			)`,
			false,
			[]*Table{
				{
					sqlName: "parents",
					goName:  "Parent",
					Columns: []Column{
						{sqlName: "first_name", goName: "FirstName", Type: TEXT, CompositePrimaryKey: true, Nullable: false},
						{sqlName: "last_name", goName: "LastName", Type: TEXT, CompositePrimaryKey: true, Nullable: false},
					},
				},
				{
					sqlName: "schools",
					goName:  "School",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
					},
				},
				{
					sqlName: "children",
					goName:  "Child",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "parent_first", goName: "ParentFirst", Type: TEXT, Nullable: false},
						{sqlName: "parent_last", goName: "ParentLast", Type: TEXT, Nullable: false},
						{sqlName: "school_id", goName: "SchoolID", Type: INT, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						// The inline single-column FK is parsed before the table-level composite clause.
						{Table: "schools", LocalColumns: []string{"school_id"}, Columns: []string{"id"}, OnDelete: Restrict},
						{Table: "parents", LocalColumns: []string{"parent_first", "parent_last"}, Columns: []string{"first_name", "last_name"}, OnDelete: Cascade},
					},
				},
			},
		},
		{
			"line break: inline FK action continues on the next line",
			`CREATE TABLE parents (
				id	INTEGER NOT NULL PRIMARY KEY
			);
			CREATE TABLE children (
				id			INTEGER NOT NULL PRIMARY KEY,
				parent_id	INTEGER NOT NULL REFERENCES parents (id)
					ON DELETE CASCADE
			)`,
			false,
			[]*Table{
				{
					sqlName: "parents",
					goName:  "Parent",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
					},
				},
				{
					sqlName: "children",
					goName:  "Child",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "parent_id", goName: "ParentID", Type: INT, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "parents", LocalColumns: []string{"parent_id"}, Columns: []string{"id"}, OnDelete: Cascade},
					},
				},
			},
		},
		{
			"line break: table-level FK clause split across multiple lines",
			`CREATE TABLE parents (
				id	INTEGER NOT NULL PRIMARY KEY
			);
			CREATE TABLE children (
				id			INTEGER NOT NULL PRIMARY KEY,
				parent_id	INTEGER NOT NULL,
				FOREIGN KEY (parent_id)
					REFERENCES parents (id)
					ON DELETE CASCADE
			)`,
			false,
			[]*Table{
				{
					sqlName: "parents",
					goName:  "Parent",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
					},
				},
				{
					sqlName: "children",
					goName:  "Child",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "parent_id", goName: "ParentID", Type: INT, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "parents", LocalColumns: []string{"parent_id"}, Columns: []string{"id"}, OnDelete: Cascade},
					},
				},
			},
		},
		{
			"quoted identifiers containing spaces",
			`CREATE TABLE "user accounts" (
				"full name"	TEXT NOT NULL,
				"home town"	TEXT
			)`,
			false,
			[]*Table{
				{
					sqlName: "user accounts",
					goName:  "User Account",
					Columns: []Column{
						{sqlName: "full name", goName: "Full Name", Type: TEXT, Nullable: false},
						{sqlName: "home town", goName: "Home Town", Type: TEXT, Nullable: true},
					},
				},
			},
		},
		{
			"bracket and backtick quoted identifiers",
			"CREATE TABLE [user accounts] (\n\t\t\t\t`full name` TEXT NOT NULL\n\t\t\t)",
			false,
			[]*Table{
				{
					sqlName: "user accounts",
					goName:  "User Account",
					Columns: []Column{
						{sqlName: "full name", goName: "Full Name", Type: TEXT, Nullable: false},
					},
				},
			},
		},
		{
			"string default containing a space",
			`CREATE TABLE t (
				status	TEXT NOT NULL DEFAULT 'in progress'
			)`,
			false,
			[]*Table{
				{
					sqlName: "t",
					goName:  "T",
					Columns: []Column{
						{sqlName: "status", goName: "Status", Type: TEXT, Nullable: false, DefaultString: sql.NullString{Valid: true, String: "in progress"}},
					},
				},
			},
		},
		{
			"block comment attaches to a column",
			`CREATE TABLE t (
				id	INTEGER NOT NULL PRIMARY KEY /* the primary key */
			)`,
			false,
			[]*Table{
				{
					sqlName: "t",
					goName:  "T",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false, Comment: "the primary key"},
					},
				},
			},
		},
		{
			"case-insensitive keywords and types, with a quoted keyword as a column name",
			`create table parents (
				id integer not null primary key
			);
			create table children (
				id			integer not null primary key autoincrement,
				name		text unique default 'bob',
				"check"		text not null,
				parent_id	integer not null references parents (id) on delete cascade
			)`,
			false,
			[]*Table{
				{
					sqlName: "parents",
					goName:  "Parent",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
					},
				},
				{
					sqlName: "children",
					goName:  "Child",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "name", goName: "Name", Type: TEXT, Nullable: true, DefaultString: sql.NullString{Valid: true, String: "bob"}},
						// "check" is a quoted identifier, so it must be treated as a column name, NOT
						// the CHECK keyword that the table-constraint dispatch looks for.
						{sqlName: "check", goName: "Check", Type: TEXT, Nullable: false},
						{sqlName: "parent_id", goName: "ParentID", Type: INT, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Table: "parents", LocalColumns: []string{"parent_id"}, Columns: []string{"id"}, OnDelete: Cascade},
					},
					UniqueConstraints: []UniqueConstraint{{Columns: []string{"name"}}},
				},
			},
		},
		{
			"DEFAULT NULL leaves the column with no default value",
			`CREATE TABLE t (
				id		INTEGER NOT NULL PRIMARY KEY,
				note	TEXT    DEFAULT NULL,
				label	TEXT    DEFAULT null,
				count	INTEGER DEFAULT NULL
			)`,
			false,
			[]*Table{
				{
					sqlName: "t",
					goName:  "T",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						// DEFAULT NULL (any case) must NOT store the literal string "NULL" as the default.
						{sqlName: "note", goName: "Note", Type: TEXT, Nullable: true},
						{sqlName: "label", goName: "Label", Type: TEXT, Nullable: true},
						{sqlName: "count", goName: "Count", Type: INT, Nullable: true},
					},
				},
			},
		},
		{
			"signed integer DEFAULT values (negative, explicit positive, plain, zero)",
			`CREATE TABLE t (
				id		INTEGER NOT NULL PRIMARY KEY,
				balance	INTEGER NOT NULL DEFAULT -100,
				bonus	INTEGER DEFAULT +5,
				plain	INTEGER DEFAULT 7,
				zero	INTEGER DEFAULT 0,
				nodefault INTEGER DEFAULT NULL
			)`,
			false,
			[]*Table{
				{
					sqlName: "t",
					goName:  "T",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "balance", goName: "Balance", Type: INT, Nullable: false, DefaultInt: sql.NullInt64{Valid: true, Int64: -100}},
						{sqlName: "bonus", goName: "Bonus", Type: INT, Nullable: true, DefaultInt: sql.NullInt64{Valid: true, Int64: 5}},
						{sqlName: "plain", goName: "Plain", Type: INT, Nullable: true, DefaultInt: sql.NullInt64{Valid: true, Int64: 7}},
						{sqlName: "zero", goName: "Zero", Type: INT, Nullable: true, DefaultInt: sql.NullInt64{Valid: true, Int64: 0}},
						{sqlName: "nodefault", goName: "Nodefault", Type: INT, Nullable: true},
					},
				},
			},
		},
		{
			"signed REAL/FLOAT DEFAULT values (negative, explicit positive, plain, scientific)",
			`CREATE TABLE t (
				id		INTEGER NOT NULL PRIMARY KEY,
				temp	REAL NOT NULL DEFAULT -1.5,
				gain	REAL DEFAULT +2.5,
				ratio	REAL DEFAULT 0.25,
				scaled	REAL DEFAULT -2.5e3,
				whole	REAL DEFAULT 4,
				nodefault REAL DEFAULT NULL
			)`,
			false,
			[]*Table{
				{
					sqlName: "t",
					goName:  "T",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "temp", goName: "Temp", Type: FLOAT, Nullable: false, DefaultFloat: sql.NullFloat64{Valid: true, Float64: -1.5}},
						{sqlName: "gain", goName: "Gain", Type: FLOAT, Nullable: true, DefaultFloat: sql.NullFloat64{Valid: true, Float64: 2.5}},
						{sqlName: "ratio", goName: "Ratio", Type: FLOAT, Nullable: true, DefaultFloat: sql.NullFloat64{Valid: true, Float64: 0.25}},
						{sqlName: "scaled", goName: "Scaled", Type: FLOAT, Nullable: true, DefaultFloat: sql.NullFloat64{Valid: true, Float64: -2500}},
						{sqlName: "whole", goName: "Whole", Type: FLOAT, Nullable: true, DefaultFloat: sql.NullFloat64{Valid: true, Float64: 4}},
						{sqlName: "nodefault", goName: "Nodefault", Type: FLOAT, Nullable: true},
					},
				},
			},
		},
		{
			"named table-level FOREIGN KEY retains its name",
			`CREATE TABLE parents (
				id	INTEGER NOT NULL PRIMARY KEY
			);
			CREATE TABLE children (
				id			INTEGER NOT NULL PRIMARY KEY,
				parent_id	INTEGER NOT NULL,
				CONSTRAINT fk_parent FOREIGN KEY (parent_id) REFERENCES parents (id) ON DELETE CASCADE
			)`,
			false,
			[]*Table{
				{
					sqlName: "parents",
					goName:  "Parent",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
					},
				},
				{
					sqlName: "children",
					goName:  "Child",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "parent_id", goName: "ParentID", Type: INT, Nullable: false},
					},
					ForeignKeys: []*ForeignKey{
						{Name: "fk_parent", Table: "parents", LocalColumns: []string{"parent_id"}, Columns: []string{"id"}, OnDelete: Cascade},
					},
				},
			},
		},
		{
			"named and unnamed table-level CHECK constraints",
			`CREATE TABLE t (
				id		INTEGER NOT NULL PRIMARY KEY,
				age		INTEGER NOT NULL,
				score	INTEGER NOT NULL,
				CONSTRAINT ck_age CHECK (age >= 0),
				CHECK (score > age)
			)`,
			false,
			[]*Table{
				{
					sqlName: "t",
					goName:  "T",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "age", goName: "Age", Type: INT, Nullable: false},
						{sqlName: "score", goName: "Score", Type: INT, Nullable: false},
					},
					CheckConstraints: []CheckConstraint{
						{Name: "ck_age", Expr: "age >= 0"},
						{Name: "", Expr: "score > age"},
					},
				},
			},
		},
		{
			"named column-level constraints (PRIMARY KEY, UNIQUE, CHECK, FOREIGN KEY)",
			`CREATE TABLE parents (
				id	INTEGER NOT NULL PRIMARY KEY
			);
			CREATE TABLE t (
				id			INTEGER NOT NULL CONSTRAINT pk_t PRIMARY KEY,
				email		TEXT CONSTRAINT uc_email UNIQUE,
				age			INTEGER NOT NULL CONSTRAINT ck_age CHECK (age > 0),
				parent_id	INTEGER CONSTRAINT fk_parent REFERENCES parents (id) ON DELETE CASCADE
			)`,
			false,
			[]*Table{
				{
					sqlName: "parents",
					goName:  "Parent",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
					},
				},
				{
					sqlName: "t",
					goName:  "T",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "email", goName: "Email", Type: TEXT, Nullable: true},
						{sqlName: "age", goName: "Age", Type: INT, Nullable: false},
						{sqlName: "parent_id", goName: "ParentID", Type: INT, Nullable: true},
					},
					PrimaryKeyName:    "pk_t",
					UniqueConstraints: []UniqueConstraint{{Name: "uc_email", Columns: []string{"email"}}},
					CheckConstraints:  []CheckConstraint{{Name: "ck_age", Expr: "age > 0"}},
					ForeignKeys: []*ForeignKey{
						{Name: "fk_parent", Table: "parents", LocalColumns: []string{"parent_id"}, Columns: []string{"id"}, OnDelete: Cascade},
					},
				},
			},
		},
		{
			"multiple named constraints on a single column",
			`CREATE TABLE t (
				id		INTEGER NOT NULL PRIMARY KEY,
				email	TEXT CONSTRAINT nn NOT NULL CONSTRAINT uq UNIQUE
			)`,
			false,
			[]*Table{
				{
					sqlName: "t",
					goName:  "T",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "email", goName: "Email", Type: TEXT, Nullable: false},
					},
					UniqueConstraints: []UniqueConstraint{{Name: "uq", Columns: []string{"email"}}},
				},
			},
		},
		{
			"unnamed column-level CHECK is captured",
			`CREATE TABLE t (
				id	INTEGER NOT NULL PRIMARY KEY,
				age	INTEGER NOT NULL CHECK (age >= 18)
			)`,
			false,
			[]*Table{
				{
					sqlName: "t",
					goName:  "T",
					Columns: []Column{
						{sqlName: "id", goName: "ID", Type: INT, PrimaryKey: true, Nullable: false},
						{sqlName: "age", goName: "Age", Type: INT, Nullable: false},
					},
					CheckConstraints: []CheckConstraint{{Name: "", Expr: "age >= 18"}},
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
