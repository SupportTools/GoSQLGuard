// Package pages provides HTML pages for the admin UI
package pages

import (
	"net/http"
	"sort"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
)

// ServerPageData holds data for the servers page
type ServerPageData struct {
	Servers      []ServerInfo
	LastUpdated  time.Time
}

// ServerInfo holds information about a server
type ServerInfo struct {
	Name             string
	Type             string
	Host             string
	Port             string
	TotalBackups     int
	SuccessfulBackups int
	FailedBackups    int
	LastBackupTime   time.Time
	LastBackupStatus types.BackupStatus
	TotalSize        int64
	Databases        []string
}

// ServersPage renders the server management page
func ServersPage(w http.ResponseWriter, r *http.Request) {
	// Create a new template based on the common template
	tmpl := generateCommonTemplate()
	if tmpl == nil {
		http.Error(w, "Failed to generate template", http.StatusInternalServerError)
		return
	}

	// Add page-specific template
	contentTemplate := `
{{define "content"}}
<!-- Server Overview -->
<div class="row mb-4">
    <div class="col-12">
        <div class="card">
            <div class="card-header d-flex justify-content-between align-items-center">
                <span><i data-feather="server"></i> Configured Servers</span>
                <span class="text-muted small">Total: {{len .Content.Servers}} servers</span>
            </div>
            <div class="card-body">
                {{if .Content.Servers}}
                <div class="table-responsive">
                    <table class="table table-striped table-hover">
                        <thead>
                            <tr>
                                <th>Server Name</th>
                                <th>Type</th>
                                <th>Host:Port</th>
                                <th>Total Backups</th>
                                <th>Success Rate</th>
                                <th>Total Size</th>
                                <th>Last Backup</th>
                                <th>Status</th>
                                <th>Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {{range .Content.Servers}}
                            <tr>
                                <td>
                                    <strong>{{.Name}}</strong>
                                    {{if .Databases}}
                                    <br><small class="text-muted">{{len .Databases}} databases</small>
                                    {{end}}
                                </td>
                                <td>
                                    <span class="badge bg-secondary">{{.Type}}</span>
                                </td>
                                <td>{{.Host}}:{{.Port}}</td>
                                <td>{{.TotalBackups}}</td>
                                <td>
                                    {{if .TotalBackups}}
                                        {{$successRate := div (mul .SuccessfulBackups 100) .TotalBackups}}
                                        <div class="progress" style="height: 20px;">
                                            <div class="progress-bar {{if ge $successRate 90}}bg-success{{else if ge $successRate 70}}bg-warning{{else}}bg-danger{{end}}" 
                                                 role="progressbar" 
                                                 style="width: {{$successRate}}%"
                                                 aria-valuenow="{{$successRate}}" 
                                                 aria-valuemin="0" 
                                                 aria-valuemax="100">
                                                {{$successRate}}%
                                            </div>
                                        </div>
                                    {{else}}
                                        <span class="text-muted">-</span>
                                    {{end}}
                                </td>
                                <td>{{formatBytes .TotalSize}}</td>
                                <td>
                                    {{if not .LastBackupTime.IsZero}}
                                        {{formatTime .LastBackupTime}}
                                    {{else}}
                                        <span class="text-muted">Never</span>
                                    {{end}}
                                </td>
                                <td>
                                    {{if not .LastBackupTime.IsZero}}
                                        <span class="badge {{if eq .LastBackupStatus "success"}}bg-success{{else if eq .LastBackupStatus "error"}}bg-danger{{else if eq .LastBackupStatus "pending"}}bg-info{{else}}bg-warning{{end}}">
                                            {{.LastBackupStatus}}
                                        </span>
                                    {{else}}
                                        <span class="text-muted">-</span>
                                    {{end}}
                                </td>
                                <td>
                                    <a href="/status/backups?server={{.Name}}" class="btn btn-sm btn-outline-primary" title="View Backups">
                                        <i data-feather="list"></i>
                                    </a>
                                    <button class="btn btn-sm btn-outline-info test-connection" data-server="{{.Name}}" title="Test Connection">
                                        <i data-feather="check-circle"></i>
                                    </button>
                                </td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                </div>
                {{else}}
                <p class="text-muted">No servers configured.</p>
                {{end}}
            </div>
        </div>
    </div>
</div>

<!-- Server Details Cards -->
{{if .Content.Servers}}
<div class="row">
    {{range .Content.Servers}}
    <div class="col-md-6 mb-4">
        <div class="card">
            <div class="card-header">
                <h5 class="mb-0">{{.Name}}</h5>
            </div>
            <div class="card-body">
                <div class="row">
                    <div class="col-6">
                        <p class="mb-1"><strong>Type:</strong> {{.Type}}</p>
                        <p class="mb-1"><strong>Host:</strong> {{.Host}}</p>
                        <p class="mb-1"><strong>Port:</strong> {{.Port}}</p>
                    </div>
                    <div class="col-6">
                        <p class="mb-1"><strong>Total Backups:</strong> {{.TotalBackups}}</p>
                        <p class="mb-1"><strong>Successful:</strong> {{.SuccessfulBackups}}</p>
                        <p class="mb-1"><strong>Failed:</strong> {{.FailedBackups}}</p>
                    </div>
                </div>
                {{if .Databases}}
                <hr>
                <p class="mb-1"><strong>Databases:</strong></p>
                <div class="d-flex flex-wrap gap-1">
                    {{range .Databases}}
                    <span class="badge bg-secondary">{{.}}</span>
                    {{end}}
                </div>
                {{end}}
            </div>
        </div>
    </div>
    {{end}}
</div>
{{end}}

<script>
document.addEventListener('DOMContentLoaded', function() {
    // Initialize feather icons
    feather.replace();
    
    // Test connection buttons
    document.querySelectorAll('.test-connection').forEach(function(button) {
        button.addEventListener('click', function() {
            var serverName = this.dataset.server;
            var button = this;
            
            // Disable button and show loading
            button.disabled = true;
            button.innerHTML = '<span class="spinner-border spinner-border-sm" role="status"></span>';
            
            // In a real implementation, this would call an API endpoint to test the connection
            setTimeout(function() {
                button.disabled = false;
                button.innerHTML = '<i data-feather="check-circle"></i>';
                feather.replace();
                
                // Show result (this would be based on actual API response)
                alert('Connection test for ' + serverName + ' would be implemented here');
            }, 1000);
        });
    });
});
</script>
{{end}}
`
	var err error
	tmpl, err = tmpl.Parse(contentTemplate)
	if err != nil {
		http.Error(w, "Template parsing error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get data for the page
	var data ServerPageData
	data.Servers = make([]ServerInfo, 0)

	// Get configured servers
	for _, server := range config.CFG.DatabaseServers {
		serverInfo := ServerInfo{
			Name: server.Name,
			Type: server.Type,
			Host: server.Host,
			Port: server.Port,
		}

		// Get backup statistics for this server
		if metadata.DefaultStore != nil {
			backups := metadata.DefaultStore.GetBackupsFiltered(server.Name, "", "", false)
			
			// Collect database names
			dbMap := make(map[string]bool)
			var lastBackup *types.BackupMeta
			
			for _, backup := range backups {
				serverInfo.TotalBackups++
				
				if backup.Status == types.StatusSuccess {
					serverInfo.SuccessfulBackups++
					serverInfo.TotalSize += backup.Size
				} else if backup.Status == types.StatusError {
					serverInfo.FailedBackups++
				}
				
				// Track databases
				if backup.Database != "" {
					dbMap[backup.Database] = true
				}
				
				// Find the most recent backup
				if lastBackup == nil || backup.CreatedAt.After(lastBackup.CreatedAt) {
					lastBackup = &backup
				}
			}
			
			// Convert database map to slice
			for db := range dbMap {
				serverInfo.Databases = append(serverInfo.Databases, db)
			}
			sort.Strings(serverInfo.Databases)
			
			// Set last backup info
			if lastBackup != nil {
				serverInfo.LastBackupTime = lastBackup.CreatedAt
				serverInfo.LastBackupStatus = lastBackup.Status
			}
		}

		data.Servers = append(data.Servers, serverInfo)
	}

	// If no configured servers, add servers found in metadata
	if len(data.Servers) == 0 && metadata.DefaultStore != nil {
		serverMap := make(map[string]*ServerInfo)
		
		backups := metadata.DefaultStore.GetBackups()
		for _, backup := range backups {
			if backup.ServerName == "" {
				continue
			}
			
			if _, exists := serverMap[backup.ServerName]; !exists {
				serverMap[backup.ServerName] = &ServerInfo{
					Name: backup.ServerName,
					Type: backup.ServerType,
					Host: "Unknown",
					Port: "Unknown",
				}
			}
			
			server := serverMap[backup.ServerName]
			server.TotalBackups++
			
			if backup.Status == types.StatusSuccess {
				server.SuccessfulBackups++
				server.TotalSize += backup.Size
			} else if backup.Status == types.StatusError {
				server.FailedBackups++
			}
			
			// Update last backup info
			if server.LastBackupTime.IsZero() || backup.CreatedAt.After(server.LastBackupTime) {
				server.LastBackupTime = backup.CreatedAt
				server.LastBackupStatus = backup.Status
			}
		}
		
		// Convert map to slice
		for _, server := range serverMap {
			data.Servers = append(data.Servers, *server)
		}
		
		// Sort by name
		sort.Slice(data.Servers, func(i, j int) bool {
			return data.Servers[i].Name < data.Servers[j].Name
		})
	}

	data.LastUpdated = time.Now()

	// Render the template
	renderTemplate(w, tmpl, "/servers", PageData{
		Title:       "Server Management",
		Description: "Configure and monitor database servers",
		Content:     data,
	})
}