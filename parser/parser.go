package parser

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/joshsziegler/zgo/pkg/log"
)

// Parse SQL string to a set of tables with column definitions.
func Parse(sql string) ([]*Table, error) {
	tokens := Lex(sql)
	tables := make([]*Table, 0)
	for {
		switch {
		case tokens.NextType() == Comment: // Comment between statements; discarded.
			tokens.Take()
		case tokens.Next() == "": // End of SQL
			return tables, nil
		case tokens.KeywordSeq("CREATE", "TABLE"):
			table, err := parseCreateTable(tokens)
			if err != nil {
				printContext(tokens, err)
				return nil, err
			}
			tables = append(tables, table)
		case tokens.KeywordSeq("CREATE", "INDEX"):
			fallthrough
		case tokens.KeywordSeq("CREATE", "UNIQUE", "INDEX"):
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

// parseComment consumes and returns the next token's text if it is a comment, or "" otherwise.
// The lexer scans -- line and /* block */ comments into a single Comment token, so the comment
// text is already assembled here.
//
// SQLite Docs: https://www.sqlite.org/lang_comment.html
func parseComment(tokens *Tokens) string {
	if tokens.NextType() == Comment {
		return tokens.Take()
	}
	return ""
}

// parseInlineComment consumes and returns a trailing comment on the SAME line (no newline before
// it), or "" otherwise. Used so a comment on its own line is not mistaken for the previous
// definition's trailing comment.
func parseInlineComment(tokens *Tokens) string {
	if tokens.NextType() == Comment && !tokens.NextNewlineBefore() {
		return tokens.Take()
	}
	return ""
}

// parseCreateTable
// https://www.sqlite.org/syntax/create-table-stmt.html
func parseCreateTable(tokens *Tokens) (*Table, error) {
	t := &Table{
		Strict:      false,
		Temp:        false,
		IfNotExists: false,
		Columns:     make([]Column, 0),
	}

	if !tokens.TakeKeyword("CREATE") {
		return nil, fmt.Errorf("create table must begin with 'CREATE', not %s", tokens.Next())
	}
	// Temporary
	if tokens.KeywordIs("TEMP") || tokens.KeywordIs("TEMPORARY") {
		t.Temp = true
		tokens.Take()
	}
	if !tokens.TakeKeyword("TABLE") {
		return nil, fmt.Errorf("create table must begin with 'CREATE [TEMP|TEMPORARY] TABLE', not %s", tokens.Next())
	}
	// If not exists
	if tokens.KeywordIs("IF") {
		if tokens.KeywordSeq("IF", "NOT", "EXISTS") {
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
		t.SetSQLName(removeQuotes(tokens.Take()))
	} else {
		t.SetSQLName(removeQuotes(tokens.Take()))
	}
	// TODO: Handle "AS SELECT STMT" ?
	// Opening parenthesis for column definitions
	if tokens.Next() == "(" {
		tokens.Take()
	}
	// Trailing comment on the CREATE TABLE ( line itself.
	t.Comment = parseInlineComment(tokens)

	// Column(s)
	for {
		if tokens.NextType() == Comment { // Standalone comment between definitions; discarded.
			parseComment(tokens)
			continue
		}
		switch {
		case tokens.Next() == ")": // End of Table Definition
			tokens.Take()
			if tokens.Next() == ";" { // Optional semicolon
				tokens.Take()
			}
			return t, nil
		case tokens.Next() == "": // Table wasn't closed properly or something went wrong
			return nil, fmt.Errorf("ran out of tokens unexpectedly - table was likely not closed properly")
		case tokens.Next() == ",":
			tokens.Take()
		// table-constraint: CONSTRAINT, PRIMARY KEY, UNIQUE, CHECK, or FOREIGN KEY
		case tokens.KeywordIs("CONSTRAINT"), tokens.KeywordIs("PRIMARY"), tokens.KeywordIs("UNIQUE"),
			tokens.KeywordIs("CHECK"), tokens.KeywordIs("FOREIGN"):
			parseTableConstraint(tokens, t)
		default: // column-def
			col, fk, err := parseColumn(tokens, t.Strict)
			if err != nil {
				return nil, err
			}
			t.Columns = append(t.Columns, col)
			if fk != nil {
				if err := t.AddForeignKey(fk); err != nil {
					return nil, err
				}
			}
		}
	}
}

// parseColumn parses a single column definition. If the column declares an inline (single-column)
// foreign key via REFERENCES, it is returned as the second value (nil otherwise) so the caller can
// attach it to the table; foreign keys are stored on the Table, not the Column.
func parseColumn(tokens *Tokens, strict bool) (Column, *ForeignKey, error) {
	c := Column{
		PrimaryKey: false,
		Nullable:   true,
		Unique:     false,
	}
	var fk *ForeignKey

	// Name
	c.SetSQLName(removeQuotes(tokens.Take()))
	// Data Type
	err := parseColumnDataType(tokens, &c, strict)
	if err != nil {
		return c, nil, err
	}
	// Constraints
	// SQlite Docs: https://www.sqlite.org/syntax/column-constraint.html
	for {
		token := tokens.Next()
		if token == "," { // End of Column Definition (check for optional comment)
			tokens.Take()
			c.Comment = parseInlineComment(tokens)
			break
		} else if token == ")" { // End of table definition. DO NOT CONSUME TOKEN
			break
		} else if tokens.NextType() == Comment {
			c.Comment = parseComment(tokens)
		} else if tokens.KeywordSeq("NOT", "NULL") {
			// TODO: Handle [conflict-clause]
			c.Nullable = false
			tokens.TakeN(2)
		} else if tokens.KeywordIs("NOT") {
			return c, nil, fmt.Errorf("column constraint must be 'NOT NULL', not %s", tokens.NextN(2))
		} else if tokens.KeywordSeq("PRIMARY", "KEY") {
			// TODO: Handle [ASC|DESC] and [conflict-clause]
			c.PrimaryKey = true
			tokens.TakeN(2)
		} else if tokens.KeywordIs("PRIMARY") {
			return c, nil, fmt.Errorf("column constraint must be 'PRIMARY KEY', not %s", tokens.NextN(2))
		} else if tokens.KeywordIs("AUTOINCREMENT") {
			if c.PrimaryKey {
				tokens.Take() // Consume ONE token
			} else {
				return c, nil, errors.New("column constraint 'AUTOINCREMENT' must follow 'PRIMARY KEY'")
			}
		} else if tokens.KeywordIs("UNIQUE") {
			// TODO: Handle [conflict-clause]
			c.Unique = true
			tokens.Take() // Consume ONE token
		} else if tokens.KeywordIs("REFERENCES") { // Foreign Key
			fk, err = parseForeignKey(tokens, &c)
			if err != nil {
				return c, nil, err
			}
		} else if tokens.KeywordIs("CHECK") { // CHECK column constraint
			parseCheckConstraint(tokens)
		} else if tokens.KeywordIs("DEFAULT") {
			tokens.Take()
			switch c.Type {
			case INT:
				token := tokens.Take()
				if !strings.EqualFold(token, "NULL") { // TODO: How to mark explicitly as DEFAULTS TO NULL?
					val, err := strconv.ParseInt(token, 10, 64) // Should not have quotes
					if err != nil {
						return c, nil, fmt.Errorf("default value for INT/INTEGER must be a valid base 10 integer or NULL, not %s", token)
					}
					c.DefaultInt = sql.NullInt64{Valid: true, Int64: val}
				}
			case TEXT:
				token := tokens.Take()
				if !strings.EqualFold(token, "NULL") {
					c.DefaultString = sql.NullString{Valid: true, String: removeQuotes(token)}
				}
			case BOOL:
				token := tokens.Take()
				if token == "(" && tokens.Peek(1) == ")" { // FIXME: Better handle expressions
					token = tokens.Take() // This should be TRUE or FALSE
					tokens.Take()         // Take closing parenthesis
				}
				switch strings.ToUpper(token) {
				case "NULL":
					// TODO: How to mark explicitly as DEFAULTS TO NULL?
				case "TRUE", "1":
					c.DefaultBool = sql.NullBool{Valid: true, Bool: true}
				case "FALSE", "0":
					c.DefaultBool = sql.NullBool{Valid: true, Bool: false}
				default:
					return c, nil, fmt.Errorf("default value for BOOL/BOOLEAN must be TRUE/true/1 or FALSE/false/0, not %s", token)
				}
			case DATETIME:
				parseDatetimeDefault(tokens)
			default:
				return c, nil, fmt.Errorf("default values for %s type are not supported", c.Type)
			}
		} else {
			return c, nil, fmt.Errorf("unrecognized column constraint for column \"%s\" starting with: \"%s\"", c.SQLName(), tokens.NextN(5))
		}
	}

	return c, fk, nil
}

// Column Type
// - strict: Only support SQLite's STRICT data types (i.e. INT, INTEGER, REAL, TEXT, BLOB, ANY).
//
// SQLite Docs: https://www.sqlite.org/datatype3.html
// mattn/go-sqlit3 Docs: https://pkg.go.dev/github.com/mattn/go-sqlite3#hdr-Supported_Types
func parseColumnDataType(tokens *Tokens, c *Column, strict bool) error {
	raw := tokens.Take()
	s := strings.ToUpper(raw) // SQLite type names are case-insensitive (e.g. integer == INTEGER).
	switch s {
	case "INT", "INTEGER":
		c.Type = INT
		return nil
	case "FLOAT":
		fallthrough
	case "REAL":
		c.Type = FLOAT
		return nil
	case "TEXT":
		c.Type = TEXT
		return nil
	case "BLOB":
		fallthrough
	case "ANY":
		c.Type = BLOB
		return nil
	case "BOOL", "BOOLEAN":
		if strict {
			return fmt.Errorf("\"%s\" is not a valid SQLite column type when \"strict\" is enabled", raw)
		}
		c.Type = BOOL
		return nil
	case "DATETIME", "TIMESTAMP":
		if strict {
			return fmt.Errorf("\"%s\" is not a valid SQLite column type when \"strict\" is enabled", raw)
		}
		c.Type = DATETIME
		return nil
	case ",", ")": // Ugly hack
		// Data type not specified. DO NOT CONSUME A TOKEN.
		c.Type = BLOB
		tokens.Return()
		return nil
	default:
		if strict {
			return fmt.Errorf("\"%s\" is not a valid SQLite column type when \"strict\" is enabled", raw)
		}
		c.Type = BLOB
		return nil
	}
}

// parseForeignKey assumes the Next() token is "REFERENCES" and parses an inline foreign key for
// column c, returning it. Inline foreign keys always reference a single column.
func parseForeignKey(tokens *Tokens, c *Column) (*ForeignKey, error) {
	if tokens.Peek(1) == "" {
		return nil, fmt.Errorf("column foreign key constraint 'REFERENCES' must be followed by a table name, not %s", tokens.NextN(2))
	}
	tokens.Take()
	// TODO: Handle [conflict-clause]
	fk := &ForeignKey{Table: tokens.Take(), LocalColumns: []string{c.SQLName()}}
	if tokens.Next() == "(" && tokens.Peek(2) == ")" { // If the referenced column is specified
		fk.Columns = []string{tokens.Peek(1)}
		tokens.TakeN(3)
	} else {
		fk.Columns = []string{c.SQLName()} // Not specified, so it should be the same name as THIS column name.
	}
	if err := parseFkConstraints(tokens, fk); err != nil {
		return nil, err
	}
	return fk, nil
}

// foreign-key-clause
// Note this is different from an inline FK definition, and can reference multiple columns.
// https://www.sqlite.org/syntax/foreign-key-clause.html
//
// The returned ForeignKey has its LocalColumns (FROM) and Columns (TO) populated, paired
// positionally. The caller is responsible for validating and attaching it to the table.
func parseForeignKeyClause(tokens *Tokens) (*ForeignKey, error) {
	fk := &ForeignKey{}
	fk.LocalColumns = parseColumnList(tokens) // FROM-COLUMN(s)
	if len(fk.LocalColumns) == 0 {
		return nil, fmt.Errorf("foreign key clause must specify at least one column")
	}
	if !tokens.TakeKeyword("REFERENCES") {
		return nil, fmt.Errorf("foreign key clause must contain 'REFERENCES table-name', not %s", tokens.NextN(2))
	}
	fk.Table = tokens.Take() // table-name
	refCols := parseColumnList(tokens)
	if len(refCols) == 0 {
		// Referenced columns omitted: SQLite references the foreign table's primary key. We
		// approximate by reusing the local column names (as inline FKs do).
		fk.Columns = append([]string{}, fk.LocalColumns...)
	} else {
		fk.Columns = refCols // TO-COLUMN(s)
	}
	if len(fk.LocalColumns) != len(fk.Columns) {
		return nil, fmt.Errorf("foreign key clause has %d local column(s) but %d referenced column(s): %v -> %v",
			len(fk.LocalColumns), len(fk.Columns), fk.LocalColumns, fk.Columns)
	}
	if err := parseFkConstraints(tokens, fk); err != nil {
		return nil, err
	}
	return fk, nil
}

// parseFkConstraints consumes the clauses that may follow the referenced table/column list in a
// foreign-key definition: any number of ON DELETE/UPDATE and MATCH clauses (in any order),
// followed by an optional [NOT] DEFERRABLE clause. MATCH and DEFERRABLE are accepted but ignored,
// matching SQLite's behavior. Shared by inline (parseForeignKey) and table-level
// (parseForeignKeyClause) foreign keys.
// https://www.sqlite.org/syntax/foreign-key-clause.html
func parseFkConstraints(tokens *Tokens, fk *ForeignKey) error {
	for {
		switch {
		case tokens.KeywordIs("ON"):
			if err := parseFkAction(tokens, fk); err != nil {
				return err
			}
		case tokens.KeywordIs("MATCH"):
			tokens.TakeN(2) // MATCH name (parsed but ignored, like SQLite)
		case tokens.KeywordIs("DEFERRABLE"), tokens.KeywordSeq("NOT", "DEFERRABLE"):
			parseDeferrable(tokens)
		default:
			return nil
		}
	}
}

// parseDeferrable consumes a [NOT] DEFERRABLE [INITIALLY DEFERRED | INITIALLY IMMEDIATE] clause.
// It is parsed but ignored, matching SQLite's behavior.
func parseDeferrable(tokens *Tokens) {
	tokens.TakeKeyword("NOT")
	tokens.Take() // DEFERRABLE
	if tokens.KeywordSeq("INITIALLY", "DEFERRED") || tokens.KeywordSeq("INITIALLY", "IMMEDIATE") {
		tokens.TakeN(2)
	}
}

func parseFkAction(tokens *Tokens, fk *ForeignKey) error {
	switch {
	case tokens.KeywordSeq("ON", "DELETE", "CASCADE"):
		tokens.TakeN(3)
		fk.OnDelete = Cascade
	case tokens.KeywordSeq("ON", "UPDATE", "CASCADE"):
		tokens.TakeN(3)
		fk.OnUpdate = Cascade
	case tokens.KeywordSeq("ON", "DELETE", "RESTRICT"):
		tokens.TakeN(3)
		fk.OnDelete = Restrict
	case tokens.KeywordSeq("ON", "UPDATE", "RESTRICT"):
		tokens.TakeN(3)
		fk.OnUpdate = Restrict
	case tokens.KeywordSeq("ON", "DELETE", "NO", "ACTION"):
		tokens.TakeN(4)
		fk.OnDelete = NoAction
	case tokens.KeywordSeq("ON", "UPDATE", "NO", "ACTION"):
		tokens.TakeN(4)
		fk.OnUpdate = NoAction
	case tokens.KeywordSeq("ON", "DELETE", "SET", "NULL"):
		tokens.TakeN(4)
		fk.OnDelete = SetNull
	case tokens.KeywordSeq("ON", "UPDATE", "SET", "NULL"):
		tokens.TakeN(4)
		fk.OnUpdate = SetNull
	case tokens.KeywordSeq("ON", "DELETE", "SET", "DEFAULT"):
		tokens.TakeN(4)
		fk.OnDelete = SetDefault
	case tokens.KeywordSeq("ON", "UPDATE", "SET", "DEFAULT"):
		tokens.TakeN(4)
		fk.OnUpdate = SetDefault
	default:
		return fmt.Errorf("column foreign key constraint \"%s\" not supported", tokens.NextN(4))
	}
	return nil
}

func parseDatetimeDefault(tokens *Tokens) {
	t := tokens.Take()
	if strings.EqualFold(t, "NULL") {
		log.Debug("[IGNORED] DateTime Default: NULL")
		return
	}
	value := []string{}
	paren := 0
	for {
		if t == "(" {
			paren += 1
		} else if t == ")" {
			paren -= 1
			if paren == 0 {
				break
			}
		} else {
			// ignore
			value = append(value, t)
		}
		t = tokens.Take()
	}
	log.Debugf("[IGNORED] DateTime Default: %s\n", strings.Join(value, " "))
}

// table-constraint
// https://www.sqlite.org/syntax/table-constraint.html
func parseTableConstraint(tokens *Tokens, table *Table) error {
	name := ""
	switch {
	case tokens.KeywordIs("CONSTRAINT"): // Named table constraint (which is optional)
		name = tokens.Take()
		log.Debugf("[IGNORED] Table Constraint Constraint: %s", tokens.Take())
	case tokens.KeywordSeq("PRIMARY", "KEY"): // table-constraint
		tokens.TakeN(2) // PRIMARY KEY
		table.SetPrimaryKeys(parseIndexedColumn(tokens))
		// TODO: Handle conflict clause
	case tokens.KeywordIs("UNIQUE"): // table-constraint
		tokens.Take()
		cols := parseIndexedColumn(tokens)
		// TODO: Handle conflict clause
		log.Debugf("[IGNORED] Table Constraint Unique '%s' Columns %v", name, cols)
	case tokens.KeywordIs("CHECK"): // table-constraint
		parseCheckConstraint(tokens)
	case tokens.KeywordSeq("FOREIGN", "KEY"): // table-constraint
		tokens.TakeN(2)
		fk, err := parseForeignKeyClause(tokens)
		if err != nil {
			return err
		}
		if err := table.AddForeignKey(fk); err != nil {
			return err
		}
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
			cols = append(cols, removeQuotes(tokens.Take()))
		}
		t = tokens.Next()
	}
	return cols
}

// create-index-stmt
// https://www.sqlite.org/syntax/create-index-stmt.html
//
// TODO: Parse `CREATE [UNIQUE] INDEX ...` instead of ignoring it.
func parseCreateIndex(tokens *Tokens) error {
	// Find the closing semicolon and ignore this index (i.e. handle multiline)
	value := []string{}
	token := tokens.Take()
	for {
		token = tokens.Take()
		if token == ";" {
			break // Found end of index
		} else if token == "" {
			// Provide up to five tokens of context to avoid printing a huge amount of SQL
			partialValue := ""
			if len(value) > 5 {
				partialValue = strings.Join(value[0:5], " ")
			} else {
				partialValue = strings.Join(value, " ")
			}
			return fmt.Errorf("expected closing semi-colon while parsing CREATE INDEX, but found end of SQL: %s", partialValue)
		} else {
			value = append(value, token)
		}
	}
	log.Debugf("[IGNORED] CREATE INDEX: %s", strings.Join(value, " "))
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
	log.Debugf("[IGNORED] CHECK constraint: %s", strings.Join(value, " "))
}
