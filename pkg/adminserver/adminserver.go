// Package adminserver provides an HTTP server for administering GoSQLGuard.
package adminserver

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/supporttools/GoSQLGuard/pkg/api"
	"github.com/supporttools/GoSQLGuard/pkg/backup"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/database"
	"github.com/supporttools/GoSQLGuard/pkg/handlers"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/pages"
	"github.com/supporttools/GoSQLGuard/pkg/scheduler"
	"github.com/supporttools/GoSQLGuard/pkg/storage/s3"
)

var (
	taskLock      sync.Mutex
	isTaskRunning bool
)

// Server represents the admin HTTP server
type Server struct {
	httpServer *http.Server
	scheduler  *scheduler.Scheduler
	backupMgr  *backup.Manager
}

// NewServer creates a new admin server instance
func NewServer(backupMgr *backup.Manager, sched *scheduler.Scheduler) *Server {
	return &Server{
		scheduler: sched,
		backupMgr: backupMgr,
	}
}

// Start starts the admin HTTP server
func (s *Server) Start() *http.Server {
	mux := http.NewServeMux()

	// Register routes
	s.registerRoutes(mux)

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%s", config.CFG.Metrics.Port),
		Handler:      logRequestMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Start HTTP server in a goroutine
	go func() {
		log.Printf("Admin server running on port %s", config.CFG.Metrics.Port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	return s.httpServer
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop() error {
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

// registerRoutes registers all HTTP routes
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Static pages - Use new Templ-based handler for dashboard
	mux.HandleFunc("/", handlers.DashboardHandler)
	// Keep existing handlers for now, will migrate incrementally
	mux.HandleFunc("/status/backups", pages.BackupStatusPage)
	mux.HandleFunc("/status/storage", pages.StorageStatusPage)
	mux.HandleFunc("/databases", pages.DatabasesPage)
	mux.HandleFunc("/s3download", pages.S3DownloadPage)
	mux.HandleFunc("/servers", handlers.ServersHandler)             // Servers management page
	mux.HandleFunc("/mysql-options", pages.MySQLOptionsPage)        // MySQL dump options configuration
	mux.HandleFunc("/configuration", handlers.ConfigurationHandler) // Configuration management page

	// Standard endpoints
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", s.healthCheckHandler)
	mux.HandleFunc("/api/stats", s.statsHandler)

	// Backup operations
	mux.HandleFunc("/api/backups", s.listBackupsHandler)
	mux.HandleFunc("/api/backups/run", s.runBackupHandler)
	mux.HandleFunc("/api/backups/delete", s.deleteBackupHandler)
	mux.HandleFunc("/api/backups/log", s.serveLogFileHandler)
	mux.HandleFunc("/api/backups/download/local", s.downloadLocalBackupHandler)
	mux.HandleFunc("/api/backups/download/s3", s.downloadS3BackupHandler)

	// Storage operations
	mux.HandleFunc("/api/storage", s.storageInfoHandler)
	mux.HandleFunc("/api/retention/run", s.runRetentionHandler)

	// HTMX endpoints
	mux.HandleFunc("/api/dashboard/recent-backups", handlers.RecentBackupsHandler)

	// MySQL options operations
	mux.HandleFunc("/api/mysql-options/global", s.mysqlOptionsHandler)

	// Server and schedule management API
	serverHandler := api.NewServerHandler()
	scheduleHandler := api.NewScheduleHandler(s.scheduler)
	serverHandler.RegisterRoutes(mux)
	scheduleHandler.RegisterRoutes(mux)

	// Configuration management API
	// TODO: Implement config handler
	// configHandler, err := api.NewConfigHandler()
	// if err != nil {
	// 	log.Printf("Warning: Failed to initialize config API: %v", err)
	// } else {
	// 	configHandler.RegisterRoutes(mux)
	// }

	// S3 configuration API
	logger := logrus.New()
	if config.CFG.Debug {
		logger.SetLevel(logrus.DebugLevel)
	}
	s3Handler := api.NewS3ConfigHandler(&config.CFG, logger)
	s3Handler.RegisterRoutes(mux)

	// MySQL options configuration API
	mysqlOptionsHandler := api.NewMySQLOptionsHandler(&config.CFG, nil)
	mysqlOptionsHandler.RegisterRoutes(mux)

	// PostgreSQL options configuration API
	postgresqlOptionsHandler := api.NewPostgreSQLOptionsHandler(&config.CFG, nil)
	postgresqlOptionsHandler.RegisterRoutes(mux)
}

// healthCheckHandler returns a simple health status
func (s *Server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
	if err != nil {
		log.Printf("Error encoding health check response: %v", err)
	}
}

// statsHandler returns statistics about backups
func (s *Server) statsHandler(w http.ResponseWriter, r *http.Request) {
	// Get the active metadata store
	metadataStore := metadata.GetActiveStore()
	if metadataStore == nil {
		log.Printf("Metadata store not available")
		http.Error(w, "Metadata store not available", http.StatusServiceUnavailable)
		return
	}

	stats := metadataStore.GetStats()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Printf("Error encoding stats response: %v", err)
		http.Error(w, "Error generating stats", http.StatusInternalServerError)
		return
	}
}

// listBackupsHandler returns a list of backups with optional filtering
func (s *Server) listBackupsHandler(w http.ResponseWriter, r *http.Request) {
	database := r.URL.Query().Get("database")
	backupType := r.URL.Query().Get("type")
	activeOnly := r.URL.Query().Get("activeOnly") == "true"

	// Get the active metadata store
	metadataStore := metadata.GetActiveStore()
	if metadataStore == nil {
		log.Printf("Metadata store not available")
		http.Error(w, "Metadata store not available", http.StatusServiceUnavailable)
		return
	}

	backups := metadataStore.GetBackupsFiltered("", database, backupType, activeOnly)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"backups": backups,
		"count":   len(backups),
	}); err != nil {
		log.Printf("Error encoding backups response: %v", err)
		http.Error(w, "Error listing backups", http.StatusInternalServerError)
		return
	}
}

