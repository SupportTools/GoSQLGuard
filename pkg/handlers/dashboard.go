package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/templates/pages"
	"github.com/supporttools/GoSQLGuard/templates/types"
)

// Common navigation links used across pages
var commonNavLinks = []types.NavLink{
	{URL: "/", Name: "Dashboard", Icon: "home"},
	{URL: "/databases", Name: "Database Browser", Icon: "database"},
	{URL: "/status/backups", Name: "Backup Status", Icon: "list"},
	{URL: "/status/storage", Name: "Storage", Icon: "hard-drive"},
	{URL: "/servers", Name: "Servers", Icon: "server"},
	{URL: "/configuration", Name: "Configuration", Icon: "settings"},
	{URL: "/mysql-options", Name: "MySQL Options", Icon: "tool"},
	{URL: "/metrics", Name: "Metrics", Icon: "bar-chart-2", External: true},
}

// DashboardHandler handles the main dashboard page using Templ
func DashboardHandler(w http.ResponseWriter, r *http.Request) {
	// Get dashboard data
	dashboardData := getDashboardData()

	// Prepare page data
	pageData := types.PageData{
		Title:       "Dashboard",
		Description: "Current status and summary of database backups",
		AppName:     "GoSQLGuard",
		Version:     "1.0",
		Time:        time.Now().Format("2006-01-02 15:04:05"),
		NavLinks:    commonNavLinks,
	}

	// Mark active nav link
	for i := range pageData.NavLinks {
		if pageData.NavLinks[i].URL == "/" {
			pageData.NavLinks[i].Active = true
		}
	}

	// Render using Templ
	component := pages.DashboardPage(pageData, dashboardData)
	component.Render(context.Background(), w)
}

// RecentBackupsHandler returns recent backups as HTML fragment for HTMX
func RecentBackupsHandler(w http.ResponseWriter, r *http.Request) {
	dashboardData := getDashboardData()

	// Render just the recent backups table
	component := pages.RenderRecentBackups(dashboardData.RecentBackups)
	component.Render(context.Background(), w)
}

// getDashboardData retrieves data for the dashboard
func getDashboardData() pages.DashboardPageData {
	dashboardData := pages.DashboardPageData{
		Stats:        make(map[string]interface{}),
		BackupTypes:  config.CFG.BackupTypes,
		LocalEnabled: config.CFG.Local.Enabled,
		S3Enabled:    config.CFG.S3.Enabled,
		LastUpdated:  time.Now(),
	}

	// Initialize default stats
	dashboardData.Stats["totalCount"] = 0
	dashboardData.Stats["totalLocalSize"] = int64(0)
	dashboardData.Stats["totalS3Size"] = int64(0)
	dashboardData.Stats["statusCounts"] = map[string]int{
		"success": 0,
		"pending": 0,
		"error":   0,
		"deleted": 0,
	}

	if metadata.DefaultStore != nil {
		stats := metadata.DefaultStore.GetStats()
		if stats != nil {
			dashboardData.Stats = stats
		}

		// Get recent backups (last 5)
		allBackups := metadata.DefaultStore.GetBackups()
		if len(allBackups) > 5 {
			dashboardData.RecentBackups = allBackups[len(allBackups)-5:]
		} else {
			dashboardData.RecentBackups = allBackups
		}
	}

	// Get databases
	if len(config.CFG.MySQL.IncludeDatabases) > 0 {
		dashboardData.Databases = config.CFG.MySQL.IncludeDatabases
	} else {
		dashboardData.Databases = []string{}
	}

	return dashboardData
}

// RunBackupAPIHandler handles backup run requests via API
func RunBackupAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	backupType := r.URL.Query().Get("type")
	if backupType == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Missing backup type parameter",
		})
		return
	}

	// TODO: Integrate with backup manager to actually run the backup
	// For now, just return a success message
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Backup " + backupType + " started successfully",
	})
}

// RunRetentionAPIHandler handles retention policy enforcement requests
func RunRetentionAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Integrate with retention manager to actually run retention
	// For now, just return a success message
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Retention policy enforcement started",
	})
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
