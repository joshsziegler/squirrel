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
