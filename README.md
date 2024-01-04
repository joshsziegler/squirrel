# Squirrel

Squirrel is a bare-bones SQLite schema parser and Go access layer generator.
It allows you to parse the output of `sqlite3 my.db .dump` to produce a Go code.
Warning: This is very rough and not recommended for production use!

# Install

```bash
go install github.com/joshsziegler/squirrel@latest # Install this binary
go install go install mvdan.cc/gofumpt@latest      # Install gofumpt for formatting output
```

## To Do

- [ ] Multi-column Primary Keys (on their own line)
- [ ] Foreign Keys defined on their own line
- [ ] Multi-column Foreign Keys (on their own line)
- [ ] Defaults using expressions, such as (datetime('now'))
- [ ] Indexes
- [ ] Multi-column Unique constraints
- [ ] Check constraints (e.g. CHECK( type IN ('special', 'user-defined')))
- [ ] Named constraints (e.g. CONSTRAINT uc_owner_channel UNIQUE (fk_owner_id, fk_channel_id))
