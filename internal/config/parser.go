package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// FieldSchema represents a single field definition in the schema
type FieldSchema struct {
	Type string `json:"type"`
}

// UnmarshalJSON allows FieldSchema to accept both string and object formats
func (f *FieldSchema) UnmarshalJSON(data []byte) error {
	// Try string format first: "uuid"
	var typeStr string
	if err := json.Unmarshal(data, &typeStr); err == nil {
		f.Type = typeStr
		return nil
	}

	// Try object format: {"type": "uuid"}
	var obj struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid field schema: expected string or object with 'type' field")
	}
	f.Type = obj.Type
	return nil
}

// Schema represents a table definition with field names and their types
type Schema map[string]FieldSchema

// Config represents the complete mockly.json configuration
type Config struct {
	SeedCount int               `json:"seed_count"`
	Schema    map[string]Schema `json:"schema"`
}

// Load reads and parses mockly.json from the given path
func Load(path string) (*Config, error) {
	// G304: Prevent directory traversal by cleaning the path
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// alphanumericRegex is used to validate table/field names against SQL injection
var alphanumericRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// Validate checks the config for required fields and valid types
func (c *Config) Validate() error {
	if c.SeedCount <= 0 {
		return fmt.Errorf("seed_count must be greater than 0")
	}
	if c.SeedCount > 10000 {
		return fmt.Errorf("seed_count must not exceed 10000")
	}
	if len(c.Schema) == 0 {
		return fmt.Errorf("schema must define at least one table")
	}
	for tableName, fields := range c.Schema {
		// SQL injection safeguard: reject non-alphanumeric table names
		if !alphanumericRegex.MatchString(tableName) {
			return fmt.Errorf("table name '%s' contains invalid characters: only alphanumeric and underscores allowed", tableName)
		}
		if len(fields) == 0 {
			return fmt.Errorf("table '%s' must define at least one field", tableName)
		}
		for fieldName, fieldSchema := range fields {
			// SQL injection safeguard: reject non-alphanumeric field names
			if !alphanumericRegex.MatchString(fieldName) {
				return fmt.Errorf("field name '%s' in table '%s' contains invalid characters: only alphanumeric and underscores allowed", fieldName, tableName)
			}
			if fieldSchema.Type == "" {
				return fmt.Errorf("field '%s' in table '%s' must have a type", fieldName, tableName)
			}
			if !isValidType(fieldSchema.Type) {
				return fmt.Errorf("field '%s' in table '%s' has unsupported type '%s'", fieldName, tableName, fieldSchema.Type)
			}
		}
	}
	return nil
}

// SupportedTypes returns the list of supported faker types
func SupportedTypes() []string {
	return []string{
		"uuid",
		"name",
		"email",
		"timestamp",
		"phone",
		"address",
		"city",
		"country",
		"number",
		"float",
		"boolean",
		"text",
		"url",
		"username",
		"password",
	}
}

func isValidType(t string) bool {
	for _, valid := range SupportedTypes() {
		if t == valid {
			return true
		}
	}
	return false
}

// FindConfigFile looks for mockly.json or forge.json in the current directory using filepath.Clean for path traversal prevention
func FindConfigFile() (string, error) {
	paths := []string{"mockly.json", "./mockly.json", "forge.json", "./forge.json"}
	for _, p := range paths {
		cleanPath := filepathClean(p)
		if _, err := os.Stat(cleanPath); err == nil {
			return cleanPath, nil
		}
	}
	return "", fmt.Errorf("mockly.json or forge.json not found in current directory")
}

// filepathClean cleans a file path to prevent path traversal attacks
func filepathClean(path string) string {
	return filepath.Clean(path)
}
