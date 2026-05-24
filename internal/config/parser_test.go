package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	content := `{
		"seed_count": 50,
		"schema": {
			"users": {
				"id": {"type": "uuid"},
				"name": {"type": "name"}
			}
		}
	}`

	tmpFile := filepath.Join(t.TempDir(), "mockly.json")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.SeedCount != 50 {
		t.Errorf("expected seed_count 50, got %d", cfg.SeedCount)
	}

	if len(cfg.Schema) != 1 {
		t.Errorf("expected 1 table, got %d", len(cfg.Schema))
	}

	if _, ok := cfg.Schema["users"]; !ok {
		t.Error("expected 'users' table in schema")
	}
}

func TestLoad_InvalidSeedCount(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name: "zero seed count",
			content: `{
				"seed_count": 0,
				"schema": {"users": {"id": {"type": "uuid"}}}
			}`,
		},
		{
			name: "negative seed count",
			content: `{
				"seed_count": -1,
				"schema": {"users": {"id": {"type": "uuid"}}}
			}`,
		},
		{
			name: "exceeds max seed count",
			content: `{
				"seed_count": 10001,
				"schema": {"users": {"id": {"type": "uuid"}}}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "mockly.json")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			_, err := Load(tmpFile)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestLoad_EmptySchema(t *testing.T) {
	content := `{
		"seed_count": 50,
		"schema": {}
	}`

	tmpFile := filepath.Join(t.TempDir(), "mockly.json")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(tmpFile)
	if err == nil {
		t.Error("expected error for empty schema, got nil")
	}
}

func TestLoad_InvalidFieldType(t *testing.T) {
	content := `{
		"seed_count": 50,
		"schema": {
			"users": {
				"id": {"type": "invalid_type"}
			}
		}
	}`

	tmpFile := filepath.Join(t.TempDir(), "mockly.json")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(tmpFile)
	if err == nil {
		t.Error("expected error for invalid field type, got nil")
	}
}

func TestLoad_SQLInjectionTableName(t *testing.T) {
	content := `{
		"seed_count": 50,
		"schema": {
			"users; DROP TABLE users;": {
				"id": {"type": "uuid"}
			}
		}
	}`

	tmpFile := filepath.Join(t.TempDir(), "mockly.json")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(tmpFile)
	if err == nil {
		t.Error("expected error for SQL injection in table name, got nil")
	}
}

func TestLoad_SQLInjectionFieldName(t *testing.T) {
	content := `{
		"seed_count": 50,
		"schema": {
			"users": {
				"id; DELETE FROM users;": {"type": "uuid"}
			}
		}
	}`

	tmpFile := filepath.Join(t.TempDir(), "mockly.json")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(tmpFile)
	if err == nil {
		t.Error("expected error for SQL injection in field name, got nil")
	}
}

func TestSupportedTypes(t *testing.T) {
	types := SupportedTypes()
	expectedTypes := []string{
		"uuid", "name", "email", "timestamp", "phone",
		"address", "city", "country", "number", "float",
		"boolean", "text", "url", "username", "password",
	}

	if len(types) != len(expectedTypes) {
		t.Errorf("expected %d types, got %d", len(expectedTypes), len(types))
	}

	seen := make(map[string]bool)
	for _, t := range types {
		seen[t] = true
	}

	for _, et := range expectedTypes {
		if !seen[et] {
			t.Errorf("missing supported type: %s", et)
		}
	}
}

func TestFindConfigFile_NotFound(t *testing.T) {
	// Use a temporary directory with no config files
	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	_, err := FindConfigFile()
	if err == nil {
		t.Error("expected error when no config file exists, got nil")
	}
}

func TestIsValidType(t *testing.T) {
	validTypes := []string{"uuid", "name", "email", "number", "float", "boolean", "text"}
	invalidTypes := []string{"", "sql_injection", "DROP TABLE", "../etc/passwd"}

	for _, vt := range validTypes {
		if !isValidType(vt) {
			t.Errorf("expected '%s' to be a valid type", vt)
		}
	}

	for _, iv := range invalidTypes {
		if isValidType(iv) {
			t.Errorf("expected '%s' to be an invalid type", iv)
		}
	}
}

func TestFieldSchema_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected string
		wantErr  bool
	}{
		{
			name:     "string format",
			json:     `"uuid"`,
			expected: "uuid",
			wantErr:  false,
		},
		{
			name:     "object format",
			json:     `{"type": "email"}`,
			expected: "email",
			wantErr:  false,
		},
		{
			name:    "invalid format",
			json:    `123`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fs FieldSchema
			err := fs.UnmarshalJSON([]byte(tt.json))

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if fs.Type != tt.expected {
				t.Errorf("expected type %s, got %s", tt.expected, fs.Type)
			}
		})
	}
}
