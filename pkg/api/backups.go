package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/metadata"
)

// BackupsHandler handles backup-related API endpoints
type BackupsHandler struct{}

// NewBackupsHandler creates a new backups handler
func NewBackupsHandler() *BackupsHandler {
	return &BackupsHandler{}
}

// RegisterRoutes registers the backup API routes on the provided mux
func (h *BackupsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/backups", h.handleBackups)
	mux.HandleFunc("/api/backups/stats", h.handleBackupStats)
}

// handleBackups handles paginated backup queries
func (h *BackupsHandler) handleBackups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if we have a database store with pagination support
	dbStore, ok := metadata.DefaultStore.(*metadata.DBStore)
	if !ok {
		// Fall back to non-paginated response for file-based store
		h.handleBackupsLegacy(w, r)
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	
	opts := metadata.QueryOptions{
		// Filtering
		ServerName:   query.Get("server"),
		DatabaseName: query.Get("database"),
		BackupType:   query.Get("type"),
		Status:       query.Get("status"),
		SearchTerm:   query.Get("search"),
		ActiveOnly:   query.Get("activeOnly") == "true",
		
		// Pagination
		Page:     parseInt(query.Get("page"), 1),
		PageSize: parseInt(query.Get("pageSize"), 50),
		
		// Sorting
		SortBy:    query.Get("sortBy"),
		SortOrder: query.Get("sortOrder"),
		
		// Performance
		PreloadPaths: query.Get("includePaths") == "true",
	}

	// Parse date filters
	if startDate := query.Get("startDate"); startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			opts.StartDate = &t
		}
	}
	
	if endDate := query.Get("endDate"); endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			opts.EndDate = &t
		}
	}

	// Get paginated results
	result, err := dbStore.GetBackupsPaginated(opts)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to retrieve backups: %v", err), http.StatusInternalServerError)
		return
	}

	// Return paginated response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleBackupsLegacy handles non-paginated backup queries for file-based store
func (h *BackupsHandler) handleBackupsLegacy(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	
	// Get filter parameters
	serverName := query.Get("server")
	database := query.Get("database")
	backupType := query.Get("type")
	activeOnly := query.Get("activeOnly") == "true"

	// Get filtered backups
	var backups []metadata.BackupMeta
	if serverName != "" || database != "" || backupType != "" || activeOnly {
		backups = metadata.DefaultStore.GetBackupsFiltered(serverName, database, backupType, activeOnly)
	} else {
		backups = metadata.DefaultStore.GetBackups()
	}

	// Apply additional filters if needed
	filteredBackups := backups
	
	// Date range filter
	if startDate := query.Get("startDate"); startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			var filtered []metadata.BackupMeta
			for _, b := range filteredBackups {
				if b.CreatedAt.After(t) || b.CreatedAt.Equal(t) {
					filtered = append(filtered, b)
				}
			}
			filteredBackups = filtered
		}
	}
	
	if endDate := query.Get("endDate"); endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			// Add 1 day to include the entire end date
			endTime := t.AddDate(0, 0, 1)
			var filtered []metadata.BackupMeta
			for _, b := range filteredBackups {
				if b.CreatedAt.Before(endTime) {
					filtered = append(filtered, b)
				}
			}
			filteredBackups = filtered
		}
	}

	// Search filter
	if search := query.Get("search"); search != "" {
		var filtered []metadata.BackupMeta
		for _, b := range filteredBackups {
			if contains(b.ID, search) || contains(b.ServerName, search) || contains(b.Database, search) {
				filtered = append(filtered, b)
			}
		}
		filteredBackups = filtered
	}

	// Status filter
	if status := query.Get("status"); status != "" {
		var filtered []metadata.BackupMeta
		for _, b := range filteredBackups {
			if string(b.Status) == status {
				filtered = append(filtered, b)
			}
		}
		filteredBackups = filtered
	}

	// Create legacy response format
	response := map[string]interface{}{
		"data":       filteredBackups,
		"total":      len(filteredBackups),
		"page":       1,
		"pageSize":   len(filteredBackups),
		"totalPages": 1,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleBackupStats handles optimized stats queries
func (h *BackupsHandler) handleBackupStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if we have a database store with optimized stats
	dbStore, ok := metadata.DefaultStore.(*metadata.DBStore)
	if !ok {
		// Fall back to regular stats for file-based store
		stats := metadata.DefaultStore.GetStats()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
		return
	}

	// Get optimized stats
	stats, err := dbStore.GetStatsOptimized()
	if err != nil {
		// Fall back to regular stats on error
		stats = metadata.DefaultStore.GetStats()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// Helper functions

func parseInt(s string, defaultValue int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return defaultValue
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && 
		(s == substr || (len(s) >= len(substr) && s[:len(substr)] == substr) ||
		 (len(s) >= len(substr) && s[len(s)-len(substr):] == substr) ||
		 (len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}