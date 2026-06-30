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

// takeSignedNumber consumes an optional leading '+'/'-' sign followed by the next token, returning
// them joined (e.g. "-100", "+2.5e3"). SQLite's DEFAULT clause allows a signed-number literal, and
// the lexer emits the sign as a separate Operator token, so this reassembles it. If the next token
// is not a sign, the single next token is returned unchanged (so callers can still detect NULL).
func takeSignedNumber(tokens *Tokens) string {
	sign := ""
	if tokens.Next() == "+" || tokens.Next() == "-" {
		sign = tokens.Take()
	}
	return sign + tokens.Take()
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
			pc, err := parseColumn(tokens, t.Strict)
			if err != nil {
				return nil, err
			}
			t.Columns = append(t.Columns, pc.Column)
			if pc.ForeignKey != nil {
				if err := t.AddForeignKey(pc.ForeignKey); err != nil {
					return nil, err
				}
			}
			if pc.Unique != nil {
				if err := t.AddUniqueConstraint(pc.Unique.Name, pc.Unique.Columns); err != nil {
					return nil, err
				}
			}
			t.CheckConstraints = append(t.CheckConstraints, pc.Checks...)
			if pc.PrimaryKeyName != "" {
				t.PrimaryKeyName = pc.PrimaryKeyName
			}
		}
	}
}

// parsedColumn bundles a parsed column with the constraints that are stored at the table level
// rather than on the Column itself, so parseColumn's caller can attach them to the table.
type parsedColumn struct {
	Column         Column
	ForeignKey     *ForeignKey       // inline REFERENCES, or nil
	Unique         *UniqueConstraint // inline UNIQUE, or nil
	Checks         []CheckConstraint // inline CHECK constraint(s)
	PrimaryKeyName string            // name of an inline named PRIMARY KEY, or ""
}

