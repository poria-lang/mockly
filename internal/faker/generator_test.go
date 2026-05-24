package faker

import (
	"testing"

	"github.com/voyyar/mockly/internal/config"
)

func TestNew(t *testing.T) {
	g := New(42)
	if g == nil {
		t.Fatal("expected non-nil generator")
	}
}

func TestGenerateRow(t *testing.T) {
	g := New(42)

	schema := config.Schema{
		"id":    {Type: "uuid"},
		"name":  {Type: "name"},
		"email": {Type: "email"},
	}

	row := g.GenerateRow(schema)
	if row == nil {
		t.Fatal("expected non-nil row")
	}

	if _, ok := row["id"]; !ok {
		t.Error("expected 'id' field in row")
	}

	if _, ok := row["name"]; !ok {
		t.Error("expected 'name' field in row")
	}

	if _, ok := row["email"]; !ok {
		t.Error("expected 'email' field in row")
	}

	// Verify types
	id, ok := row["id"].(string)
	if !ok {
		t.Error("expected 'id' to be a string")
	}
	if id == "" {
		t.Error("expected non-empty id")
	}

	name, ok := row["name"].(string)
	if !ok {
		t.Error("expected 'name' to be a string")
	}
	if name == "" {
		t.Error("expected non-empty name")
	}
}

func TestGenerateRows(t *testing.T) {
	g := New(42)

	schema := config.Schema{
		"id":   {Type: "uuid"},
		"name": {Type: "name"},
	}

	rows := g.GenerateRows(schema, 10)
	if len(rows) != 10 {
		t.Errorf("expected 10 rows, got %d", len(rows))
	}

	for i, row := range rows {
		if row["id"] == "" {
			t.Errorf("row %d has empty id", i)
		}
		if row["name"] == "" {
			t.Errorf("row %d has empty name", i)
		}
	}
}

func TestGenerateRows_EmptySchema(t *testing.T) {
	g := New(42)
	schema := config.Schema{}

	rows := g.GenerateRows(schema, 5)
	if len(rows) != 5 {
		t.Errorf("expected 5 rows, got %d", len(rows))
	}
}

func TestGenerateRows_ZeroCount(t *testing.T) {
	g := New(42)
	schema := config.Schema{
		"id": {Type: "uuid"},
	}

	rows := g.GenerateRows(schema, 0)
	if len(rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(rows))
	}
}

func TestGenerateValue_AllTypes(t *testing.T) {
	g := New(42)

	tests := []struct {
		name      string
		fieldType string
	}{
		{"uuid", "uuid"},
		{"name", "name"},
		{"email", "email"},
		{"timestamp", "timestamp"},
		{"phone", "phone"},
		{"address", "address"},
		{"city", "city"},
		{"country", "country"},
		{"number", "number"},
		{"float", "float"},
		{"boolean", "boolean"},
		{"text", "text"},
		{"url", "url"},
		{"username", "username"},
		{"password", "password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := g.generateValue(tt.fieldType)
			if val == nil {
				t.Errorf("generateValue(%s) returned nil", tt.fieldType)
			}
		})
	}
}

func TestGenerateValue_UnknownType(t *testing.T) {
	g := New(42)
	val := g.generateValue("unknown_type")
	strVal, ok := val.(string)
	if !ok {
		t.Fatal("expected string result for unknown type")
	}
	expectedPrefix := "unknown_type:"
	if len(strVal) < len(expectedPrefix) || strVal[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("expected result starting with '%s', got '%s'", expectedPrefix, strVal)
	}
}

func TestGenerateValue_DeterministicSeed(t *testing.T) {
	g1 := New(42)
	g2 := New(42)

	// Test with types that use the seeded instance faker
	// (gofakeit global functions like UUID() and Name() are not seed-dependent)
	val1 := g1.generateValue("number")
	val2 := g2.generateValue("number")

	if val1 != val2 {
		t.Errorf("expected same number value with same seed, got %v vs %v", val1, val2)
	}
}

func TestValidateFieldType(t *testing.T) {
	validTypes := []string{
		"uuid", "name", "email", "timestamp", "phone",
		"address", "city", "country", "number", "float",
		"boolean", "text", "url", "username", "password",
	}

	invalidTypes := []string{"", "invalid", "DROP TABLE", "../etc"}

	for _, vt := range validTypes {
		if !ValidateFieldType(vt) {
			t.Errorf("expected '%s' to be valid", vt)
		}
	}

	for _, iv := range invalidTypes {
		if ValidateFieldType(iv) {
			t.Errorf("expected '%s' to be invalid", iv)
		}
	}
}

func TestGenerateValue_Boolean(t *testing.T) {
	g := New(42)
	hasTrue := false
	hasFalse := false

	for i := 0; i < 100; i++ {
		val := g.generateValue("boolean")
		b, ok := val.(bool)
		if !ok {
			t.Fatal("expected boolean value")
		}
		if b {
			hasTrue = true
		} else {
			hasFalse = true
		}
	}

	if !hasTrue {
		t.Error("expected at least one true value")
	}
	if !hasFalse {
		t.Error("expected at least one false value")
	}
}

func TestGenerateValue_Number(t *testing.T) {
	g := New(42)
	for i := 0; i < 100; i++ {
		val := g.generateValue("number")
		num, ok := val.(int)
		if !ok {
			t.Fatalf("expected int, got %T", val)
		}
		if num < 1 || num > 1000000 {
			t.Errorf("number out of range: %d", num)
		}
	}
}

func TestGenerateValue_Float(t *testing.T) {
	g := New(42)
	for i := 0; i < 100; i++ {
		val := g.generateValue("float")
		f, ok := val.(float64)
		if !ok {
			t.Fatalf("expected float64, got %T", val)
		}
		if f < 0 || f > 1000000 {
			t.Errorf("float out of range: %f", f)
		}
	}
}
