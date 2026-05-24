package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/voyyar/mockly/internal/config"
	"github.com/voyyar/mockly/internal/database"
	"github.com/voyyar/mockly/internal/faker"
)

// Server wraps the HTTP server and database connection
type Server struct {
	db     *database.DB
	config *config.Config
	port   int
	gen    *faker.Generator
	mux    *http.ServeMux
}

// New creates a new Server instance
func New(db *database.DB, cfg *config.Config, port int) *Server {
	gen := faker.New(0)
	return &Server{
		db:     db,
		config: cfg,
		port:   port,
		gen:    gen,
		mux:    http.NewServeMux(),
	}
}

// SetupRoutes configures all HTTP routes dynamically based on the schema
func (s *Server) SetupRoutes() {
	// Register routes for each table in the schema
	for tableName, fields := range s.config.Schema {
		pkColumn := database.GetPrimaryKeyColumn(fields)
		s.registerTableRoutes(tableName, pkColumn)
	}

	// Generic POST handler for any table — validates incoming data against the
	// schema, auto-fills system columns (UUID, timestamp), and inserts securely.
	s.mux.HandleFunc("POST /api/{table}", s.HandlePost())

	// Add health check endpoint
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	// Add root endpoint
	s.mux.HandleFunc("GET /", s.handleRoot)
}

// registerTableRoutes registers CRUD routes for a specific table
func (s *Server) registerTableRoutes(tableName, pkColumn string) {
	// List all records with optional pagination: GET /api/{table}?limit=100&offset=0
	s.mux.HandleFunc(fmt.Sprintf("GET /api/%s", tableName), func(w http.ResponseWriter, r *http.Request) {
		s.handleGetAll(w, r, tableName)
	})

	// Get single record: GET /api/{table}/{id}
	s.mux.HandleFunc(fmt.Sprintf("GET /api/%s/{%s}", tableName, "id"), func(w http.ResponseWriter, r *http.Request) {
		s.handleGetOne(w, r, tableName, pkColumn)
	})

	// Create record: POST /api/{table}
	// Note: Per-table POST is replaced by the generic POST /api/{table} handler in SetupRoutes
	// which validates incoming data against the schema and auto-fills system columns.

	// Delete record: DELETE /api/{table}/{id}
	s.mux.HandleFunc(fmt.Sprintf("DELETE /api/%s/{%s}", tableName, "id"), func(w http.ResponseWriter, r *http.Request) {
		s.handleDelete(w, r, tableName, pkColumn)
	})
}

// Start begins listening on the configured port
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("�� Mockly API server running at http://localhost%s", addr)
	log.Printf("📋 Endpoints:")
	for tableName := range s.config.Schema {
		log.Printf("   GET    /api/%s?limit=100&offset=0", tableName)
		log.Printf("   GET    /api/%s/{id}", tableName)
		log.Printf("   POST   /api/%s", tableName)
		log.Printf("   DELETE /api/%s/{id}", tableName)
	}
	log.Printf("   GET    /api/health")
	log.Printf("Press Ctrl+C to stop the server")

	return http.ListenAndServe(addr, s.mux)
}

// handleHealth returns a simple health check response
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"service": "Mockly",
	})
}

// handleRoot returns available endpoints
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	endpoints := make([]string, 0)
	for tableName := range s.config.Schema {
		endpoints = append(endpoints,
			fmt.Sprintf("GET /api/%s", tableName),
			fmt.Sprintf("GET /api/%s/{id}", tableName),
			fmt.Sprintf("POST /api/%s", tableName),
			fmt.Sprintf("DELETE /api/%s/{id}", tableName),
		)
	}
	endpoints = append(endpoints, "GET /api/health")

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"service":   "Mockly",
		"endpoints": endpoints,
	})
}

// handleGetAll returns all records from a table with optional pagination
func (s *Server) handleGetAll(w http.ResponseWriter, r *http.Request, tableName string) {
	// Parse pagination query parameters
	limit, offset := parsePaginationParams(r)

	var rows []map[string]interface{}
	var err error

	if limit > 0 {
		rows, err = s.db.GetAllRowsPaged(tableName, limit, offset)
	} else {
		rows, err = s.db.GetAllRows(tableName)
	}

	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Failed to query data: "+err.Error())
		return
	}
	if rows == nil {
		rows = []map[string]interface{}{}
	}
	s.jsonResponse(w, http.StatusOK, rows)
}

// handleGetOne returns a single record by ID
func (s *Server) handleGetOne(w http.ResponseWriter, r *http.Request, tableName, pkColumn string) {
	id := r.PathValue("id")
	if id == "" {
		s.errorResponse(w, http.StatusBadRequest, "ID is required")
		return
	}

	row, err := s.db.GetRow(tableName, pkColumn, id)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Failed to query data: "+err.Error())
		return
	}
	if row == nil {
		s.errorResponse(w, http.StatusNotFound, fmt.Sprintf("Record with %s='%s' not found", pkColumn, id))
		return
	}
	s.jsonResponse(w, http.StatusOK, row)
}

// handleCreate creates a new record using generated fake data
func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request, tableName string) {
	fields := s.config.Schema[tableName]

	// Generate fake data for the new row
	row := s.gen.GenerateRow(fields)

	if err := s.db.InsertRow(tableName, row, ""); err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Failed to insert data: "+err.Error())
		return
	}

	s.jsonResponse(w, http.StatusCreated, row)
}

// handleDelete deletes a record by ID
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request, tableName, pkColumn string) {
	id := r.PathValue("id")
	if id == "" {
		s.errorResponse(w, http.StatusBadRequest, "ID is required")
		return
	}

	deleted, err := s.db.DeleteRow(tableName, pkColumn, id)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "Failed to delete data: "+err.Error())
		return
	}
	if !deleted {
		s.errorResponse(w, http.StatusNotFound, fmt.Sprintf("Record with %s='%s' not found", pkColumn, id))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"message": "Record deleted successfully",
		"id":      id,
	})
}

// parsePaginationParams extracts limit and offset from query parameters
func parsePaginationParams(r *http.Request) (limit, offset int) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return limit, offset
}

// jsonResponse writes a JSON response
func (s *Server) jsonResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// errorResponse writes an error JSON response
func (s *Server) errorResponse(w http.ResponseWriter, statusCode int, message string) {
	s.jsonResponse(w, statusCode, map[string]interface{}{
		"error":   http.StatusText(statusCode),
		"message": message,
	})
}

// extractTableAndID parses the URL path to extract table name and optional ID
func extractTableAndID(path string) (tableName string, id string) {
	parts := strings.Split(strings.TrimPrefix(path, "/api/"), "/")
	if len(parts) > 0 {
		tableName = parts[0]
	}
	if len(parts) > 1 {
		id = parts[1]
	}
	return
}
