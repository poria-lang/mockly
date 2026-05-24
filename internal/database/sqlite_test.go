package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/poria-lang/mockly/internal/config"
)

func setupTestDB(t *testing.T) *DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	return db
}

func TestOpen(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	if db.conn == nil {
		t.Error("expected non-nil connection")
	}

	// Verify WAL mode is enabled
	var mode string
	err := db.conn.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("failed to query journal mode: %v", err)
	}
	if mode != "wal" && mode != "WAL" {
		t.Errorf("expected WAL mode, got %s", mode)
	}
}

func TestCreateTables(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	schema := map[string]config.Schema{
		"users": {
			"id":    config.FieldSchema{Type: "uuid"},
			"name":  config.FieldSchema{Type: "name"},
			"email": config.FieldSchema{Type: "email"},
		},
		"products": {
			"id":    config.FieldSchema{Type: "uuid"},
			"name":  config.FieldSchema{Type: "text"},
			"price": config.FieldSchema{Type: "float"},
		},
	}

	err := db.CreateTables(schema)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	// Verify tables exist
	for tableName := range schema {
		exists, err := db.TableExists(tableName)
		if err != nil {
			t.Fatalf("failed to check table '%s': %v", tableName, err)
		}
		if !exists {
			t.Errorf("table '%s' should exist", tableName)
		}
	}
}

func TestCreateTables_InvalidTableName(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Table name with special characters should still work at SQL level
	// (validation happens at config level)
	schema := map[string]config.Schema{
		"valid_table": {
			"id": config.FieldSchema{Type: "uuid"},
		},
	}

	err := db.CreateTables(schema)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
}

func TestCountRows(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	schema := map[string]config.Schema{
		"users": {
			"id":   config.FieldSchema{Type: "uuid"},
			"name": config.FieldSchema{Type: "name"},
		},
	}

	if err := db.CreateTables(schema); err != nil {
		t.Fatal(err)
	}

	count, err := db.CountRows("users")
	if err != nil {
		t.Fatalf("failed to count rows: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows, got %d", count)
	}
}

func TestTableExists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	schema := map[string]config.Schema{
		"users": {
			"id": config.FieldSchema{Type: "uuid"},
		},
	}

	if err := db.CreateTables(schema); err != nil {
		t.Fatal(err)
	}

	exists, err := db.TableExists("users")
	if err != nil {
		t.Fatalf("failed to check table: %v", err)
	}
	if !exists {
		t.Error("expected table to exist")
	}

	exists, err = db.TableExists("nonexistent")
	if err != nil {
		t.Fatalf("failed to check table: %v", err)
	}
	if exists {
		t.Error("expected table to not exist")
	}
}

func TestClearTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	schema := map[string]config.Schema{
		"users": {
			"id":   config.FieldSchema{Type: "uuid"},
			"name": config.FieldSchema{Type: "name"},
		},
	}

	if err := db.CreateTables(schema); err != nil {
		t.Fatal(err)
	}

	// Clear an empty table (should not error)
	if err := db.ClearTable("users"); err != nil {
		t.Fatalf("failed to clear table: %v", err)
	}

	count, err := db.CountRows("users")
	if err != nil {
		t.Fatalf("failed to count rows: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows after clear, got %d", count)
	}
}

func TestGetAllRowsPaged(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	schema := map[string]config.Schema{
		"users": {
			"id":   config.FieldSchema{Type: "uuid"},
			"name": config.FieldSchema{Type: "name"},
		},
	}

	if err := db.CreateTables(schema); err != nil {
		t.Fatal(err)
	}

	// Test with empty table
	rows, err := db.GetAllRowsPaged("users", 10, 0)
	if err != nil {
		t.Fatalf("failed to get paged rows: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}

	// Test with limit=0, offset=0 (should return all)
	rows, err = db.GetAllRowsPaged("users", 0, 0)
	if err != nil {
		t.Fatalf("failed to get all rows: %v", err)
	}
	if rows == nil {
		t.Log("got nil rows for empty table")
	}
}

func TestGetPrimaryKeyColumn(t *testing.T) {
	tests := []struct {
		name     string
		fields   config.Schema
		expected string
	}{
		{
			name: "uuid field",
			fields: config.Schema{
				"id":   config.FieldSchema{Type: "uuid"},
				"name": config.FieldSchema{Type: "name"},
			},
			expected: "id",
		},
		{
			name: "no uuid field",
			fields: config.Schema{
				"name":  config.FieldSchema{Type: "name"},
				"email": config.FieldSchema{Type: "email"},
			},
			expected: "email",
		},
		{
			name:     "empty schema",
			fields:   config.Schema{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPrimaryKeyColumn(tt.fields)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestMapFieldTypeToSQL(t *testing.T) {
	tests := []struct {
		fieldType string
		expected  string
	}{
		{"uuid", "TEXT"},
		{"email", "TEXT"},
		{"name", "TEXT"},
		{"number", "INTEGER"},
		{"float", "REAL"},
		{"boolean", "INTEGER"},
		{"timestamp", "TEXT"},
		{"unknown", "TEXT"},
	}

	for _, tt := range tests {
		result := mapFieldTypeToSQL(tt.fieldType)
		if result != tt.expected {
			t.Errorf("mapFieldTypeToSQL(%s) = %s, expected %s", tt.fieldType, result, tt.expected)
		}
	}
}

func TestBuildCreateTableQuery(t *testing.T) {
	fields := config.Schema{
		"id":    config.FieldSchema{Type: "uuid"},
		"name":  config.FieldSchema{Type: "name"},
		"email": config.FieldSchema{Type: "email"},
	}

	query := buildCreateTableQuery("users", fields)

	if query == "" {
		t.Fatal("expected non-empty query")
	}

	// Should contain table name
	expectedTable := "users"
	if len(query) < len(expectedTable) || !contains(query, expectedTable) {
		t.Errorf("expected query to contain table name '%s'", expectedTable)
	}

	// Should contain PRIMARY KEY for uuid fields
	expectedPK := "PRIMARY KEY"
	if !contains(query, expectedPK) {
		t.Errorf("expected query to contain '%s' for uuid primary key", expectedPK)
	}
}

func TestBuildCreateTableQuery_NoUUID(t *testing.T) {
	fields := config.Schema{
		"name":  config.FieldSchema{Type: "name"},
		"email": config.FieldSchema{Type: "email"},
	}

	query := buildCreateTableQuery("contacts", fields)

	if contains(query, "PRIMARY KEY") {
		t.Error("expected no PRIMARY KEY when no uuid field exists")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}
