package main

import (
	"errors"
	"fmt"
	"strings"
)

// consume N tokens and return the result
func consume(n int, tokens []string) []string {
	return tokens[n:] // Consume ONE token
}

func eatNewLines(tokens *Tokens) {
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
		eatNewLines(tokens)
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
func parseComment(tokens *Tokens) {
	if tokens.Next() == "--" {
		for {
			if tokens.Next() == "\n" {
				break
			}
			tokens.Take() // Consume token as part of comment
		}
	}
	return
}

// parseCreateTable
// https://www.sqlite.org/syntax/create-table-stmt.html
func parseCreateTable(tokens *Tokens) (*Table, error) {
	t := &Table{
		Temp:        false,
		IfNotExists: false,
		Columns:     make([]Column, 0),
	}

	if tokens.Next() != "CREATE" {
		return nil, fmt.Errorf("create table must begin with 'CREATE', not %s", tokens.Next())
	}
	tokens.Take()

	// Temporary
	if tokens.Next() == "TEMP" || tokens.Next() == "TEMPORARY" {
		t.Temp = true
		tokens.Take()
	}

	if tokens.Next() != "TABLE" {
		return nil, fmt.Errorf("create table must begin with 'CREATE [TEMP|TEMPORARY] TABLE', not %s", tokens.Next())
	}
	tokens.Take()

	// If not exists
	if tokens.Next() == "IF" {
		if tokens.Nth(1) == "NOT" && tokens.Nth(2) == "EXISTS" {
			tokens.TakeN(3)
			t.IfNotExists = true
		} else {
			return nil, fmt.Errorf("create table must use 'IF NOT EXISTS' when 'IF' is present, not %v", tokens.Peek(3))
		}
	}

	// Schema and Table Name (i.e. Schema.TableName)
	if tokens.Nth(1) == "." {
		t.SchemaName = removeQuotes(tokens.Next())
		t.Name = removeQuotes(tokens.Nth(2))
		tokens.TakeN(3)
	} else {
		t.Name = removeQuotes(tokens.Next())
		tokens.Take()
	}

	// TODO: Handle "AS SELECT STMT" ?

	// Opening parenthesis for column definitions
	if tokens.Next() == "(" {
		tokens.Take()
	}

	// Comments
	parseComment(tokens)

	// Column(s)
	for {
		eatNewLines(tokens)
		var col Column
		var err error
		col, err = parseColumn(tokens)
		if err != nil {
			return nil, err
		}
		t.Columns = append(t.Columns, col)
		token := tokens.Next()
		if token == "," { // End of Column Definition (with another to follow)
			tokens.Take()
			continue
		} else if token == ")" { // End of Table Definition
			tokens.Take()
			if tokens.Next() == ";" { // Optional semicolon
				tokens.Take()
			}
			break
		} else if token == "--" {
			parseComment(tokens)
		} else {
			break
		}
	}

	return t, nil
}

// parseColumn
func parseColumn(tokens *Tokens) (Column, error) {
	c := Column{
		PrimaryKey: false,
		Nullable:   true,
		Unique:     false,
	}

	// Column Name
	c.Name = removeQuotes(tokens.Next())
	tokens.Take()

	// Column Type (TODO: Support more than these STRICT MODE types.)
	switch tokens.Next() {
	case "INT":
		fallthrough
	case "INTEGER":
		c.Type = "int64"
		tokens.Take()
	case "REAL":
		c.Type = "float"
		tokens.Take()
	case "TEXT":
		c.Type = "string"
		tokens.Take()
	case "BLOB":
		fallthrough
	case "ANY":
		c.Type = "byte"
		tokens.Take()
	default:
		c.Type = tokens.Next()
		tokens.Take()
		// return c, fmt.Errorf("column must use a valid type, not %s", tokens.Next())
	}

	// Column Constraints
	for {
		token := tokens.Next()
		if token == "," {
			break // End of column definition. DO NOT CONSUME TOKEN.
		} else if token == ")" {
			break // End of table definition. DO NOT CONSUME TOKEN
		} else if token == "\n" {
			tokens.Take() // Eat newline
		} else if token == "--" {
			parseComment(tokens) // Eat comment
		} else if token == "NOT" {
			if tokens.Nth(1) != "NULL" {
				return c, fmt.Errorf("column constraint must be 'NOT NULL', not %v", tokens.Peek(2))
			}
			// TODO: Handle [conflict-clause]
			c.Nullable = false
			tokens.TakeN(2)
		} else if token == "PRIMARY" {
			if tokens.Nth(1) != "KEY" {
				return c, fmt.Errorf("column constraint must be 'PRIMARY KEY', not %s", tokens.Peek(2))
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
		} else {
			return c, fmt.Errorf("unrecognized column constraint starting with: %v", tokens.Peek(5))
		}
	}
	return c, nil
}
