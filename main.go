package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/carlmjohnson/versioninfo"

	"github.com/joshsziegler/squirrel/config"
	"github.com/joshsziegler/squirrel/name"
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
func GenerateGoFromSQL(schemaPath, goPath, pkgName string, ignoreTables []string, ctxOnly bool) error {
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
	templates.Write(f, pkgName, tables, ignoreTables, ctxOnly)
	return nil
}

// printVersion prints build/version information.
func printVersion() {
	fmt.Printf("Version: %s\n", versioninfo.Version)
	fmt.Printf("Revision: %s\n", versioninfo.Revision)
	if versioninfo.DirtyBuild { // Don't print this for tagged/released versions
		fmt.Printf("Dirty: %v\n", versioninfo.DirtyBuild)
	}
	fmt.Printf("Built On: %s\n", versioninfo.LastCommit)
}

func main() {
	configPath := flag.String("config", "squirrel.yaml", "Path to the YAML config file")
	showVersion := flag.Bool("version", false, "Print version information and exit")
	flag.Usage = func() {
		fmt.Printf("Usage: %s [-config path]\n\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Println("")
		fmt.Println("")
		fmt.Println("squirrel reads its settings from a YAML config file (default: squirrel.yaml).")
		fmt.Println("")
		fmt.Println("Example squirrel.yaml:")
		fmt.Println("  schema: schema.sql      # Path to the SQL schema to parse (required)")
		fmt.Println("  dest: db.go             # Path to write the generated Go to (required)")
		fmt.Println("  package: db             # Package name for the generated Go (required)")
		fmt.Println("  ignore_tables:          # Tables to parse but exclude from the generated Go")
		fmt.Println("    - goose_db_version")
		fmt.Println("  ctx_only: true          # Only emit context-aware DB methods (default: true)")
		fmt.Println("  acronyms:               # SQL word -> Go form kept uppercase and not singularized")
		fmt.Println("    dns: DNS              # e.g. dns_zones -> DNSZone")
		fmt.Println("    oauth: OAuth")
		fmt.Println("")
	}
	flag.Parse()

	if *showVersion {
		printVersion()
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Println(err)
		flag.Usage()
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Register any user-defined acronyms so they are kept uppercase (and not
	// singularized) when SQL names are converted to Go names during parsing.
	name.RegisterAcronyms(cfg.Acronyms)

	err = GenerateGoFromSQL(cfg.Schema, cfg.Dest, cfg.Package, cfg.IgnoreTables, cfg.CtxOnly)
	if err != nil {
		fmt.Println(err)
		os.Exit(1) // Return an error code so the caller knows we failed.
	}
}
