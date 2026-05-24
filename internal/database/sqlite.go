package database

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/voyyar/mockly/internal/config"
)

// identifierRegex matches only allowed characters for table/column names
var identifierRegex = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// sanitizeIdentifier strips any character that is not alphanumeric or underscore.
// This prevents SQL injection via dynamically constructed table/column names.
func sanitizeIdentifier(name string) string {
	return identifierRegex.ReplaceAllString(name, "")
}

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
}

// Open opens or creates the SQLite database file
// dbPath is cleaned to prevent path traversal attacks
func Open(dbPath string) (*DB, error) {
	// Prevent path traversal by cleaning the path
	cleanPath := filepath.Clean(dbPath)

	conn, err := sql.Open("sqlite", cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Connection pool limits to prevent exhausting file descriptors
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)

	// Enable WAL mode for better concurrent read/write performance
	_, err = conn.Exec("PRAGMA journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	_, err = conn.Exec("PRAGMA foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Increase cache size for better bulk performance (-8000 = ~8MB)
	_, err = conn.Exec("PRAGMA cache_size=-8000")
	if err != nil {
		return nil, fmt.Errorf("failed to set cache size: %w", err)
	}

	// Use MEMORY temp store for better performance during bulk inserts
	_, err = conn.Exec("PRAGMA temp_store=MEMORY")
	if err != nil {
		return nil, fmt.Errorf("failed to set temp store: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// CreateTables creates tables based on the schema configuration and adds indexes
func (db *DB) CreateTables(schema map[string]config.Schema) error {
	for tableName, fields := range schema {
		sanitizedTable := sanitizeIdentifier(tableName)
		query := buildCreateTableQuery(sanitizedTable, fields)
		if _, err := db.conn.Exec(query); err != nil {
			return fmt.Errorf("failed to create table '%s': %w", sanitizedTable, err)
		}

		// Create indexes for non-uuid fields to optimize read performance
		if err := db.createIndexes(sanitizedTable, fields); err != nil {
			return fmt.Errorf("failed to create indexes for '%s': %w", sanitizedTable, err)
		}
	}
	return nil
}

// createIndexes creates indexes on non-primary key fields for better query performance
func (db *DB) createIndexes(tableName string, fields config.Schema) error {
	for fieldName, fieldSchema := range fields {
		// Skip UUID fields (they're already primary keys) and boolean fields
		if fieldSchema.Type == "uuid" || fieldSchema.Type == "boolean" {
			continue
		}

		sanitizedField := sanitizeIdentifier(fieldName)

		// Create index on fields that look like foreign keys (end with "_id")
		// and high-cardinality fields that are commonly filtered
		if strings.HasSuffix(sanitizedField, "_id") ||
			fieldSchema.Type == "email" ||
			fieldSchema.Type == "username" {
			indexName := fmt.Sprintf("idx_%s_%s", tableName, sanitizedField)
			query := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)", indexName, tableName, sanitizedField)
			if _, err := db.conn.Exec(query); err != nil {
				return fmt.Errorf("failed to create index '%s': %w", indexName, err)
			}
			log.Printf("   🌱 Created index '%s' on '%s.%s'", indexName, tableName, sanitizedField)
		}
	}
	return nil
}

// CountRows returns the number of rows currently in the specified table
func (db *DB) CountRows(tableName string) (int, error) {
	var count int
	sanitizedTable := sanitizeIdentifier(tableName)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", sanitizedTable)
	err := db.conn.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count rows in '%s': %w", sanitizedTable, err)
	}
	return count, nil
}

// ClearTable removes all rows from a table and runs VACUUM to reclaim disk space
func (db *DB) ClearTable(tableName string) error {
	sanitizedTable := sanitizeIdentifier(tableName)
	if _, err := db.conn.Exec(fmt.Sprintf("DELETE FROM %s", sanitizedTable)); err != nil {
		return fmt.Errorf("failed to clear table '%s': %w", sanitizedTable, err)
	}
	// Reclaim disk space after bulk deletion
	if _, err := db.conn.Exec("VACUUM"); err != nil {
		return fmt.Errorf("failed to vacuum after clearing '%s': %w", sanitizedTable, err)
	}
	return nil
}

// SeedTable seeds a table with the exact number of desired rows using Smart Wipe logic.
// It processes rows in chunks to avoid memory pressure with large datasets.
func (db *DB) SeedTable(tableName string, fields config.Schema, rows []map[string]interface{}, targetCount int) error {
	sanitizedTable := sanitizeIdentifier(tableName)

	// 1. Query the actual number of rows currently in the table
	existingCount, err := db.CountRows(sanitizedTable)
	if err != nil {
		return err
	}

	// 2. If counts match perfectly, skip seeding entirely
	if existingCount == targetCount {
		log.Printf("   ℹ️  Table '%s' already has exactly %d records, skipping seeding", sanitizedTable, targetCount)
		return nil
	}

	// 3. If counts don't match, wipe the table to start fresh
	if existingCount > 0 {
		log.Printf("   🔄 Count mismatch for '%s' (%d vs %d requested). Wiping table...", sanitizedTable, existingCount, targetCount)
		if err := db.ClearTable(sanitizedTable); err != nil {
			return err
		}
	}

	// 4. Insert rows in chunks of 500 for better memory management
	const chunkSize = 500
	totalChunks := (targetCount + chunkSize - 1) / chunkSize

	for chunk := 0; chunk < totalChunks; chunk++ {
		start := chunk * chunkSize
		end := start + chunkSize
		if end > targetCount {
			end = targetCount
		}

		chunkRows := rows[start:end]

		// Wrap each chunk inside a fast SQL transaction
		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction (chunk %d): %w", chunk, err)
		}

		// Prepare insert statement from schema fields
		columns := make([]string, 0, len(fields))
		for colName := range fields {
			columns = append(columns, sanitizeIdentifier(colName))
		}

		placeholders := make([]string, len(columns))
		for i := range placeholders {
			placeholders[i] = "?"
		}

		query := fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)",
			sanitizedTable,
			strings.Join(columns, ", "),
			strings.Join(placeholders, ", "),
		)

		stmt, err := tx.Prepare(query)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to prepare insert statement: %w", err)
		}

		// Execute all inserts against the transaction
		for _, row := range chunkRows {
			values := make([]interface{}, 0, len(columns))
			for _, col := range columns {
				values = append(values, row[col])
			}
			if _, err := stmt.Exec(values...); err != nil {
				tx.Rollback()
				stmt.Close()
				return fmt.Errorf("failed to insert row: %w", err)
			}
		}

		stmt.Close()

		// Commit chunk to disk
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction (chunk %d): %w", chunk, err)
		}

		log.Printf("   🌱 Inserted chunk %d/%d (%d records) into '%s'",
			chunk+1, totalChunks, len(chunkRows), sanitizedTable)
	}

	return nil
}

