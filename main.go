package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	comment = `(--(?P<comment>.*)$)?`
	// create-table-stmt: https://www.sqlite.org/syntax/create-table-stmt.html
	createTableStmt = `CREATE\s+(?P<temp>(?:TEMP|TEMPORARY)\s+)?TABLE\s+(?P<exists>IF NOT EXISTS\s+)?[\'\"]?(?P<name>[a-zA-Z0-9]+)[\'\"]?\s+\(` + comment
	// column-constraint: https://www.sqlite.org/syntax/column-constraint.html
	columnConstraint = `(?P<constraint>CONSTRAINT\s+.*)?`
	// column-def: https://www.sqlite.org/syntax/column-def.html
	columnDef = `(\s+(?P<colName>\w+)\s+(?P<colType>\w+,?)\s+` + columnConstraint + `,?)+`
	// table-options: https://www.sqlite.org/syntax/table-options.html
	tableOptions = `\s+(?P<withoutRowID>WITHOUT ROWID)?(?P<strict>STRICT)?`

	re = regexp.MustCompile(createTableStmt + columnDef + `\s+\)`) //+ tableOptions + `\s+\)`)
)

func grep(sql string) bool {
	tables := re.FindAllString(sql, -1)
	for _, table := range tables {
		matches := re.FindStringSubmatch(table)
		tableName := matches[re.SubexpIndex("name")]
		fmt.Printf("%s\n", tableName)

		colNameIdx := re.SubexpIndex("colName")
		if colNameIdx >= 0 {
			colName := matches[colNameIdx]
			colType := matches[re.SubexpIndex("colType")]
			fmt.Printf("  - %s %s\n", colName, colType)
		}

	}
	return len(tables) > 0
}

func read_file(path string) (string, error) {
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(fileBytes), nil
}

// lexer turns a string into tokens.
func lexer(s string) []string {
	tokens := []string{}
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		token := ""
		for _, ch := range line {
			switch ch {
			case ' ', '	': // space or tab
				if token != "" {
					tokens = append(tokens, token)
					token = ""
				}
			case ',', '(', ')', ';':
				if token != "" {
					tokens = append(tokens, token)
					token = ""
				}
				tokens = append(tokens, string(ch))
			default:
				token += string(ch)
			}
		}
		if token != "" {
			tokens = append(tokens, token)
		}
		tokens = append(tokens, "\n") // Add explicit newline for line-dependent semantics, like comments
	}
	return tokens
}

func parser(tokens []string) {
	stmt := ""
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		switch token {
		case "(":
			fmt.Println(stmt)
			stmt = ""
		default:
			stmt += strings.ToLower(tokens[i])
		}
	}
}

func parse(sql string) ([]*Table, error) {
	tokens := lexer(sql)
	i := 0
	tables := make([]*Table, 0)
	for {
		if i > len(tokens) {
			break
		}
		j, table, err := parseCreateTable(tokens[i:])
		if err != nil {
			return nil, err
		}
		i += j
		tables = append(tables, table)
	}
	return tables, nil
}

func eatNewLines(i int, tokens []string) int {
	for {
		if i >= len(tokens) {
			return i
		}
		if tokens[i] == "\n" {
			i += 1 // Eat whitespace
		} else {
			break
		}
	}
	return i
}

type Table struct {
	SchemaName  string
	Name        string
	Temp        bool
	IfNotExists bool
	Columns     []Column
}

