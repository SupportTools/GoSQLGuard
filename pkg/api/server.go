package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	dbmeta "github.com/supporttools/GoSQLGuard/pkg/database/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"

	// Database drivers
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// ServerHandler handles server management API endpoints
type ServerHandler struct {
	serverRepo *dbmeta.ServerRepository
}

// NewServerHandler creates a new server handler
func NewServerHandler() *ServerHandler {
	log.Printf("DEBUG: Creating ServerHandler, metadata.DB is nil: %v", metadata.DB == nil)
	if metadata.DB == nil {
		log.Println("Warning: Database is not initialized, server management API will not work")
		return &ServerHandler{}
	}

	return &ServerHandler{
		serverRepo: dbmeta.NewServerRepository(metadata.DB),
	}
}

// RegisterRoutes registers the server API routes on the provided mux
func (h *ServerHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/servers", h.handleServers)
	mux.HandleFunc("/api/servers/test", h.handleTestConnection)
	mux.HandleFunc("/api/servers/delete", h.handleDeleteServer)
}

// handleServers handles GET and POST requests for server management
func (h *ServerHandler) handleServers(w http.ResponseWriter, r *http.Request) {
	if h.serverRepo == nil {
		http.Error(w, "Server management is not available: database not initialized", http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getServers(w, r)
	case http.MethodPost:
		h.createOrUpdateServer(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getServers returns all servers or a specific server
func (h *ServerHandler) getServers(w http.ResponseWriter, r *http.Request) {
	// Check if a specific server is requested
	serverID := r.URL.Query().Get("id")
	if serverID != "" {
		server, err := h.serverRepo.GetServerByID(serverID)
		if err != nil {
			http.Error(w, "Server not found: "+err.Error(), http.StatusNotFound)
			return
		}

		// Convert to response type
		response := convertServerToResponse(server)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Otherwise, return all servers
	servers, err := h.serverRepo.GetAllServers()
	if err != nil {
		http.Error(w, "Failed to retrieve servers: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to response type
	var response []serverResponse
	for _, server := range servers {
		response = append(response, convertServerToResponse(&server))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// serverRequest is the request structure for creating/updating a server
type serverRequest struct {
	ID               string   `json:"id,omitempty"`
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	Host             string   `json:"host"`
	Port             string   `json:"port"`
	Username         string   `json:"username"`
	Password         string   `json:"password"`
	AuthPlugin       string   `json:"authPlugin,omitempty"`
	IncludeDatabases []string `json:"includeDatabases,omitempty"`
	ExcludeDatabases []string `json:"excludeDatabases,omitempty"`
}

// serverResponse is the response structure for server information
type serverResponse struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Type             string    `json:"type"`
	Host             string    `json:"host"`
	Port             string    `json:"port"`
	Username         string    `json:"username"`
	AuthPlugin       string    `json:"authPlugin,omitempty"`
	IncludeDatabases []string  `json:"includeDatabases,omitempty"`
	ExcludeDatabases []string  `json:"excludeDatabases,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// convertServerToResponse converts a ServerConfig to a serverResponse
func convertServerToResponse(server *dbmeta.ServerConfig) serverResponse {
	resp := serverResponse{
		ID:         server.ID,
		Name:       server.Name,
		Type:       server.Type,
		Host:       server.Host,
		Port:       server.Port,
		Username:   server.Username,
		AuthPlugin: server.AuthPlugin,
		CreatedAt:  server.CreatedAt,
		UpdatedAt:  server.UpdatedAt,
	}

	// Process include/exclude databases
	for _, filter := range server.DatabaseFilters {
		if filter.FilterType == "include" {
			resp.IncludeDatabases = append(resp.IncludeDatabases, filter.DatabaseName)
		} else if filter.FilterType == "exclude" {
			resp.ExcludeDatabases = append(resp.ExcludeDatabases, filter.DatabaseName)
		}
	}

	return resp
}

// createOrUpdateServer handles creating or updating a server
func (h *ServerHandler) createOrUpdateServer(w http.ResponseWriter, r *http.Request) {
	var req serverRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Name == "" || req.Type == "" || req.Host == "" || req.Username == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Create the server config
	server := dbmeta.ServerConfig{
		Name:       req.Name,
		Type:       req.Type,
		Host:       req.Host,
		Port:       req.Port,
		Username:   req.Username,
		Password:   req.Password,
		AuthPlugin: req.AuthPlugin,
	}

	// Handle update vs create
	isUpdate := false
	var existing *dbmeta.ServerConfig

	// First check if we have an ID (explicit update)
	if req.ID != "" {
		// This is an update by ID
		server.ID = req.ID
		var err error
		existing, err = h.serverRepo.GetServerByID(req.ID)
		if err != nil {
			http.Error(w, "Server not found: "+err.Error(), http.StatusNotFound)
			return
		}
		isUpdate = true
	} else {
		// Check if a server with this name already exists
		servers, err := h.serverRepo.GetAllServers()
		if err != nil {
			http.Error(w, "Failed to check existing servers: "+err.Error(), http.StatusInternalServerError)
			return
		}

		for _, s := range servers {
			if s.Name == req.Name {
				existing = &s
				server.ID = s.ID
				isUpdate = true
				break
			}
		}

		// If not found, this is a new server
		if !isUpdate {
			server.ID = uuid.New().String()
			server.CreatedAt = time.Now()
		}
	}

	// If updating, preserve creation time
	if isUpdate && existing != nil {
		server.CreatedAt = existing.CreatedAt
	}

	server.UpdatedAt = time.Now()

	// Add database filters
	for _, dbName := range req.IncludeDatabases {
		server.DatabaseFilters = append(server.DatabaseFilters, dbmeta.ServerDatabaseFilter{
			ServerID:     server.ID,
			FilterType:   "include",
			DatabaseName: dbName,
			CreatedAt:    time.Now(),
		})
	}

	for _, dbName := range req.ExcludeDatabases {
		server.DatabaseFilters = append(server.DatabaseFilters, dbmeta.ServerDatabaseFilter{
			ServerID:     server.ID,
			FilterType:   "exclude",
			DatabaseName: dbName,
			CreatedAt:    time.Now(),
		})
	}

	// Save to database
	if isUpdate {
		log.Printf("Updating existing server: %s (ID: %s)", server.Name, server.ID)
		err = h.serverRepo.UpdateServer(&server)
	} else {
		log.Printf("Creating new server: %s (ID: %s)", server.Name, server.ID)
		err = h.serverRepo.CreateServer(&server)
	}

	if err != nil {
		http.Error(w, "Failed to save server: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update global configuration by reloading database servers
	go reloadConfigurationFromDatabase()

	// Return the server
	response := convertServerToResponse(&server)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDeleteServer handles deleting a server
func (h *ServerHandler) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	if h.serverRepo == nil {
		http.Error(w, "Server management is not available: database not initialized", http.StatusServiceUnavailable)
		return
	}

	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get server ID from query parameters
	serverID := r.URL.Query().Get("id")
	if serverID == "" {
		http.Error(w, "Server ID is required", http.StatusBadRequest)
		return
	}

	// Check if server exists
	exists, err := h.serverRepo.ServerExists(serverID)
	if err != nil {
		http.Error(w, "Failed to check server existence: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	// Delete the server
	err = h.serverRepo.DeleteServer(serverID)
	if err != nil {
		http.Error(w, "Failed to delete server: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update global configuration by reloading database servers
	go reloadConfigurationFromDatabase()

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Server deleted successfully",
	})
}

// handleTestConnection tests a database connection
func (h *ServerHandler) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req serverRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Test the database connection based on type
	var testErr error
	var databases []string

	if req.Type == "mysql" {
		// Test MySQL connection
		databases, testErr = testMySQLConnection(req)
	} else if req.Type == "postgresql" {
		// Test PostgreSQL connection
		databases, testErr = testPostgreSQLConnection(req)
	} else {
		http.Error(w, "Unsupported database type: "+req.Type, http.StatusBadRequest)
		return
	}

	if testErr != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Connection test failed: " + testErr.Error(),
		})
		return
	}

	// Return success with list of databases
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"message":   "Connection test successful",
		"databases": databases,
	})
}

// reloadConfigurationFromDatabase reloads server configurations from the database
func reloadConfigurationFromDatabase() {
	if metadata.DB == nil {
		log.Println("Cannot reload configuration: database not initialized")
		return
	}

	// Initialize server repository
	serverRepo := dbmeta.NewServerRepository(metadata.DB)

	// Get all servers from the database
	servers, err := serverRepo.GetAllServers()
	if err != nil {
		log.Printf("Failed to load server configurations from database: %v", err)
		return
	}

	// Clear existing server configurations from database
	var dbServers []config.DatabaseServerConfig
	for _, server := range servers {
		dbServer := config.DatabaseServerConfig{
			Name:       server.Name,
			Type:       server.Type,
			Host:       server.Host,
			Port:       server.Port,
			Username:   server.Username,
			Password:   server.Password,
			AuthPlugin: server.AuthPlugin,
		}

		// Process include/exclude databases
		for _, filter := range server.DatabaseFilters {
			if filter.FilterType == "include" {
				dbServer.IncludeDatabases = append(dbServer.IncludeDatabases, filter.DatabaseName)
			} else if filter.FilterType == "exclude" {
				dbServer.ExcludeDatabases = append(dbServer.ExcludeDatabases, filter.DatabaseName)
			}
		}

		dbServers = append(dbServers, dbServer)
	}

	// Critical section: update the global configuration
	// Replace file-based servers with database servers
	config.CFG.DatabaseServers = dbServers

	log.Printf("Successfully loaded %d server configurations from the database", len(servers))
}

// testMySQLConnection tests a MySQL connection and returns list of databases and any error
func testMySQLConnection(req serverRequest) ([]string, error) {
	// Set default port if not provided
	port := req.Port
	if port == "" {
		port = "3306"
	}

	// Build connection string
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/", req.Username, req.Password, req.Host, port)

	// Add auth plugin if specified
	if req.AuthPlugin != "" {
		dsn += fmt.Sprintf("?auth=%s", req.AuthPlugin)
	}

	// Open connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}
	defer db.Close()

	// Test connection
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Get list of databases
	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			continue
		}
		// Skip system databases
		if dbName != "information_schema" && dbName != "mysql" && dbName != "performance_schema" && dbName != "sys" {
			databases = append(databases, dbName)
		}
	}

	return databases, nil
}

// testPostgreSQLConnection tests a PostgreSQL connection and returns list of databases and any error
func testPostgreSQLConnection(req serverRequest) ([]string, error) {
	// Set default port if not provided
	port := req.Port
	if port == "" {
		port = "5432"
	}

	// Build connection string
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s sslmode=disable",
		req.Host, port, req.Username, req.Password)

	// Open connection
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}
	defer db.Close()

	// Test connection
	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Get list of databases
	query := `SELECT datname FROM pg_database 
		WHERE datistemplate = false 
		AND datname != 'postgres'
		ORDER BY datname`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			continue
		}
		databases = append(databases, dbName)
	}

	return databases, nil
}
