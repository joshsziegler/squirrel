package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTemp writes content to a temp file and returns its path.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "squirrel.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestLoad_Valid(t *testing.T) {
	path := writeTemp(t, `
schema: schema.sql
dest: db.go
package: db
ignore_tables:
  - goose_db_version
  - users
ctx_only: true
acronyms:
  dns: DNS
  oauth: OAuth
`)
	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "schema.sql", cfg.Schema)
	assert.Equal(t, "db.go", cfg.Dest)
	assert.Equal(t, "db", cfg.Package)
	assert.Equal(t, []string{"goose_db_version", "users"}, cfg.IgnoreTables)
	assert.True(t, cfg.CtxOnly)
	assert.Equal(t, map[string]string{"dns": "DNS", "oauth": "OAuth"}, cfg.Acronyms)
}

func TestLoad_CtxOnlyDefaultsTrue(t *testing.T) {
	path := writeTemp(t, `
schema: schema.sql
dest: db.go
package: db
`)
	cfg, err := Load(path)
	require.NoError(t, err)
	assert.True(t, cfg.CtxOnly, "ctx_only should default to true when omitted")
}

func TestLoad_CtxOnlyFalseHonored(t *testing.T) {
	path := writeTemp(t, `
schema: schema.sql
dest: db.go
package: db
ctx_only: false
`)
	cfg, err := Load(path)
	require.NoError(t, err)
	assert.False(t, cfg.CtxOnly, "ctx_only: false should be honored")
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	assert.Error(t, err)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"valid", Config{Schema: "s.sql", Dest: "db.go", Package: "db"}, false},
		{"missing schema", Config{Dest: "db.go", Package: "db"}, true},
		{"missing dest", Config{Schema: "s.sql", Package: "db"}, true},
		{"missing package", Config{Schema: "s.sql", Dest: "db.go"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
