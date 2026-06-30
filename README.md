# Squirrel

Squirrel is a bare-bones SQLite schema parser and Go access layer generator.
Warning: This is very rough and not recommended for production use!

# Install

```bash
go install github.com/joshsziegler/squirrel@latest # Install this binary
go install go install mvdan.cc/gofumpt@latest      # Install gofumpt for formatting output
```

# Using

Squirrel reads its settings from a YAML config file. By default it looks for
`squirrel.yaml` in the current directory; use `-config <path>` to point it
elsewhere.

```bash
$ squirrel                   # uses ./squirrel.yaml
$ squirrel -config build/sq.yaml
```

Example `squirrel.yaml`:

```yaml
schema: schema.sql        # Path to the SQL schema to parse (required)
dest: db.go               # Path to write the generated Go to (required)
package: db               # Package name for the generated Go (required)
ignore_tables:            # Tables to parse but exclude from the generated Go
  - goose_db_version
  - users
ctx_only: true            # Only emit context-aware DB methods (optional, default: true)
acronyms:                 # SQL word -> Go form kept uppercase and not singularized (optional)
  dns: DNS                #   e.g. dns_zones -> DNSZone
  ldap: LDAP
  oauth: OAuth            #   the value is emitted verbatim, so mixed-case forms work too
```

Squirrel already knows a handful of common acronyms (`id`, `cpu`, `gpu`, `aws`,
`ssl`, `url`, `ip`, `pid`, `uid`, `gid`, `os`). The `acronyms` map merges with
and overrides those defaults. Keys are matched case-insensitively against each
underscore-separated word; without an entry a word like `dns` would be
singularized to the incorrect `Dn`.

# Developing

Typically, running `make test` or `make build` after your changes is enough, but the `Makefile` has more.
Please run `make pre-commit` before committing and especially before creating merge requests.

## To Do

- [ ] Defaults using expressions, such as `(datetime('now'))` or `(-5)`
- [ ] Indices
- [ ] Use CHECK constraint expressions in generation (CHECK constraints are now parsed and captured, but unused)
- [ ] Add support for DB-provided timestamps such as `created_at`, `updated_at`, and `deleted_at`
- [ ] Triggers
- [ ] Add option to include or exclude rows that have been soft-deleted (i.e. `deleted_at`)
- [ ] Conflict clauses (e.g. `ON CONFLICT ...` on `NOT NULL`, `PRIMARY KEY`, `UNIQUE`, and foreign keys)
- [ ] Generated/computed columns (e.g. `total AS (qty * price) STORED`)
- [ ] `CREATE TABLE ... AS SELECT`
- [ ] `COLLATE`, and `ASC`/`DESC` on columns and indexed columns
- [ ] Reject identifiers that are reserved SQLite keywords (validation currently unused)

## Done

- [x] Signed numeric defaults and REAL/FLOAT defaults (e.g. `DEFAULT -100`, `DEFAULT +5`, `DEFAULT -1.5`, `DEFAULT 2.5e3`)
- [x] Multi-column `Unique` constraints (e.g. `UNIQUE (a, b)`)
- [x] Named UNIQUE constraints (e.g. `CONSTRAINT uc_owner_channel UNIQUE (fk_owner_id, fk_channel_id)`)
- [x] Retain names on table-level `PRIMARY KEY`, `FOREIGN KEY`, and `CHECK` constraints
- [x] Named inline column constraints (e.g. `email TEXT CONSTRAINT uc_email UNIQUE`), including column-level CHECK capture
- [x] Foreign key clauses spanning multiple lines (the `ON`, `MATCH`, and `DEFERRABLE` clauses may now wrap across lines)
- [x] Multi-column (composite) foreign keys (e.g. `FOREIGN KEY (a, b) REFERENCES t (x, y)`)
- [x] Alternative identifier quoting: `[name]` and `` `name` ``
- [x] Quoted identifiers or string literals containing whitespace (e.g. `"first name"`, `DEFAULT 'in progress'`)
