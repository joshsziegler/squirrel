package main

import (
	"fmt"
	"os"

	"github.com/carlmjohnson/versioninfo"

	"github.com/joshsziegler/squirrel/parser"
	"github.com/joshsziegler/squirrel/templates"
)

// readFile from disk and return its content as a string.
func readFile(path string) (string, error) {
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(fileBytes), nil
}

// ParseFile (as string) to a set of structs.
func ParseFile(path string) ([]*parser.Table, error) {
	file, err := readFile(path)
	if err != nil {
		return nil, err
	}
	tables, err := parser.Parse(file)
	if err != nil {
		return tables, err
	}
	return tables, nil
}

// GenerateGoFromSQL by reading the schema file from disk, parsing it, and then writing it to goPath.
// Any table in ignoreTables will be parsed, but not included in the generated Go.
func GenerateGoFromSQL(schemaPath, goPath, pkgName string, ignoreTables []string) error {
	tables, err := ParseFile(schemaPath)
	if err != nil {
		return err
	}

	// Write Go-SQL code to disk
	f, err := os.OpenFile(goPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	templates.Write(f, pkgName, tables, ignoreTables)
	return nil
}

func main() {
	if len(os.Args) < 4 {
		fmt.Printf("Usage: %s schema dest pkg [ignored_tables]\n", os.Args[0])
		fmt.Println("")
		fmt.Println("- schema: Path to the SQL schema you wish to parse (e.g. schema.sql)")
		fmt.Println("- dest: Path to write the resulting Go-to-SQL layer to (e.g. db.go)")
		fmt.Println("- pkg: Package name to use in the resulting Go code (e.g. db)")
		fmt.Println("- ignored_tables: Optional list of tables to ignore with spaces between each table (e.g. goose users).")
		fmt.Println("")
		fmt.Printf("Version: %s\n", versioninfo.Version)
		fmt.Printf("Revision: %s\n", versioninfo.Revision)
		if versioninfo.DirtyBuild { // Don't print this for tagged/released versions
			fmt.Printf("Dirty: %v\n", versioninfo.DirtyBuild)
		}
		fmt.Printf("Built On: %s\n", versioninfo.LastCommit)
		return
	}
	schema := os.Args[1]
	output := os.Args[2]
	pkgName := os.Args[3]
	ignoredTables := []string{}
	for i := 4; i < len(os.Args); i++ {
		ignoredTables = append(ignoredTables, os.Args[i])
	}
	err := GenerateGoFromSQL(schema, output, pkgName, ignoredTables)
	if err != nil {
		fmt.Println(err)
		os.Exit(1) // Return an error code so the caller knows we failed.
	}
}
