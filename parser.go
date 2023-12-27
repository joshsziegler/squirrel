package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Parse SQL string to a set of tables with column definitions.
func Parse(sql string) ([]*Table, error) {
	tokens := Lex(sql)
	tables := make([]*Table, 0)
	for {
		switch {
		case tokens.Next() == "\n":
			tokens.Take()
		case tokens.Next() == "": // End of SQL
			return tables, nil

		case tokens.NextN(2) == "CREATE TABLE": // FIXME: This won't catch TEMPORARY or TEMP
			table, err := parseCreateTable(tokens)
			if err != nil {
				printContext(tokens, err)
				return nil, err
			}
			tables = append(tables, table)
		case tokens.NextN(2) == "CREATE INDEX": // FIXME: This won't catch UNIQUE indices
			// https://www.sqlite.org/syntax/create-index-stmt.html
			err := parseCreateIndex(tokens)
			if err != nil {
				printContext(tokens, err)
				return nil, err
			}
		default:
			err := fmt.Errorf("unsupported statement: %s", tokens.NextN(3))
			printContext(tokens, err)
			return nil, err
		}
	}
}

func printContext(tokens *Tokens, err error) {
	tokens.ReturnN(10)
	fmt.Println(err)
	fmt.Println("...")
	fmt.Println(tokens.NextN(20))
	fmt.Println("...")
}

func removeNewlines(tokens *Tokens) {
	for {
		if tokens.Next() == "\n" {
			tokens.Take()
			continue
		}
		return
	}
}

// removeQuotes surrounding the provided string -- both single and double quotes -- but only if they match.
func removeQuotes(s string) string {
	l := len(s)
	if l < 2 {
		return s
	}
	if (s[0] == '"' && s[l-1] == '"') || (s[0] == '\'' && s[l-1] == '\'') {
		return s[1 : l-1]
	}
	return s
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
		} else if token == "CHECK" { // CHECK column constraint
			parseCheckConstraint(tokens)
		} else if token == "DEFAULT" {
			tokens.Take()
			switch c.Type {
			case "int64":
				token := tokens.Take()
				if token != "NULL" { // TODO: How to mark explicitly as DEFAULTS TO NULL?
					val, err := strconv.ParseInt(token, 10, 64) // Should not have quotes
					if err != nil {
						return c, fmt.Errorf("default value for INT/INTEGER must be a valid base 10 integer or NULL, not %s", token)
					}
					c.DefaultInt = sql.NullInt64{Valid: true, Int64: val}
				}
			case "string":
				if token != "NULL" {
					token := tokens.Take()
					c.DefaultString = sql.NullString{Valid: true, String: removeQuotes(token)}
				}
			case "bool":
				token := tokens.Take()
				switch token {
				case "NULL":
					// TODO: How to mark explicitly as DEFAULTS TO NULL?
				case "true", "TRUE", "1":
					c.DefaultBool = sql.NullBool{Valid: true, Bool: true}
				case "false", "FALSE", "0":
					c.DefaultBool = sql.NullBool{Valid: true, Bool: false}
				default:
					return c, fmt.Errorf("default value for BOOL/BOOLEAN must be TRUE/true/1 or FALSE/false/0, not %s", token)
				}
			case "time.Time":
				parseDatetimeDefault(tokens)
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
	cols := parseColumnList(tokens)
	if len(cols) > 1 {
		return nil, fmt.Errorf("multiple columns are not currently supported in a foreign key clause: %v", cols)
	}
	fk.ColumnName = cols[0] // FROM-COLUMN
	removeNewlines(tokens)
	if tokens.Take() != "REFERENCES" {
		tokens.Return()
		return nil, fmt.Errorf("foreign key clause must contain 'REFERENCES table-name', not %s", tokens.NextN(2))
	}
	fk.Table = tokens.Take() // table-name
	cols = parseColumnList(tokens)
	if len(cols) > 1 {
		return nil, fmt.Errorf("multiple columns are not currently supported in a foreign key clause: %v", cols)
	}
	// cols[0] // TO COLUMN
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

func parseDatetimeDefault(tokens *Tokens) {
	value := ""
	paren := 0
	for {
		t := tokens.Take()
		value += t
		if t == "(" {
			paren += 1
		} else if t == ")" {
			paren -= 1
			if paren == 0 {
				break
			}
		} else {
			// ignore
		}
	}
	fmt.Printf("ignoring default value: %s\n", value)
}

// table-constraint
// https://www.sqlite.org/syntax/table-constraint.html
func parseTableConstraint(tokens *Tokens) error {
	name := ""
	switch {
	case tokens.Next() == "CONSTRAINT": // Named table constraint (which is optional)
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
		parseCheckConstraint(tokens)
	case tokens.NextN(2) == "FOREIGN KEY": // table-constraint
		tokens.TakeN(2)
		// TODO: Handle foreign-key-clause
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

// create-index-stmt
// https://www.sqlite.org/syntax/create-index-stmt.html
func parseCreateIndex(tokens *Tokens) error {
	if tokens.Take() != "CREATE" {
		return fmt.Errorf("create index must begin with \"CREATE [UNIQUE] INDEX\", not \"%s\"", tokens.NextN(3))
	}
	if tokens.Next() == "UNIQUE" {
		tokens.Take()
	}
	if tokens.Take() != "INDEX" {
		return fmt.Errorf("create index must begin with \"CREATE [UNIQUE] INDEX\", not \"%s\"", tokens.NextN(3))
	}
	if tokens.NextN(3) == "IF NOT EXISTS" {
		tokens.TakeN(3)
	}
	name := tokens.Take()
	if strings.Contains(name, ".") {
		parts := strings.SplitN(name, ".", 2) // FIXME: Assumes there is only one period
		fmt.Printf("Schema Name: %s, Index Name: %s", parts[0], parts[1])
	}
	if tokens.Take() != "ON" {
		tokens.Return()
		return fmt.Errorf("create index must be followed by \"ON table-name\", not \"%s\"", tokens.NextN(2))
	}
	tableName := tokens.Take()
	cols := parseColumnList(tokens)
	if cols == nil {
		return fmt.Errorf("create index must be followed by one or more columns, not \"%s\"", tokens.NextN(3))
	}
	// TODO: Handle WHERE expr
	if tokens.Next() == ";" { // Optional(?) closing semicolon
		tokens.Take()
	}
	fmt.Printf("Index: %s ON %s (%v)", name, tableName, cols)
	return nil

}

// parseCheckConstraint: CHECK ( expr )
// See column-constraint and table-constraint
func parseCheckConstraint(tokens *Tokens) {
	tokens.Take()
	value := []string{}
	paren := 0
	for {
		t := tokens.Next()
		if t == "(" {
			tokens.Take()
			paren += 1
		} else if t == ")" {
			tokens.Take()
			paren -= 1
			if paren < 1 {
				break // End of CHECK EXPRESSION
			}
		} else {
			// Unsupported token
			value = append(value, tokens.Take())
		}
	}
	fmt.Printf("Ignoring CHECK constraints: %s", strings.Join(value, " "))
}

func parseExpression(tokens *Tokens) {
	// TODO
}