// runBackupHandler triggers a manual backup
func (s *Server) runBackupHandler(w http.ResponseWriter, r *http.Request) {
	// This should be a POST request
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters
	backupType := r.URL.Query().Get("type")
	databaseParam := r.URL.Query().Get("database")
	serverParam := r.URL.Query().Get("server")

	// Validate parameters
	if backupType == "" {
		http.Error(w, "Missing required parameter: type", http.StatusBadRequest)
		return
	}

	// Parse server list (comma-separated)
	var servers []string
	if serverParam != "" {
		servers = strings.Split(serverParam, ",")
	}

	// Parse database list (comma-separated)
	var databases []string
	if databaseParam != "" {
		databases = strings.Split(databaseParam, ",")
	}

	// Check if the backup type exists
	if _, exists := config.CFG.BackupTypes[backupType]; !exists {
		http.Error(w, fmt.Sprintf("Invalid backup type: %s", backupType), http.StatusBadRequest)
		return
	}

	// Validate servers if specified
	if len(servers) > 0 {
		for _, serverName := range servers {
			// Check if each server exists in configuration
			serverExists := false
			for _, server := range config.CFG.DatabaseServers {
				if server.Name == serverName {
					serverExists = true
					break
				}
			}

			if !serverExists {
				http.Error(w, fmt.Sprintf("Server not found: %s", serverName), http.StatusBadRequest)
				return
			}
		}
	}

	// Check if scheduler is available
	if s.scheduler == nil {
		http.Error(w, "Scheduler not configured", http.StatusInternalServerError)
		return
	}

	// Check if a task is already running
	if !triggerBackup(s, backupType, servers, databases) {
		http.Error(w, "A backup task is already running", http.StatusConflict)
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	response := map[string]string{
		"status":  "accepted",
		"message": fmt.Sprintf("Backup of type %s initiated", backupType),
	}

	if len(databases) > 0 {
		response["message"] = fmt.Sprintf("Backup of databases %s (type: %s) initiated",
			strings.Join(databases, ", "), backupType)
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// deleteBackupHandler marks a backup as deleted
func (s *Server) deleteBackupHandler(w http.ResponseWriter, r *http.Request) {
	// This should be a POST request
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get backup ID
	backupID := r.URL.Query().Get("id")
	if backupID == "" {
		http.Error(w, "Missing required parameter: id", http.StatusBadRequest)
		return
	}

	// Get the active metadata store
	metadataStore := metadata.GetActiveStore()
	if metadataStore == nil {
		log.Printf("Metadata store not available")
		http.Error(w, "Metadata store not available", http.StatusServiceUnavailable)
		return
	}

	// Check if the backup exists
	backup, exists := metadataStore.GetBackupByID(backupID)
	if !exists {
		http.Error(w, fmt.Sprintf("Backup with ID %s not found", backupID), http.StatusNotFound)
		return
	}

	// Mark as deleted in metadata
	if err := metadataStore.MarkBackupDeleted(backupID); err != nil {
		log.Printf("Error marking backup as deleted: %v", err)
		http.Error(w, "Error deleting backup", http.StatusInternalServerError)
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":   "success",
		"message":  fmt.Sprintf("Backup %s marked as deleted", backupID),
		"id":       backupID,
		"database": backup.Database,
		"type":     backup.BackupType,
	}); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// storageInfoHandler returns information about storage destinations
func (s *Server) storageInfoHandler(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"local": map[string]interface{}{
			"enabled": config.CFG.Local.Enabled,
			"path":    config.CFG.Local.BackupDirectory,
		},
		"s3": map[string]interface{}{
			"enabled": config.CFG.S3.Enabled,
			"bucket":  config.CFG.S3.Bucket,
			"region":  config.CFG.S3.Region,
			"prefix":  config.CFG.S3.Prefix,
		},
	}

	// Get the active metadata store
	metadataStore := metadata.GetActiveStore()
	if metadataStore == nil {
		log.Printf("Metadata store not available")
		http.Error(w, "Metadata store not available", http.StatusServiceUnavailable)
		return
	}

	// Add stats
	stats := metadataStore.GetStats()
	info["stats"] = stats

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		log.Printf("Error encoding storage info response: %v", err)
		http.Error(w, "Error generating storage info", http.StatusInternalServerError)
		return
	}
}

// serveLogFileHandler serves the log file for a backup
func (s *Server) serveLogFileHandler(w http.ResponseWriter, r *http.Request) {
	// Get backup ID from request
	backupID := r.URL.Query().Get("id")
	if backupID == "" {
		http.Error(w, "Missing required parameter: id", http.StatusBadRequest)
		return
	}

	// Get the active metadata store
	metadataStore := metadata.GetActiveStore()
	if metadataStore == nil {
		http.Error(w, "Metadata store not available", http.StatusServiceUnavailable)
		return
	}

	// Check if the backup exists
	backup, exists := metadataStore.GetBackupByID(backupID)
	if !exists {
		http.Error(w, fmt.Sprintf("Backup with ID %s not found", backupID), http.StatusNotFound)
		return
	}

	// Check if the log file exists
	if backup.LogFilePath == "" {
		http.Error(w, fmt.Sprintf("No log file available for backup %s", backupID), http.StatusNotFound)
		return
	}

	// Check if the file exists on disk
	if _, err := os.Stat(backup.LogFilePath); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("Log file not found on disk: %s", backup.LogFilePath), http.StatusNotFound)
		return
	}

	// Read the log file
	logContent, err := os.ReadFile(backup.LogFilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading log file: %v", err), http.StatusInternalServerError)
		return
	}

	// Create a simple HTML page to display the log
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Backup Log - {{.BackupID}}</title>
    <style>
        body {
            font-family: monospace;
            background-color: #1e1e1e;
            color: #d4d4d4;
            padding: 20px;
            margin: 0;
        }
        pre {
            white-space: pre-wrap;
            word-wrap: break-word;
            margin: 0;
            padding: 10px;
            background-color: #252526;
            border-radius: 5px;
        }
        .header {
            margin-bottom: 20px;
            padding: 10px;
            background-color: #2d2d30;
            border-radius: 5px;
        }
        h1 {
            margin: 0;
            font-size: 18px;
            color: #cccccc;
        }
        .info {
            color: #9cdcfe;
            margin-top: 5px;
            font-size: 14px;
        }
        a {
            color: #007acc;
            text-decoration: none;
        }
        a:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>Backup Log</h1>
        <div class="info">
            Backup ID: {{.BackupID}}<br>
            Server: {{.ServerName}}<br>
            Database: {{.Database}}<br>
            Type: {{.BackupType}}<br>
            Status: {{.Status}}<br>
            <a href="/status/backups">← Back to Backup List</a>
        </div>
    </div>
    <pre>{{.LogContent}}</pre>