// parseCreateTable
// https://www.sqlite.org/syntax/create-table-stmt.html
func parseCreateTable(tokens []string) (int, *Table, error) {
	i := 0
	t := &Table{
		Temp:        false,
		IfNotExists: false,
		Columns:     make([]Column, 0),
	}

	i = eatNewLines(i, tokens)

	if tokens[i] != "CREATE" {
		return i, nil, fmt.Errorf("create table must begin with 'CREATE', not %s", tokens[i])
	}
	i += 1 // Eat the token

	// Temporary
	if tokens[i] == "TEMP" || tokens[i] == "TEMPORARY" {
		i += 1
		t.Temp = true
	}

	if tokens[i] != "TABLE" {
		return i, nil, fmt.Errorf("create table must begin with 'CREATE [TEMP|TEMPORARY] TABLE', not %s", tokens[0:i])
	}
	i += 1 // Eat the token

	// If not exists
	if tokens[i] == "IF" {
		if tokens[i+1] == "NOT" && tokens[i+2] == "EXISTS" {
			i += 3
			t.IfNotExists = true
		} else {
			return i, nil, fmt.Errorf("create table must use 'IF NOT EXISTS' when 'IF' is present, not %s", tokens[0:i+2])
		}
	}

	// Schema and Table Name
	if tokens[i+1] == "." {
		t.SchemaName = tokens[i]
		t.Name = tokens[i+2]
		i += 3
	} else {
		t.Name = tokens[i]
		i += 1
	}

	// TODO: Handle "AS SELECT STMT" ?

	if tokens[i] == "(" {
		i += 1
	}

	// Comments
	if tokens[i] == "--" {
		i += 1
		for {
			if tokens[i] == "\n" {
				i += 1
				break
			} else {
				i += 1 // Consume token as part of comment
			}
		}
	}

	// Column(s)
	for {
		i = eatNewLines(i, tokens)
		j, col, err := parseColumn(tokens[i:])
		i += j
		if err != nil {
			return i, nil, err
		}
		t.Columns = append(t.Columns, col)
		if tokens[i] == "," { // End of Column Definition
			i += 1
			continue
		} else if tokens[i] == ")" { // End of Table Definition
			i += 1
			if tokens[i] == ";" { // Optional semicolon
				i += 1
			}
			break
		} else {
			break
		}
	}

	return i, t, nil
}

type Column struct {
	Name       string
	Type       string
	PrimaryKey bool
	Nullable   bool
	Unique     bool
}

func parseColumn(tokens []string) (int, Column, error) {
	i := 0
	c := Column{
		PrimaryKey: false,
		Nullable:   true,
		Unique:     false,
	}

	c.Name = tokens[i]
	i += 1

	switch tokens[i] {
	case "INT":
		fallthrough
	case "INTEGER":
		c.Type = "int64"
		i += 1
	case "REAL":
		c.Type = "float"
		i += 1
	case "TEXT":
		c.Type = "string"
		i += 1
	case "BLOB":
		fallthrough
	case "ANY":
		c.Type = "byte"
		i += 1
	default:
		return i, c, fmt.Errorf("column must use a valid type, not %s", tokens[i])
	}

	for {
		token := tokens[i]
		if token == "," {
			break // End of column definition. DO NOT CONSUME TOKEN.
		} else if token == ")" {
			break // End of table definition. DO NOT CONSUME TOKEN
		} else if token == "\n" {
			i += 1 // Eat newline
		} else if token == "NOT" {
			if tokens[i+1] != "NULL" {
				return i, c, fmt.Errorf("column constraint must be 'NOT NULL', not %s", tokens[i:i+1])
			}
			// TODO: Handle [conflict-clause]
			c.Nullable = false
			i += 2
		} else if token == "PRIMARY" {
			if tokens[i+1] != "KEY" {
				return i, c, fmt.Errorf("column constraint must be 'PRIMARY KEY', not %s", tokens[i:i+1])
			}
			// TODO: Handle [ASC|DESC] and [conflict-clause]
			c.PrimaryKey = true
			i += 2
		} else if token == "AUTOINCREMENT" {
			if c.PrimaryKey {
				i += 1
			} else {
				return i, c, errors.New("column constraint 'AUTOINCREMENT' must follow 'PRIMARY KEY'")
			}
		} else if token == "UNIQUE" {
			// TODO: Handle [conflict-clause]
			c.Unique = true
			i += 1
		} else {
			return i, c, fmt.Errorf("unrecognized column constraint starting with: %s", tokens[i])
		}
	}
	return i, c, nil
}

func main() {
	file, err := read_file("test.sql")
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return
	}
	t, err := parse(file)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return
	}
	fmt.Printf("%+v\n", t)
}
