package parser

import "fmt"

// keywords are defined by SQLite: https://www.sqlite.org/lang_keywords.html
// Last updated 2024.010.01
var keywords = []string{
	"ABORT",
	"ACTION",
	"ADD",
	"AFTER",
	"ALL",
	"ALTER",
	"ALWAYS",
	"ANALYZE",
	"AND",
	"AS",
	"ASC",
	"ATTACH",
	"AUTOINCREMENT",
	"BEFORE",
	"BEGIN",
	"BETWEEN",
	"BY",
	"CASCADE",
	"CASE",
	"CAST",
	"CHECK",
	"COLLATE",
	"COLUMN",
	"COMMIT",
	"CONFLICT",
	"CONSTRAINT",
	"CREATE",
	"CROSS",
	"CURRENT",
	"CURRENT_DATE",
	"CURRENT_TIME",
	"CURRENT_TIMESTAMP",
	"DATABASE",
	"DEFAULT",
	"DEFERRABLE",
	"DEFERRED",
	"DELETE",
	"DESC",
	"DETACH",
	"DISTINCT",
	"DO",
	"DROP",
	"EACH",
	"ELSE",
	"END",
	"ESCAPE",
	"EXCEPT",
	"EXCLUDE",
	"EXCLUSIVE",
	"EXISTS",
	"EXPLAIN",
	"FAIL",
	"FILTER",
	"FIRST",
	"FOLLOWING",
	"FOR",
	"FOREIGN",
	"FROM",
	"FULL",
	"GENERATED",
	"GLOB",
	"GROUP",
	"GROUPS",
	"HAVING",
	"IF",
	"IGNORE",
	"IMMEDIATE",
	"IN",
	"INDEX",
	"INDEXED",
	"INITIALLY",
	"INNER",
	"INSERT",
	"INSTEAD",
	"INTERSECT",
	"INTO",
	"IS",
	"ISNULL",
	"JOIN",
	"KEY",
	"LAST",
	"LEFT",
	"LIKE",
	"LIMIT",
	"MATCH",
	"MATERIALIZED",
	"NATURAL",
	"NO",
	"NOT",
	"NOTHING",
	"NOTNULL",
	"NULL",
	"NULLS",
	"OF",
	"OFFSET",
	"ON",
	"OR",
	"ORDER",
	"OTHERS",
	"OUTER",
	"OVER",
	"PARTITION",
	"PLAN",
	"PRAGMA",
	"PRECEDING",
	"PRIMARY",
	"QUERY",
	"RAISE",
	"RANGE",
	"RECURSIVE",
	"REFERENCES",
	"REGEXP",
	"REINDEX",
	"RELEASE",
	"RENAME",
	"REPLACE",
	"RESTRICT",
	"RETURNING",
	"RIGHT",
	"ROLLBACK",
	"ROW",
	"ROWS",
	"SAVEPOINT",
	"SELECT",
	"SET",
	"TABLE",
	"TEMP",
	"TEMPORARY",
	"THEN",
	"TIES",
	"TO",
	"TRANSACTION",
	"TRIGGER",
	"UNBOUNDED",
	"UNION",
	"UNIQUE",
	"UPDATE",
	"USING",
	"VACUUM",
	"VALUES",
	"VIEW",
	"VIRTUAL",
	"WHEN",
	"WHERE",
	"WINDOW",
	"WITH",
	"WITHOUT",
}

// parseName returns the next token if it is a valid name in SQLite, or an error if it is not.
// The rules for what is allowed are not made clear on the SQLite website and documentaion, but
// some information is provided on keywords: https://www.sqlite.org/lang_keywords.html
//
// TODO: This does not test for alternative quoting methods such as [name] and `name` because these
// are included in SQLite for compatibility only.
func parseName(tokens *Tokens) (string, error) {
	token := tokens.Next()
	l := len(token)
	switch string(token[0]) {
	case `'`:
		if string(token[l-1]) == `'` {
			tokens.Take()
			token = token[1 : l-1]
			return token, nil // Quoted so ignore the keywords list
		} else {
			return "", fmt.Errorf("quoted name does not have a closing quote: %s", token)
		}
	case `"`:
		if string(token[l-1]) == `"` {
			tokens.Take()
			token = token[1 : l-1]
			return token, nil // Quoted so ignore the keywords list
		} else {
			return "", fmt.Errorf("quoted name does not have a closing quote: %s", token)
		}
	}
	for _, keyword := range keywords {
		if token == keyword {
			return "", fmt.Errorf("name may not be a SQLite keyword: %s", token)
		}
	}
	tokens.Take()
	return token, nil
}
