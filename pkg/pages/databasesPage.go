// Package pages provides HTML pages for the admin UI.
package pages

import (
	"log"
	"net/http"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/backup/database/mysql"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
)

// DatabasesPageData holds data for the databases browsing page
type DatabasesPageData struct {
	Servers         []string            // List of server names
	ServerDatabases map[string][]string // Map of server name to list of databases
	SelectedServer  string              // Currently selected server
	SelectedDB      string              // Currently selected database
	DBBackups       []types.BackupMeta  // Backups for the selected database
	BackupTypes     map[string]config.BackupTypeConfig
	LocalEnabled    bool
	S3Enabled       bool
	LastUpdated     time.Time
}

// DatabasesPage renders the databases browsing page
func DatabasesPage(w http.ResponseWriter, r *http.Request) {
	// Create a new template based on the common template
	tmpl := generateCommonTemplate()
	if tmpl == nil {
		http.Error(w, "Failed to generate template", http.StatusInternalServerError)
		return
	}

	// Add page-specific template
	contentTemplate := `
{{define "content"}}
<div class="row">
    <!-- Database List Panel -->
    <div class="col-md-3">
        <!-- Server Selection Dropdown -->
        <div class="card mb-3">
            <div class="card-header">
                <h5 class="mb-0">Server</h5>
            </div>
            <div class="card-body">
                <select class="form-select" id="serverSelect" onchange="changeServer(this.value)">
                    <option value="" {{if eq .Content.SelectedServer ""}}selected{{end}}>All Servers</option>
                    {{range $server := .Content.Servers}}
                    <option value="{{$server}}" {{if eq $.Content.SelectedServer $server}}selected{{end}}>{{$server}}</option>
                    {{end}}
                </select>
            </div>
        </div>
        
        <!-- Database List -->
        <div class="card">
            <div class="card-header">
                <h5 class="mb-0">Databases</h5>
            </div>
            <div class="card-body p-0">
                <div class="list-group list-group-flush">
                    {{$selectedServer := .Content.SelectedServer}}
                    {{if $selectedServer}}
                        <!-- Show databases for selected server -->
                        {{range $db := index .Content.ServerDatabases $selectedServer}}
                        <a href="/databases?server={{$selectedServer}}&db={{$db}}" class="list-group-item list-group-item-action {{if eq $.Content.SelectedDB $db}}active{{end}}">
                            <i data-feather="database" class="me-2"></i> {{$db}}
                        </a>
                        {{else}}
                        <div class="list-group-item text-muted">No databases found on this server</div>
                        {{end}}
                    {{else}}
                        <!-- Show databases from all servers, grouped by server -->
                        {{range $server, $databases := .Content.ServerDatabases}}
                        <div class="list-group-item list-group-item-secondary">{{$server}}</div>
                        {{range $db := $databases}}
                        <a href="/databases?server={{$server}}&db={{$db}}" class="list-group-item list-group-item-action {{if and (eq $.Content.SelectedServer $server) (eq $.Content.SelectedDB $db)}}active{{end}}">
                            <i data-feather="database" class="me-2"></i> {{$db}}
                        </a>
                        {{else}}
                        <div class="list-group-item text-muted ps-4">No databases found</div>
                        {{end}}
                        {{else}}
                        <div class="list-group-item text-muted">No servers configured</div>
                        {{end}}
                    {{end}}
                </div>
            </div>
        </div>
    </div>

    <!-- Database Backups Panel -->
    <div class="col-md-9">
        {{if .Content.SelectedDB}}
        <div class="card">
            <div class="card-header d-flex justify-content-between align-items-center">
                <h5 class="mb-0">Backups for {{.Content.SelectedDB}}</h5>
                <button class="btn btn-sm btn-primary" id="runBackupForDb">
                    <i data-feather="play"></i> Run Backup
                </button>
            </div>
            <div class="card-body">
                {{if .Content.DBBackups}}
                <div class="table-responsive">
                    <table class="table table-striped table-hover">
                        <thead>
                            <tr>
                                <th>Type</th>
                                <th>Created</th>
                                <th>Completed</th>
                                <th>Size</th>
                                <th>Status</th>
                                <th>Retention</th>
                                <th>Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {{range .Content.DBBackups}}
                            <tr>
                                <td>{{.BackupType}}</td>
                                <td>{{formatTime .CreatedAt}}</td>
                                <td>{{if not .CompletedAt.IsZero}}{{formatTime .CompletedAt}}{{else}}-{{end}}</td>
                                <td>{{formatBytes .Size}}</td>
                                <td>
                                    <span class="badge {{if eq .Status "success"}}bg-success{{else if eq .Status "error"}}bg-danger{{else if eq .Status "pending"}}bg-info{{else}}bg-warning{{end}}">
                                        {{.Status}}
                                    </span>
                                </td>
                                <td>
                                    {{if .RetentionPolicy}}
                                    <span title="{{.RetentionPolicy}}">
                                        {{if not .ExpiresAt.IsZero}}
                                        Expires: {{formatTime .ExpiresAt}}
                                        {{else}}
                                        Keep forever
                                        {{end}}
                                    </span>
                                    {{else}}
                                    -
                                    {{end}}
                                </td>
                                <td>
                                    <div class="btn-group">
                                        {{if .LogFilePath}}
                                        <a href="/api/backups/log?id={{.ID}}" target="_blank" class="btn btn-sm btn-outline-info" title="View Log File">
                                            <i data-feather="file-text"></i>
                                        </a>
                                        {{end}}
                                        
                                        {{if and .LocalPath (eq .Status "success")}}
                                        <a href="/api/backups/download/local?id={{.ID}}" class="btn btn-sm btn-outline-primary" title="Download Local Backup">
                                            <i data-feather="download"></i>
                                        </a>
                                        {{end}}
                                        
                                        {{if and .S3Key (eq .S3UploadStatus "success")}}
                                        <a href="/s3download?id={{.ID}}" class="btn btn-sm btn-outline-secondary" title="S3 Download Options">
                                            <i data-feather="cloud"></i>
                                        </a>
                                        {{end}}
                                        
                                        {{if eq .Status "success"}}
                                        <button class="btn btn-sm btn-outline-danger delete-backup" data-id="{{.ID}}" title="Delete Backup">
                                            <i data-feather="trash-2"></i>
                                        </button>
                                        {{end}}
                                    </div>
                                </td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                </div>
                {{else}}
                <p class="card-text text-muted">No backups found for this database.</p>
                {{end}}
            </div>
        </div>
        {{else}}
        <div class="card">
            <div class="card-body text-center">
                <h4 class="text-muted mt-3 mb-3">Select a database from the list</h4>
                <p class="text-muted">Choose a database from the left panel to view its backups</p>
            </div>
        </div>
        {{end}}
    </div>
</div>

<!-- Run Backup Modal -->
<div class="modal fade" id="runBackupModal" tabindex="-1" aria-hidden="true">
    <div class="modal-dialog">
        <div class="modal-content">
            <div class="modal-header">
                <h5 class="modal-title">Run Backup for {{.Content.SelectedDB}}</h5>
                <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <div class="modal-body">
                <form id="manualBackupForm">
                    <div class="mb-3">
                        <label for="backupType" class="form-label">Backup Type</label>
                        <select class="form-select" id="backupType" name="backupType" required>
                            <option value="" selected disabled>Select Backup Type</option>
                            {{range $type, $_ := .Content.BackupTypes}}
                            <option value="{{$type}}">{{$type}}</option>
                            {{end}}
                        </select>
                    </div>
                    <input type="hidden" id="backupServer" name="backupServer" value="{{.Content.SelectedServer}}">
                    <input type="hidden" id="backupDatabase" name="backupDatabase" value="{{.Content.SelectedDB}}">
                </form>
            </div>
            <div class="modal-footer">
                <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Cancel</button>
                <button type="button" class="btn btn-primary" id="startBackupBtn">Start Backup</button>
            </div>
        </div>
    </div>
</div>

<!-- JavaScript for actions -->
<script>
// Function to change server selection
function changeServer(server) {
    window.location.href = '/databases?server=' + encodeURIComponent(server);
}

document.addEventListener('DOMContentLoaded', function() {
    // Initialize modal
    var backupModal = new bootstrap.Modal(document.getElementById('runBackupModal'));
    
    // Show backup modal when Run Backup button is clicked
    document.getElementById('runBackupForDb').addEventListener('click', function() {
        backupModal.show();
    });
    
    // Handle manual backup form submission
    document.getElementById('startBackupBtn').addEventListener('click', function() {
        var backupType = document.getElementById('backupType').value;
        var database = document.getElementById('backupDatabase').value;
        
        if (!backupType) {
            alert('Please select a backup type');
            return;
        }
        
        // Disable button and show loading state
        var startBackupBtn = this;
        var originalButtonText = startBackupBtn.innerHTML;
        startBackupBtn.disabled = true;
        startBackupBtn.innerHTML = '<span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span> Starting...';
        
        try {
            var url = '/api/backups/run?type=' + encodeURIComponent(backupType);
            var server = document.getElementById('backupServer').value;
            
            if (database) {
                url += '&database=' + encodeURIComponent(database);
            }
            
            if (server) {
                url += '&server=' + encodeURIComponent(server);
            }
            
            var fetchResponse;
            fetch(url, {
                method: 'POST'
            })
            .then(function(response) {
                fetchResponse = response;
                return response.json();
            })
            .then(function(result) {
                backupModal.hide();
                
                if (fetchResponse.ok) {
                    // Show success alert
                    showAlert('success', result.message + ' The page will refresh in a moment to show the new backup.');
                    
                    // Reset form and refresh page after a delay
                    document.getElementById('backupType').selectedIndex = 0;
                    setTimeout(function() {
                        window.location.reload();
                    }, 3000);
                } else {
                    // Show error alert
                    showAlert('danger', 'Error: ' + (result.message || 'Failed to start backup'));
                }
            })
            .catch(function(error) {
                backupModal.hide();
                showAlert('danger', 'Error: ' + error.message);
            })
            .finally(function() {
                // Restore button state
                startBackupBtn.disabled = false;
                startBackupBtn.innerHTML = originalButtonText;
            });
        } catch (error) {
            backupModal.hide();
            showAlert('danger', 'Error: ' + error.message);
            // Restore button state
            startBackupBtn.disabled = false;
            startBackupBtn.innerHTML = originalButtonText;
        }
    });
    
    // Function to show alert message
    function showAlert(type, message) {
        var alertDiv = document.createElement('div');
        alertDiv.className = 'alert alert-' + type + ' alert-dismissible fade show';
        alertDiv.role = 'alert';
        alertDiv.innerHTML = message +
            '<button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>';
        
        // Insert alert at the top of the content
        var content = document.querySelector('.row');
        content.parentNode.insertBefore(alertDiv, content);
        
        // Auto-dismiss after 5 seconds
        setTimeout(function() {
            alertDiv.classList.remove('show');
            setTimeout(function() {
                alertDiv.remove();
            }, 150);
        }, 5000);
    }
    
    // Handle delete backup buttons
    document.querySelectorAll('.delete-backup').forEach(function(button) {
        button.addEventListener('click', function() {
            var backupId = this.dataset.id;
            if (!confirm('Are you sure you want to delete this backup? This action cannot be undone.')) {
                return;
            }
            
            var deleteResponse;
            fetch('/api/backups/delete?id=' + backupId, {
                method: 'POST'
            })
            .then(function(response) {
                deleteResponse = response;
                return response.json();
            })
            .then(function(result) {
                alert(result.message);
                // Reload page after successful deletion
                if (deleteResponse.ok) {
                    window.location.reload();
                }
            })
            .catch(function(error) {
                alert('Error: ' + error.message);
            });
        });
    });
    
    // Initialize feather icons
    feather.replace();
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

	// Get selected server and database from query parameters
	selectedServer := r.URL.Query().Get("server")
	selectedDB := r.URL.Query().Get("db")

	// Create data for the page
	var data DatabasesPageData
	data.SelectedServer = selectedServer
	data.SelectedDB = selectedDB
	data.BackupTypes = config.CFG.BackupTypes
	data.LocalEnabled = config.CFG.Local.Enabled
	data.S3Enabled = config.CFG.S3.Enabled
	data.LastUpdated = time.Now()
	data.ServerDatabases = make(map[string][]string)

	// Populate server list
	if len(config.CFG.DatabaseServers) > 0 {
		// Get servers from multi-server configuration
		for _, server := range config.CFG.DatabaseServers {
			data.Servers = append(data.Servers, server.Name)
		}
	} else if config.CFG.MySQL.Host != "" {
		// Legacy configuration has a single server
		data.Servers = append(data.Servers, "default")
	}

	// Get databases for each server
	for _, serverName := range data.Servers {
		var databases []string

		// Find the server configuration
		var serverConfig *config.DatabaseServerConfig
		for i := range config.CFG.DatabaseServers {
			if config.CFG.DatabaseServers[i].Name == serverName {
				serverConfig = &config.CFG.DatabaseServers[i]
				break
			}
		}

		if serverConfig != nil {
			// Using multi-server configuration
			if len(serverConfig.IncludeDatabases) > 0 {
				// Use explicitly included databases
				databases = serverConfig.IncludeDatabases
			} else if serverConfig.Type == "mysql" {
				// Query MySQL server for databases
				// For now, we'll just use the common function
				// TODO: Query each server directly when needed
				queryDatabases, err := mysql.GetAllDatabases()
				if err != nil {
					log.Printf("Error fetching databases for server %s: %v", serverName, err)
				} else {
					// Apply exclude filter if needed
					if len(serverConfig.ExcludeDatabases) > 0 {
						excludeMap := make(map[string]bool)
						for _, db := range serverConfig.ExcludeDatabases {
							excludeMap[db] = true
						}

						for _, db := range queryDatabases {
							if !excludeMap[db] {
								databases = append(databases, db)
							}
						}
					} else {
						databases = queryDatabases
					}
				}
			}
		} else if serverName == "default" && config.CFG.MySQL.Host != "" {
			// Legacy configuration
			if len(config.CFG.MySQL.IncludeDatabases) > 0 {
				// Use explicitly included databases
				databases = config.CFG.MySQL.IncludeDatabases
			} else {
				// Query the MySQL server for databases
				queryDatabases, err := mysql.GetAllDatabases()
				if err != nil {
					log.Printf("Error fetching databases: %v", err)
				} else {
					// Apply exclude filter if needed
					if len(config.CFG.MySQL.ExcludeDatabases) > 0 {
						excludeMap := make(map[string]bool)
						for _, db := range config.CFG.MySQL.ExcludeDatabases {
							excludeMap[db] = true
						}

						for _, db := range queryDatabases {
							if !excludeMap[db] {
								databases = append(databases, db)
							}
						}
					} else {
						databases = queryDatabases
					}
				}
			}
		}

		// Add databases to the map
		data.ServerDatabases[serverName] = databases
	}

	// If a database is selected, get its backups
	if selectedDB != "" && metadata.DefaultStore != nil {
		// Use selected server for filtering backups
		data.DBBackups = metadata.DefaultStore.GetBackupsFiltered(selectedServer, selectedDB, "", true)
	}

	// Render the template
	renderTemplate(w, tmpl, "/databases", PageData{
		Title:       "Database Browser",
		Description: "Browse databases and their backups",
		Content:     data,
	})
}
