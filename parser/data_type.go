package parser

import "fmt"

// Datatype is the domain-type representing out SQLite-to-Go type mapping.
type Datatype string

const (
	UNKNOWN  Datatype = "Unknown"
	INT               = "int"
	BOOL              = "bool"
	FLOAT             = "float"
	TEXT              = "text"
	BLOB              = "blob"
	DATETIME          = "datetime"
)

// ToGo converts this domain type to the equivalent Go type.
// Uses Int64 for all integers and Float64 for all floats.
func (t Datatype) ToGo(nullable bool) string {
	switch t {
	case INT:
		if nullable {
			return "sql.NullInt64"
		}
		return "int64"
	case BOOL:
		if nullable {
			return "sql.NullBool"
		}
		return "bool"
	case FLOAT:
		if nullable {
			return "sql.NullFloat64"
		}
		return "float64"
	case TEXT:
		if nullable {
			return "sql.NullString"
		}
		return "string"
	case BLOB:
		return "[]byte" // TODO: Is this the right Go type for unmarshalling from SQL?
	case DATETIME:
		if nullable {
			return "sql.NullTime"
		}
		return "time.Time"
	default: // Unknown datatype
		panic("unknown datatype")
	}
}

// FromSQL to internal domain type.
//
// SQLite's STRICT data types (i.e. INT, INTEGER, REAL, TEXT, BLOB, ANY).
// SQLite Docs: https://www.sqlite.org/datatype3.html
// mattn/go-sqlit3 Docs: https://pkg.go.dev/github.com/mattn/go-sqlite3#hdr-Supported_Types
func FromSQL(s string) (Datatype, error) {
	switch s {
	case "INT", "INTEGER":
		return INT, nil
	case "BOOL", "BOOLEAN": // SQLite does not have a bool or boolean type and represents them as integers (0 and 1) internally.
		return BOOL, nil
	case "FLOAT": // TODO: This isn't a valid SQLite type.
		fallthrough
	case "REAL":
		return FLOAT, nil
	case "TEXT":
		return TEXT, nil
	case "BLOB":
		fallthrough
	case "ANY":
		return BLOB, nil
	case "DATETIME":
		fallthrough
	case "TIMESTAMP":
		return DATETIME, nil
	default:
		// TODO: IF strict, return an error. Otherwise use "ANY"
		return "", fmt.Errorf("\"%s\" is not a valid SQLite column type", s)
	}
}
