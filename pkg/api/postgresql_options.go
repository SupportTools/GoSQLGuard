package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/database"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
)

// PostgreSQLOptionsHandler handles PostgreSQL-specific database options API endpoints
type PostgreSQLOptionsHandler struct {
	Config *config.AppConfig
	Logger *logrus.Logger
}

// PostgreSQLOptionsRequest represents a request for PostgreSQL database options
type PostgreSQLOptionsRequest struct {
	Global    *database.PostgreSQLDumpOptions            `json:"global,omitempty"`
	PerServer map[string]*database.PostgreSQLDumpOptions `json:"per_server,omitempty"`
}

// PostgreSQLOptionsResponse represents the response containing PostgreSQL database options
type PostgreSQLOptionsResponse struct {
	Success   bool                                           `json:"success"`
	Message   string                                         `json:"message"`
	Global    *config.PostgreSQLDumpOptionsConfig            `json:"global,omitempty"`
	PerServer map[string]*config.PostgreSQLDumpOptionsConfig `json:"per_server,omitempty"`
}

// NewPostgreSQLOptionsHandler creates a new handler for PostgreSQL options endpoints
func NewPostgreSQLOptionsHandler(cfg *config.AppConfig, logger *logrus.Logger) *PostgreSQLOptionsHandler {
	return &PostgreSQLOptionsHandler{
		Config: cfg,
		Logger: logger,
	}
}

// RegisterRoutes registers the PostgreSQL options API routes
func (h *PostgreSQLOptionsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/postgresql-options", h.handlePostgreSQLOptions)
	mux.HandleFunc("/api/postgresql-options/server", h.handleServerPostgreSQLOptions)
}

