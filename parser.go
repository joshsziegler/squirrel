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

func eatNewLines(tokens []string) []string {
	for {
		if len(tokens) > 0 && tokens[0] == "\n" {
			tokens = tokens[1:] // Eat whitespace
			continue
		}
		return tokens
	}
}

// Parse SQL string to a set of tables with column definitions.
func Parse(sql string) ([]*Table, error) {
	tokens := Lex(sql)
	tables := make([]*Table, 0)
	for {
		tokens = eatNewLines(tokens)
		if len(tokens) < 1 {
			break
		}
		var table *Table
		var err error
		tokens, table, err = parseCreateTable(tokens)
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
func parseComment(tokens []string) []string {
	if tokens[0] == "--" {
		for i := 1; i < len(tokens); i++ {
			if tokens[i] == "\n" {
				tokens = consume(i+1, tokens) // Consume entire comment to end of line
				break
			}
			// Consume token as part of comment
		}
	}
	return tokens
}

// parseCreateTable
// https://www.sqlite.org/syntax/create-table-stmt.html
func parseCreateTable(tokens []string) ([]string, *Table, error) {
	t := &Table{
		Temp:        false,
		IfNotExists: false,
		Columns:     make([]Column, 0),
	}

	if tokens[0] != "CREATE" {
		return tokens, nil, fmt.Errorf("create table must begin with 'CREATE', not %s", tokens[0])
	}
	tokens = tokens[1:]

	// Temporary
	if tokens[0] == "TEMP" || tokens[0] == "TEMPORARY" {
		t.Temp = true
		tokens = tokens[1:]
	}

	if tokens[0] != "TABLE" {
		return tokens, nil, fmt.Errorf("create table must begin with 'CREATE [TEMP|TEMPORARY] TABLE', not %s", tokens[0])
	}
	tokens = tokens[1:]

	// If not exists
	if tokens[0] == "IF" {
		if tokens[1] == "NOT" && tokens[2] == "EXISTS" {
			tokens = tokens[3:] // Consume THREE tokens (i.e. IF NOT EXISTS)
			t.IfNotExists = true
		} else {
			return tokens, nil, fmt.Errorf("create table must use 'IF NOT EXISTS' when 'IF' is present, not %s", tokens[:2])
		}
	}

	// Schema and Table Name
	if tokens[1] == "." {
		t.SchemaName = removeQuotes(tokens[0])
		t.Name = removeQuotes(tokens[2])
		tokens = tokens[3:] // Consume THREE tokens (i.e. IF NOT EXISTS)
	} else {
		t.Name = removeQuotes(tokens[0])
		tokens = tokens[1:]
	}

	// TODO: Handle "AS SELECT STMT" ?

	// Opening parenthesis for column definitions
	if tokens[0] == "(" {
		tokens = tokens[1:]
	}

	// Comments
	tokens = parseComment(tokens)

	// Column(s)
	for {
		tokens = eatNewLines(tokens)
		var col Column
		var err error
		tokens, col, err = parseColumn(tokens)
		if err != nil {
			return tokens, nil, err
		}
		t.Columns = append(t.Columns, col)
		token := tokens[0]
		if token == "," { // End of Column Definition (with another to follow)
			tokens = tokens[1:]
			continue
		} else if token == ")" { // End of Table Definition
			if tokens[1] == ";" { // Optional semicolon
				tokens = tokens[2:]
			} else {
				tokens = tokens[1:]
			}
			break
		} else if token == "--" {
			tokens = parseComment(tokens)
		} else {
			break
		}
	}

	return tokens, t, nil
}

// parseColumn
func parseColumn(tokens []string) ([]string, Column, error) {
	c := Column{
		PrimaryKey: false,
		Nullable:   true,
		Unique:     false,
	}

	// Column Name
	c.Name = removeQuotes(tokens[0])
	tokens = tokens[1:]

	// Column Type (TODO: Support more than these STRICT MODE types.)
	switch tokens[0] {
	case "INT":
		fallthrough
	case "INTEGER":
		c.Type = "int64"
		tokens = tokens[1:]
	case "REAL":
		c.Type = "float"
		tokens = tokens[1:]
	case "TEXT":
		c.Type = "string"
		tokens = tokens[1:]
	case "BLOB":
		fallthrough
	case "ANY":
		c.Type = "byte"
		tokens = tokens[1:]
	default:
		c.Type = tokens[0]
		tokens = tokens[1:]
		// return tokens, c, fmt.Errorf("column must use a valid type, not %s", tokens[0])
	}

	// Column Constraints
	for {
		token := tokens[0]
		if token == "," {
			break // End of column definition. DO NOT CONSUME TOKEN.
		} else if token == ")" {
			break // End of table definition. DO NOT CONSUME TOKEN
		} else if token == "\n" {
			tokens = tokens[1:] // Eat newline
		} else if token == "--" {
			tokens = parseComment(tokens) // Eat comment
		} else if token == "NOT" {
			if tokens[1] != "NULL" {
				return tokens, c, fmt.Errorf("column constraint must be 'NOT NULL', not %s", tokens[:1])
			}
			// TODO: Handle [conflict-clause]
			c.Nullable = false
			tokens = tokens[2:] // Consume TWO tokens (i.e. NOT NULL)
		} else if token == "PRIMARY" {
			if tokens[1] != "KEY" {
				return tokens, c, fmt.Errorf("column constraint must be 'PRIMARY KEY', not %s", tokens[:1])
			}
			// TODO: Handle [ASC|DESC] and [conflict-clause]
			c.PrimaryKey = true
			tokens = tokens[2:] // Consume TWO tokens (i.e. PRIMARY KEY)
		} else if token == "AUTOINCREMENT" {
			if c.PrimaryKey {
				tokens = tokens[1:] // Consume ONE token
			} else {
				return tokens, c, errors.New("column constraint 'AUTOINCREMENT' must follow 'PRIMARY KEY'")
			}
		} else if token == "UNIQUE" {
			// TODO: Handle [conflict-clause]
			c.Unique = true
			tokens = tokens[1:] // Consume ONE token
		} else {
			return tokens, c, fmt.Errorf("unrecognized column constraint starting with: %s", tokens[:5])
		}
	}
	return tokens, c, nil
}