// parseColumn parses a single column definition, returning the column together with any inline
// constraints that belong on the table (foreign key, unique, check, primary-key name). A
// "CONSTRAINT <name>" prefix may precede any column constraint and names the constraint that
// follows it.
// SQLite Docs: https://www.sqlite.org/syntax/column-constraint.html
func parseColumn(tokens *Tokens, strict bool) (parsedColumn, error) {
	pc := parsedColumn{Column: Column{PrimaryKey: false, Nullable: true}}
	c := &pc.Column
	// constraintName holds the name from a pending "CONSTRAINT <name>" prefix; it applies to the
	// next constraint and is cleared once consumed.
	constraintName := ""

	// Name
	c.SetSQLName(removeQuotes(tokens.Take()))
	// Data Type
	if err := parseColumnDataType(tokens, c, strict); err != nil {
		return pc, err
	}
	// Constraints
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
		} else if tokens.KeywordIs("CONSTRAINT") {
			tokens.Take()                  // CONSTRAINT
			constraintName = tokens.Take() // names the constraint that follows
		} else if tokens.KeywordSeq("NOT", "NULL") {
			// TODO: Handle [conflict-clause]
			c.Nullable = false
			tokens.TakeN(2)
			constraintName = "" // no model slot for a NOT NULL constraint name
		} else if tokens.KeywordIs("NOT") {
			return pc, fmt.Errorf("column constraint must be 'NOT NULL', not %s", tokens.NextN(2))
		} else if tokens.KeywordSeq("PRIMARY", "KEY") {
			// TODO: Handle [ASC|DESC] and [conflict-clause]
			c.PrimaryKey = true
			pc.PrimaryKeyName = constraintName
			constraintName = ""
			tokens.TakeN(2)
		} else if tokens.KeywordIs("PRIMARY") {
			return pc, fmt.Errorf("column constraint must be 'PRIMARY KEY', not %s", tokens.NextN(2))
		} else if tokens.KeywordIs("AUTOINCREMENT") {
			if c.PrimaryKey {
				tokens.Take() // Consume ONE token
			} else {
				return pc, errors.New("column constraint 'AUTOINCREMENT' must follow 'PRIMARY KEY'")
			}
		} else if tokens.KeywordIs("UNIQUE") {
			// TODO: Handle [conflict-clause]
			// Inline UNIQUE is recorded as a single-column table-level constraint.
			pc.Unique = &UniqueConstraint{Name: constraintName, Columns: []string{c.SQLName()}}
			constraintName = ""
			tokens.Take() // Consume ONE token
		} else if tokens.KeywordIs("REFERENCES") { // Foreign Key
			fk, err := parseForeignKey(tokens, c)
			if err != nil {
				return pc, err
			}
			fk.Name = constraintName
			constraintName = ""
			pc.ForeignKey = fk
		} else if tokens.KeywordIs("CHECK") { // CHECK column constraint
			pc.Checks = append(pc.Checks, CheckConstraint{Name: constraintName, Expr: parseCheckConstraint(tokens)})
			constraintName = ""
		} else if tokens.KeywordIs("DEFAULT") {
			tokens.Take()
			constraintName = "" // no model slot for a DEFAULT constraint name
			switch c.Type {
			case INT:
				token := takeSignedNumber(tokens)
				if !strings.EqualFold(token, "NULL") { // TODO: How to mark explicitly as DEFAULTS TO NULL?
					val, err := strconv.ParseInt(token, 10, 64) // Should not have quotes
					if err != nil {
						return pc, fmt.Errorf("default value for INT/INTEGER must be a valid base 10 integer or NULL, not %s", token)
					}
					c.DefaultInt = sql.NullInt64{Valid: true, Int64: val}
				}
			case FLOAT:
				token := takeSignedNumber(tokens)
				if !strings.EqualFold(token, "NULL") {
					val, err := strconv.ParseFloat(token, 64)
					if err != nil {
						return pc, fmt.Errorf("default value for REAL/FLOAT must be a valid number or NULL, not %s", token)
					}
					c.DefaultFloat = sql.NullFloat64{Valid: true, Float64: val}
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
					return pc, fmt.Errorf("default value for BOOL/BOOLEAN must be TRUE/true/1 or FALSE/false/0, not %s", token)
				}
			case DATETIME:
				parseDatetimeDefault(tokens)
			default:
				return pc, fmt.Errorf("default values for %s type are not supported", c.Type)
			}
		} else {
			return pc, fmt.Errorf("unrecognized column constraint for column \"%s\" starting with: \"%s\"", c.SQLName(), tokens.NextN(5))
		}
	}

	return pc, nil
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
	// Optional CONSTRAINT <name> prefix that may precede any table-constraint.
	name := ""
	if tokens.KeywordIs("CONSTRAINT") {
		tokens.Take()        // CONSTRAINT
		name = tokens.Take() // constraint name
	}
	switch {
	case tokens.KeywordSeq("PRIMARY", "KEY"): // table-constraint
		tokens.TakeN(2) // PRIMARY KEY
		table.PrimaryKeyName = name
		table.SetPrimaryKeys(parseIndexedColumn(tokens))
		// TODO: Handle conflict clause
	case tokens.KeywordIs("UNIQUE"): // table-constraint
		tokens.Take()
		// TODO: Handle conflict clause
		if err := table.AddUniqueConstraint(name, parseIndexedColumn(tokens)); err != nil {
			return err
		}
	case tokens.KeywordIs("CHECK"): // table-constraint
		table.CheckConstraints = append(table.CheckConstraints, CheckConstraint{Name: name, Expr: parseCheckConstraint(tokens)})
	case tokens.KeywordSeq("FOREIGN", "KEY"): // table-constraint
		tokens.TakeN(2)
		fk, err := parseForeignKeyClause(tokens)
		if err != nil {
			return err
		}
		fk.Name = name
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
// parseCheckConstraint parses a CHECK ( expr ) constraint, returning the expression as a
// space-normalized token string (best-effort; nested parentheses are preserved but the outer pair
// is not). The leading CHECK keyword is consumed.
func parseCheckConstraint(tokens *Tokens) string {
	tokens.Take() // CHECK
	if tokens.Next() != "(" {
		return "" // malformed; nothing to capture
	}
	tokens.Take() // opening parenthesis
	value := []string{}
	paren := 1
	for paren > 0 {
		t := tokens.Take()
		switch t {
		case "":
			return strings.Join(value, " ") // ran out of tokens
		case "(":
			paren++
			value = append(value, t)
		case ")":
			paren--
			if paren > 0 {
				value = append(value, t)
			}
		default:
			value = append(value, t)
		}
	}
	return strings.Join(value, " ")
}
