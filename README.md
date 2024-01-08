# Squirrel

Squirrel is a bare-bones SQLite schema parser and Go access layer generator.
Warning: This is very rough and not recommended for production use!

# Install

```bash
go install github.com/joshsziegler/squirrel@latest # Install this binary
go install go install mvdan.cc/gofumpt@latest      # Install gofumpt for formatting output
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
