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

func main() {
	tables, err := ParseFile("schema.sql")
	if err != nil {
		panic(err.Error())
	}

	// Write Go-SQL code to disk
	f, err := os.OpenFile("out", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	fmt.Fprintf(f, "package main\n\n")
	for _, table := range tables {
		table.ORM(f)
		fmt.Fprintf(f, "\n\n")
	}

	// Format the result
	cmd := exec.Command("gofumpt", "-l", "-w", "out")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error formatting output: %s\n", output)
		fmt.Printf("Error: %s\n", err.Error())
		return
	}
}
