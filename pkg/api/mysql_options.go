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

type MySQLOptionsHandler struct {
	Config *config.AppConfig
	Logger *logrus.Logger
}

type MySQLOptionsRequest struct {
	Global    *database.MySQLDumpOptions            `json:"global,omitempty"`
	PerServer map[string]*database.MySQLDumpOptions `json:"per_server,omitempty"`
}

type MySQLOptionsResponse struct {
	Success bool                                  `json:"success"`
	Message string                                `json:"message"`
	Global  *config.MySQLDumpOptionsConfig        `json:"global,omitempty"`
	PerServer map[string]*config.MySQLDumpOptionsConfig `json:"per_server,omitempty"`
}

func NewMySQLOptionsHandler(cfg *config.AppConfig, logger *logrus.Logger) *MySQLOptionsHandler {
	return &MySQLOptionsHandler{
		Config: cfg,
		Logger: logger,
	}
}

func (h *MySQLOptionsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/mysql-options", h.handleMySQLOptions)
	mux.HandleFunc("/api/mysql-options/server", h.handleServerMySQLOptions)
}

func (h *MySQLOptionsHandler) handleMySQLOptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getMySQLOptions(w, r)
	case http.MethodPut, http.MethodPost:
		h.updateMySQLOptions(w, r)
	default:
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *MySQLOptionsHandler) getMySQLOptions(w http.ResponseWriter, r *http.Request) {
	response := MySQLOptionsResponse{
		Success: true,
		Global:  &h.Config.MySQLDumpOptions,
		PerServer: make(map[string]*config.MySQLDumpOptionsConfig),
	}

	// Collect per-server MySQL options
	for _, server := range h.Config.DatabaseServers {
		if server.Type == "mysql" {
			response.PerServer[server.Name] = &server.MySQLDumpOptions
		}
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *MySQLOptionsHandler) updateMySQLOptions(w http.ResponseWriter, r *http.Request) {
	var req MySQLOptionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Update global options if provided
	if req.Global != nil {
		h.Config.MySQLDumpOptions = convertToConfigOptions(req.Global)
	}

	// Update per-server options if provided
	if req.PerServer != nil {
		for serverName, options := range req.PerServer {
			// Find the server in config
			for i, server := range h.Config.DatabaseServers {
				if server.Name == serverName && server.Type == "mysql" {
					h.Config.DatabaseServers[i].MySQLDumpOptions = convertToConfigOptions(options)
					break
				}
			}
		}
	}

	// Save configuration if using MySQL config
	if h.Config.MetadataDB.Enabled {
		if err := h.saveMySQLOptionsToDatabase(); err != nil {
			h.sendError(w, fmt.Sprintf("Failed to save MySQL options: %v", err), http.StatusInternalServerError)
			return
		}
	}

	response := MySQLOptionsResponse{
		Success: true,
		Message: "MySQL options updated successfully",
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *MySQLOptionsHandler) handleServerMySQLOptions(w http.ResponseWriter, r *http.Request) {
	serverName := r.URL.Query().Get("server")
	if serverName == "" {
		h.sendError(w, "Server name is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getServerMySQLOptions(w, serverName)
	case http.MethodPut, http.MethodPost:
		h.updateServerMySQLOptions(w, r, serverName)
	case http.MethodDelete:
		h.deleteServerMySQLOptions(w, serverName)
	default:
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *MySQLOptionsHandler) getServerMySQLOptions(w http.ResponseWriter, serverName string) {
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

	if foundServer.Type != "mysql" {
		h.sendError(w, "Server is not a MySQL server", http.StatusBadRequest)
		return
	}

	response := MySQLOptionsResponse{
		Success: true,
		PerServer: map[string]*config.MySQLDumpOptionsConfig{
			serverName: &foundServer.MySQLDumpOptions,
		},
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *MySQLOptionsHandler) updateServerMySQLOptions(w http.ResponseWriter, r *http.Request, serverName string) {
	var options database.MySQLDumpOptions
	if err := json.NewDecoder(r.Body).Decode(&options); err != nil {
		h.sendError(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Find and update the server
	found := false
	for i, server := range h.Config.DatabaseServers {
		if server.Name == serverName {
			if server.Type != "mysql" {
				h.sendError(w, "Server is not a MySQL server", http.StatusBadRequest)
				return
			}
			h.Config.DatabaseServers[i].MySQLDumpOptions = convertToConfigOptions(&options)
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
		if err := h.saveMySQLOptionsToDatabase(); err != nil {
			h.sendError(w, fmt.Sprintf("Failed to save MySQL options: %v", err), http.StatusInternalServerError)
			return
		}
	}

	configOptions := convertToConfigOptions(&options)
	response := MySQLOptionsResponse{
		Success: true,
		Message: fmt.Sprintf("MySQL options updated for server: %s", serverName),
		PerServer: map[string]*config.MySQLDumpOptionsConfig{
			serverName: &configOptions,
		},
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *MySQLOptionsHandler) deleteServerMySQLOptions(w http.ResponseWriter, serverName string) {
	// Find and clear the server's MySQL options
	found := false
	for i, server := range h.Config.DatabaseServers {
		if server.Name == serverName {
			if server.Type != "mysql" {
				h.sendError(w, "Server is not a MySQL server", http.StatusBadRequest)
				return
			}
			h.Config.DatabaseServers[i].MySQLDumpOptions = config.MySQLDumpOptionsConfig{}
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
		if err := h.saveMySQLOptionsToDatabase(); err != nil {
			h.sendError(w, fmt.Sprintf("Failed to save MySQL options: %v", err), http.StatusInternalServerError)
			return
		}
	}

	response := MySQLOptionsResponse{
		Success: true,
		Message: fmt.Sprintf("MySQL options removed for server: %s", serverName),
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *MySQLOptionsHandler) saveMySQLOptionsToDatabase() error {
	db := metadata.DB
	if db == nil {
		return fmt.Errorf("metadata database not initialized")
	}

	// Save global options
	// Convert config options to database options for saving
	globalDBOptions := database.MySQLDumpOptions{
		SingleTransaction:  h.Config.MySQLDumpOptions.SingleTransaction,
		Quick:              h.Config.MySQLDumpOptions.Quick,
		SkipLockTables:     h.Config.MySQLDumpOptions.SkipLockTables,
		SkipAddLocks:       h.Config.MySQLDumpOptions.SkipAddLocks,
		SkipComments:       h.Config.MySQLDumpOptions.SkipComments,
		ExtendedInsert:     h.Config.MySQLDumpOptions.ExtendedInsert,
		SkipExtendedInsert: h.Config.MySQLDumpOptions.SkipExtendedInsert,
		Compress:           h.Config.MySQLDumpOptions.Compress,
		CustomOptions:      h.Config.MySQLDumpOptions.CustomOptions,
	}
	globalJSON, err := json.Marshal(globalDBOptions)
	if err != nil {
		return fmt.Errorf("failed to marshal global MySQL options: %w", err)
	}

	query := `
		INSERT INTO mysql_options (id, name, options, updated_at)
		VALUES (1, 'global', ?, NOW())
		ON DUPLICATE KEY UPDATE
			options = VALUES(options),
			updated_at = NOW()
	`

	if err := db.Exec(query, string(globalJSON)).Error; err != nil {
		return fmt.Errorf("failed to save global MySQL options to database: %w", err)
	}

	// Save per-server options
	for _, server := range h.Config.DatabaseServers {
		if server.Type == "mysql" {
			// Convert config options to database options for saving
			dbOptions := database.MySQLDumpOptions{
				SingleTransaction:  server.MySQLDumpOptions.SingleTransaction,
				Quick:              server.MySQLDumpOptions.Quick,
				SkipLockTables:     server.MySQLDumpOptions.SkipLockTables,
				SkipAddLocks:       server.MySQLDumpOptions.SkipAddLocks,
				SkipComments:       server.MySQLDumpOptions.SkipComments,
				ExtendedInsert:     server.MySQLDumpOptions.ExtendedInsert,
				SkipExtendedInsert: server.MySQLDumpOptions.SkipExtendedInsert,
				Compress:           server.MySQLDumpOptions.Compress,
				CustomOptions:      server.MySQLDumpOptions.CustomOptions,
			}
			serverJSON, err := json.Marshal(dbOptions)
			if err != nil {
				if h.Logger != nil {
					h.Logger.Warnf("Failed to marshal MySQL options for server %s: %v", server.Name, err)
				}
				continue
			}

			query := `
				INSERT INTO mysql_options (server_name, name, options, updated_at)
				VALUES (?, 'server', ?, NOW())
				ON DUPLICATE KEY UPDATE
					options = VALUES(options),
					updated_at = NOW()
			`

			if err := db.Exec(query, server.Name, string(serverJSON)).Error; err != nil {
				if h.Logger != nil {
					h.Logger.Warnf("Failed to save MySQL options for server %s: %v", server.Name, err)
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

// convertToConfigOptions converts database.MySQLDumpOptions to config.MySQLDumpOptionsConfig
func convertToConfigOptions(opts *database.MySQLDumpOptions) config.MySQLDumpOptionsConfig {
	if opts == nil {
		return config.MySQLDumpOptionsConfig{}
	}
	return config.MySQLDumpOptionsConfig{
		SingleTransaction:  opts.SingleTransaction,
		Quick:              opts.Quick,
		SkipLockTables:     opts.SkipLockTables,
		SkipAddLocks:       opts.SkipAddLocks,
		SkipComments:       opts.SkipComments,
		ExtendedInsert:     opts.ExtendedInsert,
		SkipExtendedInsert: opts.SkipExtendedInsert,
		Compress:           opts.Compress,
		CustomOptions:      opts.CustomOptions,
	}
}

func (h *MySQLOptionsHandler) sendJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		if h.Logger != nil {
			h.Logger.Errorf("Failed to encode JSON response: %v", err)
		}
	}
}

func (h *MySQLOptionsHandler) sendError(w http.ResponseWriter, message string, status int) {
	response := MySQLOptionsResponse{
		Success: false,
		Message: message,
	}
	h.sendJSON(w, response, status)
}