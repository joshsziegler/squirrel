package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// consume N tokens and return the result
func consume(n int, tokens []string) []string {
	return tokens[n:] // Consume ONE token
}

func consumeNewLines(tokens *Tokens) {
	for {
		if tokens.Next() == "\n" {
			tokens.Take()
			continue
		}
		return
	}
}

// Parse SQL string to a set of tables with column definitions.
func Parse(sql string) ([]*Table, error) {
	tokens := Lex(sql)
	tables := make([]*Table, 0)
	for {
		consumeNewLines(tokens)
		if tokens.Next() == "" {
			break
		}
		table, err := parseCreateTable(tokens)
		if err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, nil
}

// removeQuotes surrounding the provided string.
func removeQuotes(name string) string {
	return strings.Trim(name, `'"`)
}

// parseComment removes the quote starting with the first token, or returns as-is.
//
// SQLite Docs: https://www.sqlite.org/lang_comment.html
//
// TODO: Handle multi-line comments.
// TODO: Handle comments anywhere whitespace may occur, per the SQLite specification.
func parseComment(tokens *Tokens) string {
	comment := make([]string, 0)
	if tokens.Next() == "--" {
		tokens.Take()
		for {
			t := tokens.Next()
			if t == "\n" {
				break
			}
			comment = append(comment, t)
			tokens.Take() // Consume token as part of comment
		}
	}
	return strings.Join(comment, " ")
}

// parseCreateTable
// https://www.sqlite.org/syntax/create-table-stmt.html
func parseCreateTable(tokens *Tokens) (*Table, error) {
	t := &Table{
		Temp:        false,
		IfNotExists: false,
		Columns:     make([]Column, 0),
	}

	if tokens.Take() != "CREATE" {
		return nil, fmt.Errorf("create table must begin with 'CREATE', not %s", tokens.Next())
	}
	// Temporary
	if tokens.Next() == "TEMP" || tokens.Next() == "TEMPORARY" {
		t.Temp = true
		tokens.Take()
	}
	if tokens.Take() != "TABLE" {
		return nil, fmt.Errorf("create table must begin with 'CREATE [TEMP|TEMPORARY] TABLE', not %s", tokens.Next())
	}
	// If not exists
	if tokens.Next() == "IF" {
		if tokens.NextN(3) == "IF NOT EXISTS" {
			tokens.TakeN(3)
			t.IfNotExists = true
		} else {
			return nil, fmt.Errorf("create table must use 'IF NOT EXISTS' when 'IF' is present, not %s", tokens.NextN(3))
		}
	}
	// Schema and Table Name (i.e. Schema.TableName)
	if tokens.Peek(1) == "." {
		t.SchemaName = removeQuotes(tokens.Take())
		tokens.Take() // period delimiter
		t.Name = removeQuotes(tokens.Take())
	} else {
		t.Name = removeQuotes(tokens.Take())
	}
	// TODO: Handle "AS SELECT STMT" ?
	// Opening parenthesis for column definitions
	if tokens.Next() == "(" {
		tokens.Take()
	}
	// Comments
	t.Comment = parseComment(tokens)

	// Column(s)
	for {
		switch tokens.Next() {
		case ")": // End of Table Definition
			tokens.Take()
			if tokens.Next() == ";" { // Optional semicolon
				tokens.Take()
			}
			return t, nil
		case "": // Table wasn't closed properly or something went wrong
			return nil, fmt.Errorf("ran out of tokens unexpectedly - table was likely not closed properly")
		case "\n": // Newline
			tokens.Take()
		case ",":
			tokens.Take()
		case "--": // Comment
			parseComment(tokens)
		case "CONSTRAINT": // Named table constraint
			parseTableConstraint(tokens)
		case "PRIMARY": // table-constraint
			parseTableConstraint(tokens)
		case "UNIQUE": // table-constraint
			parseTableConstraint(tokens)
		case "CHECK": // table-constraint
			parseTableConstraint(tokens)
		case "FOREIGN": // table-constraint
			parseTableConstraint(tokens)
		default: // column-def
			col, err := parseColumn(tokens)
			if err != nil {
				return nil, err
			}
			t.Columns = append(t.Columns, col)
		}
	}
}

// parseColumn
func parseColumn(tokens *Tokens) (Column, error) {
	c := Column{
		PrimaryKey: false,
		Nullable:   true,
		Unique:     false,
	}

	// Name
	c.Name = removeQuotes(tokens.Take())
	// Data Type
	err := parseColumnDataType(tokens, &c, true)
	if err != nil {
		return c, err
	}
	// Constraints
	// SQlite Docs: https://www.sqlite.org/syntax/column-constraint.html
	for {
		token := tokens.Next()
		if token == "," { // End of Column Definition (check for optional comment)
			tokens.Take()
			if tokens.Next() == "--" {
				c.Comment = parseComment(tokens)
			}
			break
		} else if token == ")" { // End of table definition. DO NOT CONSUME TOKEN
			break
		} else if token == "\n" {
			tokens.Take()
		} else if token == "--" {
			c.Comment = parseComment(tokens)
		} else if token == "NOT" {
			if tokens.NextN(2) != "NOT NULL" {
				return c, fmt.Errorf("column constraint must be 'NOT NULL', not %s", tokens.NextN(2))
			}
			// TODO: Handle [conflict-clause]
			c.Nullable = false
			tokens.TakeN(2)
		} else if token == "PRIMARY" {
			if tokens.NextN(2) != "PRIMARY KEY" {
				return c, fmt.Errorf("column constraint must be 'PRIMARY KEY', not %s", tokens.NextN(2))
			}
			// TODO: Handle [ASC|DESC] and [conflict-clause]
			c.PrimaryKey = true
			tokens.TakeN(2)
		} else if token == "AUTOINCREMENT" {
			if c.PrimaryKey {
				tokens.Take() // Consume ONE token
			} else {
				return c, errors.New("column constraint 'AUTOINCREMENT' must follow 'PRIMARY KEY'")
			}
		} else if token == "UNIQUE" {
			// TODO: Handle [conflict-clause]
			c.Unique = true
			tokens.Take() // Consume ONE token
		} else if token == "REFERENCES" { // Foreign Key
			err = parseForeignKey(tokens, &c)
			if err != nil {
				return c, err
			}
		} else if token == "DEFAULT" {
			tokens.Take()
			token := tokens.Take()
			if token == "" {
				return c, fmt.Errorf("column DEFAULT must be followed by a constant value or expression, not %s", token)
			}
			switch c.Type {
			case "int64":
				c.DefaultInt, err = strconv.ParseInt(token, 10, 64) // Should not have quotes
				if err != nil {
					return c, fmt.Errorf("default value for INT/INTEGER must be a valid base 10 integer, not %s", token)
				}
			case "string":
				c.DefaultString = removeQuotes(token)
			case "bool":
				switch token {
				case "true", "TRUE", "1":
					c.DefaultBool = true
				case "false", "FALSE", "0":
					c.DefaultBool = false
				default:
					return c, fmt.Errorf("default value for BOOL/BOOLEAN must be TRUE/true/1 or FALSE/false/0, not %s", token)
				}
			default:
				return c, fmt.Errorf("default values for %s type are not supported", c.Type)
			}
		} else {
			return c, fmt.Errorf("unrecognized column constraint starting with: %s", tokens.NextN(5))
		}
	}

	return c, nil
}

// Column Type
// - strict: Only support SQLite's STRICT data types (i.e. INT, INTEGER, REAL, TEXT, BLOB, ANY).
//
// SQLite Docs: https://www.sqlite.org/datatype3.html
// mattn/go-sqlit3 Docs: https://pkg.go.dev/github.com/mattn/go-sqlite3#hdr-Supported_Types
func parseColumnDataType(tokens *Tokens, c *Column, strict bool) error {
	switch tokens.Next() {
	case "INT":
		fallthrough
	case "INTEGER":
		c.Type = "int64"
		tokens.Take()
	case "BOOL":
		fallthrough
	case "BOOLEAN": // SQLite does not have a bool or boolean type and represents them as integers (0 and 1) internally.
		c.Type = "bool"
		tokens.Take()
	case "FLOAT": // TODO: This isn't a valid SQLite type.
		fallthrough
	case "REAL":
		c.Type = "float64"
		tokens.Take()
	case "TEXT":
		c.Type = "string"
		tokens.Take()
	case "BLOB":
		fallthrough
	case "ANY":
		c.Type = "[]byte"
		tokens.Take()
	case "DATETIME":
		fallthrough
	case "TIMESTAMP":
		c.Type = "time.Time"
		tokens.Take()
	default:
		if strict {
			return fmt.Errorf("column '%s' must use a valid type, not %s", c.Name, tokens.Next())
		}
		c.Type = tokens.Next()
		tokens.Take()
	}
	return nil
}

// parseForeignKey assumes the Next() token is "REFERENCES" and takes over parsing the rest of the parsing.
func parseForeignKey(tokens *Tokens, c *Column) error {
	if tokens.Peek(1) == "" {
		return fmt.Errorf("column foreign key constraint 'REFERENCES' must be followed by a table name, not %s", tokens.NextN(2))
	}
	tokens.Take()
	// TODO: Handle [conflict-clause]
	c.ForeignKey = &ForeignKey{Table: tokens.Take()}
	if tokens.Next() == "(" && tokens.Peek(2) == ")" { // If the column is specified
		c.ForeignKey.ColumnName = tokens.Peek(1)
		tokens.TakeN(3)
	} else {
		c.ForeignKey.ColumnName = c.Name // Not specified, so it should be the same name as THIS column name.
	}
	// ON UPDATE/ON DELETE (can have multiple)
	for {
		if tokens.Next() != "ON" {
			break
		}
		err := parseFkAction(tokens, c.ForeignKey)
		if err != nil {
			return err
		}
	}
	return nil
}

// foreign-key-clause
// Note this is different from an inline FK definition, and can reference multiple columns.
// https://www.sqlite.org/syntax/foreign-key-clause.html
//
// TODO: Handle rest of foreign-key-clause, including MATCH, [NOT] DEFERRABLE, and multiple columns
func parseForeignKeyClause(tokens *Tokens) (*ForeignKey, error) {
	fk := &ForeignKey{}
	// if tokens.Take() != "REFERENCES" {
	// 	tokens.Return()
	// 	return nil, fmt.Errorf("foreign key clause must start with 'REFERENCES table-name', not %s", tokens.NextN(2))
	// }
	cols := parseColumnList(tokens)
	if len(cols) > 1 {
		return nil, fmt.Errorf("multiple columns are not currently supported in a foreign key clause: %v", cols)
	}
	fk.ColumnName = cols[0]
	err := parseFkAction(tokens, fk)
	if err != nil {
		return nil, err
	}
	return fk, nil
}

// TODO: Handle ON (UPDATE|DELETE) RESTRICT
// TODO: Handle ON (UPDATE|DELETE) NO ACTION
func parseFkAction(tokens *Tokens, fk *ForeignKey) error {
	switch {
	case tokens.NextN(3) == "ON DELETE CASCADE":
		tokens.TakeN(3)
		fk.OnDelete = Cascade
	case tokens.NextN(3) == "ON UPDATE CASCADE":
		tokens.TakeN(3)
		fk.OnUpdate = Cascade
	case tokens.NextN(4) == "ON DELETE SET NULL":
		tokens.TakeN(4)
		fk.OnDelete = SetNull
	case tokens.NextN(4) == "ON UPDATE SET NULL":
		tokens.TakeN(4)
		fk.OnUpdate = SetNull
	case tokens.NextN(4) == "ON DELETE SET DEFAULT":
		tokens.TakeN(4)
		fk.OnDelete = SetDefault
	case tokens.NextN(4) == "ON UPDATE SET DEFAULT":
		tokens.TakeN(4)
		fk.OnUpdate = SetDefault
	default:
		return fmt.Errorf("column foreign key constraint \"%s\" not supported", tokens.NextN(4))
	}
	return nil
}

// table-constraint
// https://www.sqlite.org/syntax/table-constraint.html
func parseTableConstraint(tokens *Tokens) error {
	name := ""
	switch {
	case tokens.Next() == "CONSTRAINT": // Named table constraint
		name = tokens.Take()
		fmt.Printf("Constraint: %s\n", tokens.Take())
	case tokens.NextN(2) == "PRIMARY KEY": // table-constraint
		tokens.TakeN(2) // PRIMARY KEY
		cols := parseIndexedColumn(tokens)
		// TODO: Handle conflict clause
		fmt.Printf("Primary Key '%s' Columns: %v\n", name, cols)
	case tokens.Next() == "UNIQUE": // table-constraint
		tokens.Take()
		cols := parseIndexedColumn(tokens)
		// TODO: Handle conflict clause
		fmt.Printf("Unique '%s' Columns %v\n", name, cols)
	case tokens.Next() == "CHECK": // table-constraint
	case tokens.NextN(2) == "FOREIGN KEY": // table-constraint
		tokens.TakeN(2)
		// TODO: Handle foreign-key-clause
		// cols := parseColumnList(tokens)
		fk, err := parseForeignKeyClause(tokens)
		if err != nil {
			return err
		}
		fmt.Printf("Foreign Key '%s' Foreign Key: %+v\n", name, fk)
	}
	return nil
}

// indexed-column
// https://www.sqlite.org/syntax/indexed-column.html
// TODO: Handle rest of indexed-column, and not just column list: [expr] [COLLATION collation-name] [ASC|DESC]
func parseIndexedColumn(tokens *Tokens) []string {
	return parseColumnList(tokens)
}

// ( column [, column]+ )
// TODO: Handle [expr] [COLLATION collation-name] [ASC|DESC]
func parseColumnList(tokens *Tokens) []string {
	if tokens.Take() != "(" {
		tokens.Return()
		return nil
	}
	cols := []string{}
	t := tokens.Next()
	for {
		if t == ")" { // End of column list
			tokens.Take()
			break
		} else if t == "," { // Column Separator
			tokens.Take()
		} else { // Column Name
			cols = append(cols, tokens.Take())
		}
		t = tokens.Next()
	}
	return cols
}
