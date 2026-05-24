package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// HandlePost processes POST requests with user-submitted JSON data.
// It validates incoming fields against the schema, auto-fills system columns
// (UUID, timestamp), and inserts the data securely using parameterized queries.
func (s *Server) HandlePost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract the table name from the Go 1.22+ wildcard path
		tableName := r.PathValue("table")

		fields, exists := s.config.Schema[tableName]
		if !exists {
			s.errorResponse(w, http.StatusNotFound, fmt.Sprintf("Table '%s' not found in schema", tableName))
			return
		}

		// 2. Decode the incoming JSON payload into a generic map
		var inputData map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&inputData); err != nil {
			s.errorResponse(w, http.StatusBadRequest, "Invalid JSON payload syntax")
			return
		}

		// 3. Match inputs against the defined schema constraints
		finalData := make(map[string]interface{})

		for colName, fieldSchema := range fields {
			userVal, provided := inputData[colName]

			switch fieldSchema.Type {
			case "uuid":
				// Auto-generate UUID if not provided or empty
				if !provided || userVal == nil || userVal == "" {
					finalData[colName] = uuid.New().String()
				} else {
					finalData[colName] = userVal
				}

			case "timestamp":
				// Auto-fill current timestamp if not provided or empty
				if !provided || userVal == nil || userVal == "" {
					finalData[colName] = time.Now().Format(time.RFC3339)
				} else {
					finalData[colName] = userVal
				}

			case "increment":
				// Let SQLite handle native AUTOINCREMENT — skip this column
				continue

			case "number":
				if !provided || userVal == nil {
					finalData[colName] = 0
				} else {
					finalData[colName] = userVal
				}

			default:
				// For all other types, use user value or zero-value default
				if !provided {
					finalData[colName] = getDefaultValueForType(fieldSchema.Type)
				} else {
					finalData[colName] = userVal
				}
			}
		}

		// 4. Insert securely using the existing parameterized InsertRow method
		if err := s.db.InsertRow(tableName, finalData, ""); err != nil {
			s.errorResponse(w, http.StatusInternalServerError, "Database write failure: "+err.Error())
			return
		}

		// 5. Send back a clean success response
		s.jsonResponse(w, http.StatusCreated, map[string]interface{}{
			"status":  "success",
			"message": fmt.Sprintf("Successfully appended new record into %s", tableName),
		})
	}
}

// getDefaultValueForType returns a safe zero-value default for the given schema type.
// This prevents SQL null errors when a client submits a partial JSON body.
func getDefaultValueForType(t string) interface{} {
	switch t {
	case "float", "price":
		return 0.0
	case "number":
		return 0
	case "boolean":
		return false
	case "text", "name", "email", "phone", "address", "city", "country", "url", "username", "password", "":
		return ""
	default:
		return ""
	}
}