func (h *PostgreSQLOptionsHandler) handlePostgreSQLOptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getPostgreSQLOptions(w, r)
	case http.MethodPut, http.MethodPost:
		h.updatePostgreSQLOptions(w, r)
	default:
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *PostgreSQLOptionsHandler) getPostgreSQLOptions(w http.ResponseWriter, r *http.Request) {
	response := PostgreSQLOptionsResponse{
		Success:   true,
		Global:    &h.Config.PostgreSQLDumpOptions,
		PerServer: make(map[string]*config.PostgreSQLDumpOptionsConfig),
	}

	// Collect per-server PostgreSQL options
	for _, server := range h.Config.DatabaseServers {
		if server.Type == "postgresql" {
			response.PerServer[server.Name] = &server.PostgreSQLDumpOptions
		}
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *PostgreSQLOptionsHandler) updatePostgreSQLOptions(w http.ResponseWriter, r *http.Request) {
	var req PostgreSQLOptionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Update global options if provided
	if req.Global != nil {
		h.Config.PostgreSQLDumpOptions = convertToConfigPostgreSQLOptions(req.Global)
	}

	// Update per-server options if provided
	if req.PerServer != nil {
		for serverName, options := range req.PerServer {
			// Find the server in config
			for i, server := range h.Config.DatabaseServers {
				if server.Name == serverName && server.Type == "postgresql" {
					h.Config.DatabaseServers[i].PostgreSQLDumpOptions = convertToConfigPostgreSQLOptions(options)
					break
				}
			}
		}
	}

	// Save configuration if using MySQL config
	if h.Config.MetadataDB.Enabled {
		if err := h.savePostgreSQLOptionsToDatabase(); err != nil {
			h.sendError(w, fmt.Sprintf("Failed to save PostgreSQL options: %v", err), http.StatusInternalServerError)
			return
		}
	}

	response := PostgreSQLOptionsResponse{
		Success:   true,
		Message:   "PostgreSQL options updated successfully",
		Global:    &h.Config.PostgreSQLDumpOptions,
		PerServer: make(map[string]*config.PostgreSQLDumpOptionsConfig),
	}

	if req.PerServer != nil {
		for serverName, options := range req.PerServer {
			configOptions := convertToConfigPostgreSQLOptions(options)
			response.PerServer[serverName] = &configOptions
		}
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *PostgreSQLOptionsHandler) handleServerPostgreSQLOptions(w http.ResponseWriter, r *http.Request) {
	serverName := r.URL.Query().Get("server")
	if serverName == "" {
		h.sendError(w, "Server name is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getServerPostgreSQLOptions(w, serverName)
	case http.MethodPut, http.MethodPost:
		h.updateServerPostgreSQLOptions(w, r, serverName)
	case http.MethodDelete:
		h.deleteServerPostgreSQLOptions(w, serverName)
	default:
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *PostgreSQLOptionsHandler) getServerPostgreSQLOptions(w http.ResponseWriter, serverName string) {
	// Find the server
	var foundServer *config.DatabaseServerConfig
	for _, server := range h.Config.DatabaseServers {
		if server.Name == serverName {
			foundServer = &server
			break
		}
	}

	if foundServer == nil {
		h.sendError(w, "Server not found", http.StatusNotFound)
		return
	}

	if foundServer.Type != "postgresql" {
		h.sendError(w, "Server is not a PostgreSQL server", http.StatusBadRequest)
		return
	}

	response := PostgreSQLOptionsResponse{
		Success: true,
		PerServer: map[string]*config.PostgreSQLDumpOptionsConfig{
			serverName: &foundServer.PostgreSQLDumpOptions,
		},
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *PostgreSQLOptionsHandler) updateServerPostgreSQLOptions(w http.ResponseWriter, r *http.Request, serverName string) {
	var options database.PostgreSQLDumpOptions
	if err := json.NewDecoder(r.Body).Decode(&options); err != nil {
		h.sendError(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Find and update the server
	found := false
	for i, server := range h.Config.DatabaseServers {
		if server.Name == serverName {
			if server.Type != "postgresql" {
				h.sendError(w, "Server is not a PostgreSQL server", http.StatusBadRequest)
				return
			}
			h.Config.DatabaseServers[i].PostgreSQLDumpOptions = convertToConfigPostgreSQLOptions(&options)
			found = true
			break
		}
	}

	if !found {
		h.sendError(w, "Server not found", http.StatusNotFound)
		return
	}

	// Save configuration if using MySQL config
	if h.Config.MetadataDB.Enabled {
		if err := h.savePostgreSQLOptionsToDatabase(); err != nil {
			h.sendError(w, fmt.Sprintf("Failed to save PostgreSQL options: %v", err), http.StatusInternalServerError)
			return
		}
	}

	configOptions := convertToConfigPostgreSQLOptions(&options)
	response := PostgreSQLOptionsResponse{
		Success: true,
		Message: fmt.Sprintf("PostgreSQL options updated for server: %s", serverName),
		PerServer: map[string]*config.PostgreSQLDumpOptionsConfig{
			serverName: &configOptions,
		},
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *PostgreSQLOptionsHandler) deleteServerPostgreSQLOptions(w http.ResponseWriter, serverName string) {
	// Find and clear the server's PostgreSQL options
	found := false
	for i, server := range h.Config.DatabaseServers {
		if server.Name == serverName {
			if server.Type != "postgresql" {
				h.sendError(w, "Server is not a PostgreSQL server", http.StatusBadRequest)
				return
			}
			h.Config.DatabaseServers[i].PostgreSQLDumpOptions = config.PostgreSQLDumpOptionsConfig{}
			found = true
			break
		}
	}

	if !found {
		h.sendError(w, "Server not found", http.StatusNotFound)
		return
	}

	// Save configuration if using MySQL config
	if h.Config.MetadataDB.Enabled {
		if err := h.savePostgreSQLOptionsToDatabase(); err != nil {
			h.sendError(w, fmt.Sprintf("Failed to save PostgreSQL options: %v", err), http.StatusInternalServerError)
			return
		}
	}

	response := PostgreSQLOptionsResponse{
		Success: true,
		Message: fmt.Sprintf("PostgreSQL options removed for server: %s", serverName),
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *PostgreSQLOptionsHandler) savePostgreSQLOptionsToDatabase() error {
	db := metadata.DB
	if db == nil {
		return fmt.Errorf("metadata database not initialized")
	}

	// Save global options
	// Convert config options to database options for saving
	globalDBOptions := database.PostgreSQLDumpOptions{
		Format:              h.Config.PostgreSQLDumpOptions.Format,
		Verbose:             h.Config.PostgreSQLDumpOptions.Verbose,
		NoComments:          h.Config.PostgreSQLDumpOptions.NoComments,
		SchemaOnly:          h.Config.PostgreSQLDumpOptions.SchemaOnly,
		DataOnly:            h.Config.PostgreSQLDumpOptions.DataOnly,
		Blobs:               h.Config.PostgreSQLDumpOptions.Blobs,
		NoBlobs:             h.Config.PostgreSQLDumpOptions.NoBlobs,
		Clean:               h.Config.PostgreSQLDumpOptions.Clean,
		Create:              h.Config.PostgreSQLDumpOptions.Create,
		IfExists:            h.Config.PostgreSQLDumpOptions.IfExists,
		NoOwner:             h.Config.PostgreSQLDumpOptions.NoOwner,
		NoPrivileges:        h.Config.PostgreSQLDumpOptions.NoPrivileges,
		NoTablespaces:       h.Config.PostgreSQLDumpOptions.NoTablespaces,
		NoPassword:          h.Config.PostgreSQLDumpOptions.NoPassword,
		InsertColumns:       h.Config.PostgreSQLDumpOptions.InsertColumns,
		OnConflictDoNothing: h.Config.PostgreSQLDumpOptions.OnConflictDoNothing,
		Jobs:                h.Config.PostgreSQLDumpOptions.Jobs,
		Compress:            h.Config.PostgreSQLDumpOptions.Compress,
		CustomOptions:       h.Config.PostgreSQLDumpOptions.CustomOptions,
	}
	globalJSON, err := json.Marshal(globalDBOptions)
	if err != nil {
		return fmt.Errorf("failed to marshal global PostgreSQL options: %w", err)
	}

	query := `
		INSERT INTO postgresql_options (id, name, options, updated_at)
		VALUES (1, 'global', ?, NOW())
		ON DUPLICATE KEY UPDATE
			options = VALUES(options),
			updated_at = NOW()
	`

	if err := db.Exec(query, string(globalJSON)).Error; err != nil {
		return fmt.Errorf("failed to save global PostgreSQL options to database: %w", err)
	}

	// Save per-server options
	for _, server := range h.Config.DatabaseServers {
		if server.Type == "postgresql" {
			// Convert config options to database options for saving
			dbOptions := database.PostgreSQLDumpOptions{
				Format:              server.PostgreSQLDumpOptions.Format,
				Verbose:             server.PostgreSQLDumpOptions.Verbose,
				NoComments:          server.PostgreSQLDumpOptions.NoComments,
				SchemaOnly:          server.PostgreSQLDumpOptions.SchemaOnly,
				DataOnly:            server.PostgreSQLDumpOptions.DataOnly,
				Blobs:               server.PostgreSQLDumpOptions.Blobs,
				NoBlobs:             server.PostgreSQLDumpOptions.NoBlobs,
				Clean:               server.PostgreSQLDumpOptions.Clean,
				Create:              server.PostgreSQLDumpOptions.Create,
				IfExists:            server.PostgreSQLDumpOptions.IfExists,
				NoOwner:             server.PostgreSQLDumpOptions.NoOwner,
				NoPrivileges:        server.PostgreSQLDumpOptions.NoPrivileges,
				NoTablespaces:       server.PostgreSQLDumpOptions.NoTablespaces,
				NoPassword:          server.PostgreSQLDumpOptions.NoPassword,
				InsertColumns:       server.PostgreSQLDumpOptions.InsertColumns,
				OnConflictDoNothing: server.PostgreSQLDumpOptions.OnConflictDoNothing,
				Jobs:                server.PostgreSQLDumpOptions.Jobs,
				Compress:            server.PostgreSQLDumpOptions.Compress,
				CustomOptions:       server.PostgreSQLDumpOptions.CustomOptions,
			}
			serverJSON, err := json.Marshal(dbOptions)
			if err != nil {
				if h.Logger != nil {
					h.Logger.Warnf("Failed to marshal PostgreSQL options for server %s: %v", server.Name, err)
				}
				continue
			}

			query := `
				INSERT INTO postgresql_options (server_name, name, options, updated_at)
				VALUES (?, 'server', ?, NOW())
				ON DUPLICATE KEY UPDATE
					options = VALUES(options),
					updated_at = NOW()
			`

			if err := db.Exec(query, server.Name, string(serverJSON)).Error; err != nil {
				if h.Logger != nil {
					h.Logger.Warnf("Failed to save PostgreSQL options for server %s: %v", server.Name, err)
				}
			}
		}
	}

	// Increment config version to trigger reload
	if err := db.Exec("UPDATE config_version SET version = version + 1, updated_at = NOW() WHERE id = 1").Error; err != nil {
		if h.Logger != nil {
			h.Logger.Warnf("Failed to increment config version: %v", err)
		}
	}

	return nil
}

// convertToConfigPostgreSQLOptions converts database.PostgreSQLDumpOptions to config.PostgreSQLDumpOptionsConfig
func convertToConfigPostgreSQLOptions(opts *database.PostgreSQLDumpOptions) config.PostgreSQLDumpOptionsConfig {
	if opts == nil {
		return config.PostgreSQLDumpOptionsConfig{}
	}
	return config.PostgreSQLDumpOptionsConfig{
		Format:              opts.Format,
		Verbose:             opts.Verbose,
		NoComments:          opts.NoComments,
		SchemaOnly:          opts.SchemaOnly,
		DataOnly:            opts.DataOnly,
		Blobs:               opts.Blobs,
		NoBlobs:             opts.NoBlobs,
		Clean:               opts.Clean,
		Create:              opts.Create,
		IfExists:            opts.IfExists,
		NoOwner:             opts.NoOwner,
		NoPrivileges:        opts.NoPrivileges,
		NoTablespaces:       opts.NoTablespaces,
		NoPassword:          opts.NoPassword,
		InsertColumns:       opts.InsertColumns,
		OnConflictDoNothing: opts.OnConflictDoNothing,
		Jobs:                opts.Jobs,
		Compress:            opts.Compress,
		CustomOptions:       opts.CustomOptions,
	}
}

func (h *PostgreSQLOptionsHandler) sendJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		if h.Logger != nil {
			h.Logger.Errorf("Failed to encode JSON response: %v", err)
		}
	}
}

func (h *PostgreSQLOptionsHandler) sendError(w http.ResponseWriter, message string, status int) {
	response := PostgreSQLOptionsResponse{
		Success: false,
		Message: message,
	}
	h.sendJSON(w, response, status)
}
