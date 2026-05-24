package faker

import (
	"fmt"
	"time"

	"github.com/brianvoe/gofakeit/v7"

	"github.com/voyyar/mockly/internal/config"
)

// Generator handles generating fake data for database seeding
type Generator struct {
	faker *gofakeit.Faker
}

// New creates a new Generator instance with an optional seed
func New(seed uint64) *Generator {
	var f *gofakeit.Faker
	if seed > 0 {
		f = gofakeit.New(seed)
	} else {
		f = gofakeit.New(uint64(time.Now().UnixNano()))
	}
	return &Generator{faker: f}
}

// GenerateRow generates a single row of fake data based on the schema
func (g *Generator) GenerateRow(schema config.Schema) map[string]interface{} {
	row := make(map[string]interface{})
	for fieldName, fieldSchema := range schema {
		row[fieldName] = g.generateValue(fieldSchema.Type)
	}
	return row
}

// GenerateRows generates multiple rows of fake data
func (g *Generator) GenerateRows(schema config.Schema, count int) []map[string]interface{} {
	rows := make([]map[string]interface{}, 0, count)
	for i := 0; i < count; i++ {
		rows = append(rows, g.GenerateRow(schema))
	}
	return rows
}

// generateValue generates a single fake value based on its type
func (g *Generator) generateValue(fieldType string) interface{} {
	switch fieldType {
	case "uuid":
		return gofakeit.UUID()
	case "name":
		return gofakeit.Name()
	case "email":
		return gofakeit.Email()
	case "timestamp":
		// Generate a timestamp in ISO 8601 format within the last 5 years
		t := g.faker.DateRange(time.Now().AddDate(-5, 0, 0), time.Now())
		return t.Format(time.RFC3339)
	case "phone":
		return gofakeit.Phone()
	case "address":
		return gofakeit.Address().Address
	case "city":
		return gofakeit.City()
	case "country":
		return gofakeit.Country()
	case "number":
		return g.faker.Number(1, 1000000)
	case "float":
		return g.faker.Float64Range(0, 1000000)
	case "boolean":
		return g.faker.Bool()
	case "text":
		return g.faker.Sentence(10)
	case "url":
		return gofakeit.URL()
	case "username":
		return gofakeit.Username()
	case "password":
		return gofakeit.Password(true, true, true, true, false, 16)
	default:
		return fmt.Sprintf("unknown_type:%s", fieldType)
	}
}

// ValidateFieldType checks if the given field type is supported
func ValidateFieldType(fieldType string) bool {
	validTypes := []string{
		"uuid", "name", "email", "timestamp", "phone",
		"address", "city", "country", "number", "float",
		"boolean", "text", "url", "username", "password",
	}
	for _, t := range validTypes {
		if t == fieldType {
			return true
		}
	}
	return false
}
