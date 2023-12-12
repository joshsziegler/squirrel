package main

import (
	"fmt"
	"os"
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

func main() {
	tables, err := ParseFile("schema.sql")
	if err != nil {
		panic(err.Error())
	}
	for _, table := range tables {
		fmt.Printf("%s %s (IfNotExists: %v, Temp: %v)\n", table.SchemaName, table.Name, table.IfNotExists, table.Temp)
		for _, col := range table.Columns {
			fmt.Printf("  - %+v\n", col)
		}

	}
}
