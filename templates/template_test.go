package templates

import (
	"bytes"
	"strings"
	"testing"

	"github.com/joshsziegler/squirrel/name"
	"github.com/joshsziegler/squirrel/parser"
)

// generate parses schema SQL and returns the generated Go as a string. It mirrors the real
// SQL-parsing-to-Go-generation pipeline used by GenerateGoFromSQL.
func generate(t *testing.T, schema string) string {
	t.Helper()
	tables, err := parser.Parse(schema)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	Write(&buf, "db", tables, nil, true)
	return buf.String()
}

func assertContains(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Errorf("expected generated output to contain %q:\n%s", want, out)
	}
}

func assertNotContains(t *testing.T, out, notWant string) {
	t.Helper()
	if strings.Contains(out, notWant) {
		t.Errorf("expected generated output NOT to contain %q:\n%s", notWant, out)
	}
}

// TestGenerate_Acronyms verifies the full SQL-to-Go path keeps acronyms uppercase and does not
// singularize them: built-in acronyms work out of the box, and user-registered acronyms (from the
// config's "acronyms" map) take effect for both table and column names.
func TestGenerate_Acronyms(t *testing.T) {
	// Without registration, "dns" is not an acronym, so dns_zones would singularize to the
	// incorrect "DnZone". Register it (as the config loader does) to fix that.
	name.RegisterAcronyms(map[string]string{"dns": "DNS"})

	out := generate(t, `
CREATE TABLE aws_nodes (
	id       INTEGER NOT NULL PRIMARY KEY,
	dns_name TEXT NOT NULL
);

CREATE TABLE dns_zones (
	id   INTEGER NOT NULL PRIMARY KEY,
	name TEXT NOT NULL
);
`)

	// Built-in acronym ("aws") on a table name, singularized: aws_nodes -> AWSNode.
	if !strings.Contains(out, "type AWSNode struct") {
		t.Errorf("expected struct AWSNode in generated output:\n%s", out)
	}
	// User-registered acronym on a column name: dns_name -> DNSName (not "DnName").
	if !strings.Contains(out, "DNSName string `db:\"dns_name\"`") {
		t.Errorf("expected field DNSName in generated output:\n%s", out)
	}
	// User-registered acronym on a table name, singularized: dns_zones -> DNSZone (not "DnZone").
	if !strings.Contains(out, "type DNSZone struct") {
		t.Errorf("expected struct DNSZone in generated output:\n%s", out)
	}
	// The un-registered, singularized form must not leak into any generated Go name.
	if strings.Contains(out, "DnZone") {
		t.Errorf("found incorrectly singularized DnZone in generated output:\n%s", out)
	}
}

// TestGenerate_SingleColumnConstraintComments verifies that single-column PK, UNIQUE, and
// FOREIGN KEY constraints are surfaced as comments on the generated struct field (since v1.11.0
// constraint names are stored, and single-column FKs/PKs/uniques are annotated per column).
func TestGenerate_SingleColumnConstraintComments(t *testing.T) {
	out := generate(t, `
CREATE TABLE users (
	id    INTEGER NOT NULL PRIMARY KEY,
	email TEXT NOT NULL UNIQUE
);

CREATE TABLE posts (
	id      INTEGER NOT NULL PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE
);
`)

	assertContains(t, out, "ID int64 `db:\"id\"` // PK")
	assertContains(t, out, "Email string `db:\"email\"` // Unique")
	assertContains(t, out, "UserID int64 `db:\"user_id\"` // FK: users.id")
}

// TestGenerate_CompositePrimaryKey verifies a named, multi-column PRIMARY KEY is surfaced as a
// struct-level comment naming the constraint and every column (v1.11.0 / v1.11.2).
func TestGenerate_CompositePrimaryKey(t *testing.T) {
	out := generate(t, `
CREATE TABLE albums (
	artist      TEXT NOT NULL,
	album_title TEXT NOT NULL,
	year        INT NOT NULL,
	CONSTRAINT author_book PRIMARY KEY (artist, album_title)
);
`)

	assertContains(t, out, "// Composite PK author_book: artist, album_title")
	// The composite-PK columns must not be mislabeled as single-column PKs at the field level.
	assertNotContains(t, out, "Artist string `db:\"artist\"` // PK")
}