// InsertRow inserts a single row into the specified table
func (db *DB) InsertRow(tableName string, fields map[string]interface{}, idField string) error {
	sanitizedTable := sanitizeIdentifier(tableName)

	columns := make([]string, 0, len(fields))
	values := make([]interface{}, 0, len(fields))
	placeholders := make([]string, 0, len(fields))

	i := 1
	for colName, colValue := range fields {
		sanitizedCol := sanitizeIdentifier(colName)
		columns = append(columns, sanitizedCol)
		values = append(values, colValue)
		placeholders = append(placeholders, fmt.Sprintf("?%d", i))
		i++
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		sanitizedTable,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err := db.conn.Exec(query, values...)
	return err
}

// GetAllRows retrieves all rows from a table with optional pagination
func (db *DB) GetAllRows(tableName string) ([]map[string]interface{}, error) {
	return db.GetAllRowsPaged(tableName, 0, 0)
}

// GetAllRowsPaged retrieves a paginated subset of rows from a table
// Use limit=0 and offset=0 to fetch all rows (unlimited)
func (db *DB) GetAllRowsPaged(tableName string, limit, offset int) ([]map[string]interface{}, error) {
	sanitizedTable := sanitizeIdentifier(tableName)
	query := fmt.Sprintf("SELECT * FROM %s", sanitizedTable)
	if limit > 0 {
		query = fmt.Sprintf("%s LIMIT %d OFFSET %d", query, limit, offset)
	}

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query table '%s': %w", sanitizedTable, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string for JSON serialization
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return results, nil
}

// GetRow retrieves a single row by ID from a table
func (db *DB) GetRow(tableName, idColumn, idValue string) (map[string]interface{}, error) {
	sanitizedTable := sanitizeIdentifier(tableName)
	sanitizedIDCol := sanitizeIdentifier(idColumn)
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", sanitizedTable, sanitizedIDCol)
	row := db.conn.QueryRow(query, idValue)

	columns, err := db.getColumnNames(sanitizedTable)
	if err != nil {
		return nil, err
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := row.Scan(valuePtrs...); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	result := make(map[string]interface{})
	for i, col := range columns {
		val := values[i]
		if b, ok := val.([]byte); ok {
			result[col] = string(b)
		} else {
			result[col] = val
		}
	}

	return result, nil
}

// DeleteRow deletes a row by ID from a table
func (db *DB) DeleteRow(tableName, idColumn, idValue string) (bool, error) {
	sanitizedTable := sanitizeIdentifier(tableName)
	sanitizedIDCol := sanitizeIdentifier(idColumn)
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", sanitizedTable, sanitizedIDCol)
	result, err := db.conn.Exec(query, idValue)
	if err != nil {
		return false, fmt.Errorf("failed to delete row: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return affected > 0, nil
}

// TableExists checks if a table exists in the database
func (db *DB) TableExists(name string) (bool, error) {
	// The table name here is bound as a parameter to a prepared statement,
	// so sanitization is defense-in-depth. We still sanitize for consistency.
	sanitizedName := sanitizeIdentifier(name)
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?"
	var count int
	err := db.conn.QueryRow(query, sanitizedName).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// getColumnNames returns the column names for a table
func (db *DB) getColumnNames(tableName string) ([]string, error) {
	sanitizedTable := sanitizeIdentifier(tableName)
	query := fmt.Sprintf("PRAGMA table_info(%s)", sanitizedTable)
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get table info: %w", err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var defaultVal interface{}
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultVal, &pk); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, nil
}

// mapFieldTypeToSQL maps a faker type to a SQLite column type
func mapFieldTypeToSQL(fieldType string) string {
	switch fieldType {
	case "uuid", "email", "name", "phone", "address", "city", "country", "url", "username", "password", "text":
		return "TEXT"
	case "number":
		return "INTEGER"
	case "float":
		return "REAL"
	case "boolean":
		return "INTEGER"
	case "timestamp":
		return "TEXT"
	default:
		return "TEXT"
	}
}

// buildCreateTableQuery builds a CREATE TABLE SQL statement
func buildCreateTableQuery(tableName string, fields config.Schema) string {
	sanitizedTable := sanitizeIdentifier(tableName)

	var columns []string
	idColumn := ""

	// Check if there's an id/uuid field to use as primary key
	for fieldName, fieldSchema := range fields {
		sanitizedField := sanitizeIdentifier(fieldName)
		if fieldSchema.Type == "uuid" {
			idColumn = sanitizedField
		}
		colDef := fmt.Sprintf("%s %s", sanitizedField, mapFieldTypeToSQL(fieldSchema.Type))
		columns = append(columns, colDef)
	}

	// Add primary key constraint if a uuid/id field exists
	if idColumn != "" {
		columns = append(columns, fmt.Sprintf("PRIMARY KEY (%s)", idColumn))
	}

	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n)", sanitizedTable, strings.Join(columns, ",\n  "))
	return query
}

// GetPrimaryKeyColumn returns the primary key column name for a table
func GetPrimaryKeyColumn(fields config.Schema) string {
	for fieldName, fieldSchema := range fields {
		if fieldSchema.Type == "uuid" {
			return fieldName
		}
	}
	// Fallback to first field if no uuid found
	for fieldName := range fields {
		return fieldName
	}
	return ""
}
