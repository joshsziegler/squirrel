# Squirrel

Squirrel is a bare-bones SQLite schema parser and Go access layer generator.
Warning: This is very rough and not recommended for production use!

# Install

```bash
go install github.com/joshsziegler/squirrel@latest # Install this binary
go install go install mvdan.cc/gofumpt@latest      # Install gofumpt for formatting output
```

# Using

```bash
$ squirrel
Usage: squirrel schema dest pkg
- schema: Path to the SQL schema you wish to parse (e.g. schema.sql)
- dest: Path to write the resulting Go-to-SQL layer to (e.g. db.go)
- pkg: Package name to use in the resulting Go code (e.g. db)
```

# Developing

Typically, running `make test` or `make build` after your changes is enough, but the `Makefile` has more.
Please run `make pre-commit` before committing and especially before creating merge requests.

## To Do

- [ ] Defaults using expressions, such as `(datetime('now')`)
- [ ] Indices
- [ ] Multi-column `Unique` constraints
- [ ] Check constraints (e.g. `CHECK( type IN ('special', 'user-defined'))`)
- [ ] Named constraints (e.g. `CONSTRAINT uc_owner_channel UNIQUE (fk_owner_id, fk_channel_id)`)
- [ ] Add support for DB-provided timestamps such as `created_at`, `updated_at`, and `deleted_at`
- [ ] Triggers
- [ ] Add option to include or exclude rows that have been soft-deleted (i.e. `deleted_at`)