// TestGenerate_CompositeUniqueConstraints verifies multi-column UNIQUE constraints are surfaced at
// the struct level, both named and unnamed (v1.11.0 stores their names; v1.11.2 moves them to the
// struct level so every involved column can be listed).
func TestGenerate_CompositeUniqueConstraints(t *testing.T) {
	out := generate(t, `
CREATE TABLE memberships (
	id      INTEGER NOT NULL PRIMARY KEY,
	org_id  INTEGER NOT NULL,
	user_id INTEGER NOT NULL,
	CONSTRAINT uc_org_user UNIQUE (org_id, user_id)
);

CREATE TABLE players (
	id             INTEGER NOT NULL PRIMARY KEY,
	server         INT NOT NULL,
	character_name TEXT NOT NULL,
	UNIQUE (server, character_name)
);
`)

	assertContains(t, out, "// Composite Unique uc_org_user: org_id, user_id")
	assertContains(t, out, "// Composite Unique: server, character_name")
}

// TestGenerate_CompositeForeignKey verifies a table-level multi-column FOREIGN KEY is surfaced as a
// struct-level comment mapping the local columns to the referenced table and columns (composite FK
// support plus v1.11.2's move of composite comments to the struct level).
func TestGenerate_CompositeForeignKey(t *testing.T) {
	out := generate(t, `
CREATE TABLE parents (
	first_name TEXT NOT NULL,
	last_name  TEXT NOT NULL,
	PRIMARY KEY (first_name, last_name)
);

CREATE TABLE children (
	id           INTEGER NOT NULL PRIMARY KEY,
	parent_first TEXT NOT NULL,
	parent_last  TEXT NOT NULL,
	FOREIGN KEY (parent_first, parent_last) REFERENCES parents (first_name, last_name) ON DELETE CASCADE
);
`)

	assertContains(t, out, "// Composite FK: (parent_first, parent_last) -> parents (first_name, last_name)")
}

// TestGenerate_FloatAndSignedDefaults verifies REAL/FLOAT columns map to float64 and that signed
// numeric defaults are captured in the field comment (added since v1.8.0).
func TestGenerate_FloatAndSignedDefaults(t *testing.T) {
	out := generate(t, `
CREATE TABLE items (
	id         INTEGER NOT NULL PRIMARY KEY,
	weight     REAL NOT NULL,
	adjustment INTEGER NOT NULL DEFAULT -100,
	ratio      REAL NOT NULL DEFAULT -1.5
);
`)

	assertContains(t, out, "Weight float64 `db:\"weight\"`")
	assertContains(t, out, "Ratio float64 `db:\"ratio\"`")
	assertContains(t, out, "// Default: -100")
	assertContains(t, out, "// Default: -1.5")
}

// TestGenerate_EmptyStringDefault is a regression test for v1.11.1: a DEFAULT set to an empty-string
// literal must not be treated as the end of the SQL, so columns and tables that follow it are still
// parsed and generated.
func TestGenerate_EmptyStringDefault(t *testing.T) {
	out := generate(t, `
CREATE TABLE products (
	id          INTEGER NOT NULL PRIMARY KEY,
	description TEXT NOT NULL DEFAULT '',
	name        TEXT NOT NULL
);

CREATE TABLE tags (
	id   INTEGER NOT NULL PRIMARY KEY,
	name TEXT NOT NULL
);
`)

	// The column carrying the empty-string default is generated.
	assertContains(t, out, "Description string `db:\"description\"`")
	// Parsing continued past DEFAULT '': the following column and the following table both exist.
	assertContains(t, out, "type Product struct")
	assertContains(t, out, "type Tag struct")
}

// TestGenerate_CtxOnly verifies the ctx_only config flag controls whether the generated DB
// interface includes the non-context method set (v1.8.0).
func TestGenerate_CtxOnly(t *testing.T) {
	schema := `
CREATE TABLE users (
	id   INTEGER NOT NULL PRIMARY KEY,
	name TEXT NOT NULL
);
`
	tables, err := parser.Parse(schema)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var ctxOnly bytes.Buffer
	Write(&ctxOnly, "db", tables, nil, true)
	assertContains(t, ctxOnly.String(), "ExecContext(ctx context.Context")
	assertNotContains(t, ctxOnly.String(), "Exec(query string")

	var both bytes.Buffer
	Write(&both, "db", tables, nil, false)
	assertContains(t, both.String(), "ExecContext(ctx context.Context")
	assertContains(t, both.String(), "Exec(query string")
}