</body>
</html>`

	// Parse and execute the template
	t, err := template.New("log").Parse(tmpl)
	if err != nil {
		http.Error(w, "Error rendering log page", http.StatusInternalServerError)
		return
	}

	data := struct {
		BackupID   string
		ServerName string
		Database   string
		BackupType string
		Status     string
		LogContent string
	}{
		BackupID:   backupID,
		ServerName: backup.ServerName,
		Database:   backup.Database,
		BackupType: backup.BackupType,
		Status:     string(backup.Status),
		LogContent: string(logContent),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "Error rendering log page", http.StatusInternalServerError)
		return
	}
}

// downloadLocalBackupHandler serves a backup file from local storage for download
func (s *Server) downloadLocalBackupHandler(w http.ResponseWriter, r *http.Request) {
	// Get backup ID from request
	backupID := r.URL.Query().Get("id")
	if backupID == "" {
		http.Error(w, "Missing required parameter: id", http.StatusBadRequest)
		return
	}

	// Get the active metadata store
	metadataStore := metadata.GetActiveStore()
	if metadataStore == nil {
		log.Printf("Metadata store not available")
		http.Error(w, "Metadata store not available", http.StatusServiceUnavailable)
		return
	}

	// Check if the backup exists
	backup, exists := metadataStore.GetBackupByID(backupID)
	if !exists {
		http.Error(w, fmt.Sprintf("Backup with ID %s not found", backupID), http.StatusNotFound)
		return
	}

	// Check if local file path is available
	if backup.LocalPath == "" {
		http.Error(w, fmt.Sprintf("No local file available for backup %s", backupID), http.StatusNotFound)
		return
	}

	// Check if the file exists on disk
	if _, err := os.Stat(backup.LocalPath); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("Backup file not found on disk: %s", backup.LocalPath), http.StatusNotFound)
		return
	}

	// Get filename part for the download
	filename := fmt.Sprintf("%s-%s-%s.sql.gz", backup.Database, backup.BackupType, backup.CreatedAt.Format("2006-01-02-15-04-05"))

	// Set appropriate headers for file download
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Serve the file
	http.ServeFile(w, r, backup.LocalPath)

	log.Printf("Served local backup download: %s", backup.LocalPath)
}

// downloadS3BackupHandler handles downloading a backup from S3 using presigned URLs
func (s *Server) downloadS3BackupHandler(w http.ResponseWriter, r *http.Request) {
	// Get backup ID from request
	backupID := r.URL.Query().Get("id")
	if backupID == "" {
		http.Error(w, "Missing required parameter: id", http.StatusBadRequest)
		return
	}

	// Get the active metadata store
	metadataStore := metadata.GetActiveStore()
	if metadataStore == nil {
		log.Printf("Metadata store not available")
		http.Error(w, "Metadata store not available", http.StatusServiceUnavailable)
		return
	}

	// Check if the backup exists
	backup, exists := metadataStore.GetBackupByID(backupID)
	if !exists {
		http.Error(w, fmt.Sprintf("Backup with ID %s not found", backupID), http.StatusNotFound)
		return
	}

	// Check if S3 key is available
	if backup.S3Key == "" {
		http.Error(w, fmt.Sprintf("No S3 file available for backup %s", backupID), http.StatusNotFound)
		return
	}

	// Initialize S3 client
	s3Client, err := s3.NewClient()
	if err != nil {
		log.Printf("Error initializing S3 client: %v", err)
		http.Error(w, "Failed to connect to S3 storage", http.StatusInternalServerError)
		return
	}

	// Generate presigned URL with 15-minute expiration
	presignedURL, err := s3Client.GeneratePresignedURL(backup.S3Key, 15*time.Minute)
	if err != nil {
		log.Printf("Error generating presigned URL: %v", err)
		http.Error(w, "Failed to generate download link", http.StatusInternalServerError)
		return
	}

	// Get filename for the download
	filename := fmt.Sprintf("%s-%s-%s.sql.gz", backup.Database, backup.BackupType, backup.CreatedAt.Format("2006-01-02-15-04-05"))

	// Check if this is a direct download request
	if r.URL.Query().Get("redirect") == "true" {
		// Redirect directly to the presigned URL
		http.Redirect(w, r, presignedURL, http.StatusFound)
		log.Printf("Redirected to S3 presigned URL for backup: %s", backupID)
		return
	}

	// Otherwise return JSON with the URL and backup details
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status":       "success",
		"message":      "S3 presigned URL generated successfully",
		"id":           backupID,
		"database":     backup.Database,
		"type":         backup.BackupType,
		"size":         backup.Size,
		"created_at":   backup.CreatedAt,
		"s3_bucket":    config.CFG.S3.Bucket,
		"s3_key":       backup.S3Key,
		"download_url": presignedURL,
		"expires_in":   "15 minutes",
		"filename":     filename,
		"content_type": "application/gzip",
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding S3 download response: %v", err)
	}

	log.Printf("Generated presigned URL for S3 backup: %s", backupID)
}

// runRetentionHandler triggers retention policy enforcement
func (s *Server) runRetentionHandler(w http.ResponseWriter, r *http.Request) {
	// This should be a POST request
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if scheduler is available
	if s.scheduler == nil {
		http.Error(w, "Scheduler not configured", http.StatusInternalServerError)
		return
	}

	// Check if a task is already running
	if !triggerRetention(s) {
		http.Error(w, "A task is already running", http.StatusConflict)
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "accepted",
		"message": "Retention policy enforcement initiated",
	}); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

// mysqlOptionsHandler handles MySQL dump options configuration
func (s *Server) mysqlOptionsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Return current MySQL options configuration
		response := map[string]interface{}{
			"globalOptions": config.CFG.MySQLDumpOptions,
			"success":       true,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Error encoding MySQL options response: %v", err)
			http.Error(w, "Error generating response", http.StatusInternalServerError)
		}

	case http.MethodPost:
		// Update MySQL options configuration
		var options database.MySQLDumpOptions
		if err := json.NewDecoder(r.Body).Decode(&options); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// For now, just log the request as the configuration isn't properly integrated
		log.Printf("MySQL options update requested: %+v", options)

		// TODO: Save options to configuration and persist
		// This would require updating the config package to support saving

		response := map[string]interface{}{
			"success": true,
			"message": "MySQL options configuration is not yet fully implemented. Options logged for review.",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Error encoding response: %v", err)
			http.Error(w, "Error generating response", http.StatusInternalServerError)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// logRequestMiddleware logs HTTP requests
func logRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("HTTP %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// triggerBackup ensures only one backup runs at a time
func triggerBackup(s *Server, backupType string, servers []string, databases []string) bool {
	taskLock.Lock()
	defer taskLock.Unlock()

	if isTaskRunning {
		return false
	}

	isTaskRunning = true

	go func() {
		defer func() {
			taskLock.Lock()
			isTaskRunning = false
			taskLock.Unlock()
		}()

		// Log backup information
		if len(servers) > 0 {
			log.Printf("Running manual backup of type %s for servers: %v", backupType, servers)
		} else {
			log.Printf("Running manual backup of type %s for all servers", backupType)
		}

		if len(databases) > 0 {
			log.Printf("Only backing up databases: %v", databases)
		}

		// Run the backup with server and database filters
		err := s.scheduler.RunOnce(backupType, servers, databases)
		if err != nil {
			log.Printf("Error running backup: %v", err)
		}
	}()

	return true
}

// triggerRetention ensures only one retention task runs at a time
func triggerRetention(s *Server) bool {
	taskLock.Lock()
	defer taskLock.Unlock()

	if isTaskRunning {
		return false
	}

	isTaskRunning = true

	go func() {
		defer func() {
			taskLock.Lock()
			isTaskRunning = false
			taskLock.Unlock()
		}()

		log.Println("Running manual retention policy enforcement")
		s.scheduler.RunRetentionOnce()
	}()

	return true
}
