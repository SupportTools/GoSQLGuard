package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/templates/pages"
	"github.com/supporttools/GoSQLGuard/templates/types"
)

// ServersHandler handles the servers page using Templ
func ServersHandler(w http.ResponseWriter, r *http.Request) {
	// Prepare servers data
	serversData := pages.ServersPageData{
		Servers:     config.CFG.DatabaseServers,
		BackupStats: make(map[string]pages.ServerBackupStats),
	}

	// Get backup statistics for each server
	if metadata.DefaultStore != nil {
		allBackups := metadata.DefaultStore.GetBackups()
		
		for _, server := range config.CFG.DatabaseServers {
			stats := pages.ServerBackupStats{
				TotalBackups: 0,
				Databases:    make([]string, 0),
			}
			
			databaseMap := make(map[string]bool)
			var lastBackupTime time.Time
			
			// Count backups and collect stats for this server
			for _, backup := range allBackups {
				if backup.ServerName == server.Name {
					stats.TotalBackups++
					stats.TotalSize += uint64(backup.Size)
					
					// Track unique databases
					if !databaseMap[backup.Database] {
						databaseMap[backup.Database] = true
						stats.Databases = append(stats.Databases, backup.Database)
					}
					
					// Track most recent backup
					if backup.CreatedAt.After(lastBackupTime) {
						lastBackupTime = backup.CreatedAt
					}
				}
			}
			
			// Format last backup time
			if !lastBackupTime.IsZero() {
				stats.LastBackup = lastBackupTime.Format("2006-01-02 15:04:05")
			}
			
			serversData.BackupStats[server.Name] = stats
		}
	}

	// Prepare page data
	pageData := types.PageData{
		Title:       "Servers",
		Description: "Database server management and statistics",
		AppName:     "GoSQLGuard",
		Version:     "1.0",
		Time:        time.Now().Format("2006-01-02 15:04:05"),
		NavLinks:    commonNavLinks,
	}

	// Mark active nav link
	for i := range pageData.NavLinks {
		if pageData.NavLinks[i].URL == "/servers" {
			pageData.NavLinks[i].Active = true
		}
	}

	// Render using Templ
	component := pages.ServersPage(pageData, serversData)
	component.Render(context.Background(), w)
}