package main

import (
	"fmt"
	"os"
	"os/exec"
)

// readFile from disk and return its content as a string.
func readFile(path string) (string, error) {
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(fileBytes), nil
}

func ParseFile(path string) ([]*Table, error) {
	file, err := readFile(path)
	if err != nil {
		return nil, err
	}
	tables, err := Parse(file)
	if err != nil {
		return tables, err
	}
	return tables, nil
}

func GenerateGoFromSQL(schemaPath, goPath, pkgName string) error {
	tables, err := ParseFile(schemaPath)
	if err != nil {
		return err
	}

	// Write Go-SQL code to disk
	f, err := os.OpenFile(goPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	w := ShortWriter{w: f}

	Header(w, pkgName)
	for _, table := range tables {
		TableToGo(w, table)
		w.F("\n\n")
	}

	// Format the result
	cmd := exec.Command("gofumpt", "-l", "-w", "out")
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
		fmt.Println("- dest: Path to write the resuling Go-to-SQL layer to (e.g. db.go)")
		fmt.Println("- pkg: Package name to use in the resuling Go code (e.g. db)")
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
