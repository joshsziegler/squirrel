// Package config loads squirrel's settings from a YAML file.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all settings that drive code generation. It is normally loaded
// from a YAML file (squirrel.yaml by default).
type Config struct {
	// Schema is the path to the SQL schema to parse (required).
	Schema string `yaml:"schema"`
	// Dest is the path to write the generated Go to (required).
	Dest string `yaml:"dest"`
	// Package is the package name to use in the generated Go (required).
	Package string `yaml:"package"`
	// IgnoreTables lists tables that are parsed but excluded from the generated Go.
	IgnoreTables []string `yaml:"ignore_tables"`
	// CtxOnly emits only context-aware DB methods (e.g. ExecContext) when true.
	// Defaults to true when omitted from the config file.
	CtxOnly bool `yaml:"ctx_only"`
}

// Load reads and parses the YAML config file at path. Defaults are applied
// before unmarshaling, so keys omitted from the file keep their default values.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Set defaults before unmarshaling. yaml.v3 leaves fields untouched when
	// their key is absent, so these survive unless the file overrides them.
	cfg := Config{CtxOnly: true}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return &cfg, nil
}

// Validate returns an error if any required field is missing.
func (c *Config) Validate() error {
	if c.Schema == "" {
		return fmt.Errorf("config: 'schema' is required")
	}
	if c.Dest == "" {
		return fmt.Errorf("config: 'dest' is required")
	}
	if c.Package == "" {
		return fmt.Errorf("config: 'package' is required")
	}
	return nil
}
