package main

import (
	"fmt"
	"os"
	"os/exec"

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
func GenerateGoFromSQL(schemaPath, goPath, pkgName string) error {
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

	templates.Header(f, pkgName)
	for _, table := range tables {
		templates.Table(f, table)
	}
	f.Close()

	// Format the result
	cmd := exec.Command("gofmt", "-l", "-w", "out")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("Error formatting output: %s; %s", err.Error(), output)
	}
	return nil
}

func main() {
	if len(os.Args) < 4 {
		fmt.Printf("Usage: %s schema dest pkg\n", os.Args[0])
		fmt.Println("- schema: Path to the SQL schema you wish to parse (e.g. schema.sql)")
		fmt.Println("- dest: Path to write the resulting Go-to-SQL layer to (e.g. db.go)")
		fmt.Println("- pkg: Package name to use in the resulting Go code (e.g. db)")
		return
	}
	schema := os.Args[1]
	output := os.Args[2]
	pkgName := os.Args[3]
	err := GenerateGoFromSQL(schema, output, pkgName)
	if err != nil {
		fmt.Println(err)
	}
}
