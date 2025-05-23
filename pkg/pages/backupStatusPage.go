// Package pages provides HTML pages for the admin UI.
package pages

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/backup/database/mysql"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
)

// BackupStatusPageData holds data for the backup status page
type BackupStatusPageData struct {
	Backups         []types.BackupMeta
	RecentErrors    []types.BackupMeta
	Databases       []string
	Servers         []string
	BackupTypes     map[string]config.BackupTypeConfig
	LocalEnabled    bool
	S3Enabled       bool
	LastUpdated     time.Time
	FilterType      string
	FilterDB        string
	FilterServer    string
	FilterActive    bool
	FilterStatus    string
	FilterStartDate string
	FilterEndDate   string
	FilterSearch    string
}

// BackupStatusPage renders the backup status page
func BackupStatusPage(w http.ResponseWriter, r *http.Request) {
	// Create a new template based on the common template
	tmpl := generateCommonTemplate()
	if tmpl == nil {
		http.Error(w, "Failed to generate template", http.StatusInternalServerError)
		return
	}

	// Add page-specific template
	contentTemplate := `
{{define "content"}}
<!-- Manual Backup -->
<div class="card mb-4">
    <div class="card-header">
        Run Manual Backup
    </div>
    <div class="card-body">
        <form id="manualBackupForm" class="row g-3">
            <div class="col-md-3">
                <label for="backupServer" class="form-label">Server</label>
                <select class="form-select" id="backupServer" name="backupServer" multiple>
                    <option value="all">All Servers</option>
                    {{range $server := .Content.Servers}}
                    <option value="{{$server}}">{{$server}}</option>
                    {{end}}
                </select>
                <div class="form-text">Hold Ctrl/Cmd to select multiple servers</div>
            </div>
            <div class="col-md-3">
                <label for="backupType" class="form-label">Backup Type</label>
                <select class="form-select" id="backupType" name="backupType" required>
                    <option value="" selected disabled>Select Backup Type</option>
                    {{range $type, $_ := .Content.BackupTypes}}
                    <option value="{{$type}}">{{$type}}</option>
                    {{end}}
                </select>
            </div>
            <div class="col-md-3">
                <label for="backupDatabase" class="form-label">Database (Optional)</label>
                <select class="form-select" id="backupDatabase" name="backupDatabase" multiple>
                    <option value="">All Databases</option>
                    {{range $db := .Content.Databases}}
                    <option value="{{$db}}">{{$db}}</option>
                    {{end}}
                </select>
                <div class="form-text">Hold Ctrl/Cmd to select multiple databases</div>
            </div>
            <div class="col-md-3 d-flex align-items-end">
                <button type="submit" class="btn btn-primary" id="runBackupButton">
                    <i data-feather="play"></i> Run Backup Now
                </button>
            </div>
        </form>
    </div>
</div>

<!-- Filters -->
<div class="card mb-4">
    <div class="card-header">
        Filter Backups
    </div>
    <div class="card-body">
        <form id="filterForm" class="row g-3">
            <div class="col-md-3">
                <label for="filterServer" class="form-label">Server</label>
                <select class="form-select" id="filterServer" name="server">
                    <option value="">All Servers</option>
                    {{range $server := .Content.Servers}}
                    <option value="{{$server}}" {{if eq $.Content.FilterServer $server}}selected{{end}}>{{$server}}</option>
                    {{end}}
                </select>
            </div>
            <div class="col-md-3">
                <label for="filterType" class="form-label">Backup Type</label>
                <select class="form-select" id="filterType" name="type">
                    <option value="">All Types</option>
                    {{range $type, $_ := .Content.BackupTypes}}
                    <option value="{{$type}}" {{if eq $.Content.FilterType $type}}selected{{end}}>{{$type}}</option>
                    {{end}}
                </select>
            </div>
            <div class="col-md-3">
                <label for="filterDB" class="form-label">Database</label>
                <select class="form-select" id="filterDB" name="database">
                    <option value="">All Databases</option>
                    {{range $db := .Content.Databases}}
                    <option value="{{$db}}" {{if eq $.Content.FilterDB $db}}selected{{end}}>{{$db}}</option>
                    {{end}}
                </select>
            </div>
            <div class="col-md-3">
                <label for="filterStatus" class="form-label">Status</label>
                <select class="form-select" id="filterStatus" name="status">
                    <option value="">All Statuses</option>
                    <option value="success" {{if eq $.Content.FilterStatus "success"}}selected{{end}}>Success</option>
                    <option value="error" {{if eq $.Content.FilterStatus "error"}}selected{{end}}>Error</option>
                    <option value="pending" {{if eq $.Content.FilterStatus "pending"}}selected{{end}}>Pending</option>
                    <option value="active" {{if $.Content.FilterActive}}selected{{end}}>Active Only</option>
                </select>
            </div>
            <!-- Date Range Filters -->
            <div class="col-md-2">
                <label for="filterStartDate" class="form-label">Start Date</label>
                <input type="date" class="form-control" id="filterStartDate" name="startDate" value="{{.Content.FilterStartDate}}">
            </div>
            <div class="col-md-2">
                <label for="filterEndDate" class="form-label">End Date</label>
                <input type="date" class="form-control" id="filterEndDate" name="endDate" value="{{.Content.FilterEndDate}}">
            </div>
            <!-- Search Field -->
            <div class="col-md-4">
                <label for="filterSearch" class="form-label">Search Backup ID</label>
                <input type="text" class="form-control" id="filterSearch" name="search" value="{{.Content.FilterSearch}}" placeholder="Enter backup ID to search">
            </div>
            <div class="col-12 mt-3">
                <button type="submit" class="btn btn-primary">Apply Filters</button>
                <a href="/status/backups" class="btn btn-outline-secondary ms-2">Reset</a>
                <button type="button" class="btn btn-outline-info ms-2" onclick="saveFilterPreferences()">Save Preferences</button>
            </div>
        </form>
    </div>
</div>

<!-- Error Summary -->
{{if .Content.RecentErrors}}
<div class="card mb-4 border-danger">
    <div class="card-header bg-danger text-white">
        <i data-feather="alert-circle"></i> Recent Backup Errors
    </div>
    <div class="card-body">
        <div class="table-responsive">
            <table class="table table-sm">
                <thead>
                    <tr>
                        <th>Time</th>
                        <th>Server</th>
                        <th>Database</th>
                        <th>Type</th>
                        <th>Error Message</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Content.RecentErrors}}
                    <tr>
                        <td>{{formatTime .CreatedAt}}</td>
                        <td>{{.ServerName}}</td>
                        <td>{{.Database}}</td>
                        <td>{{.BackupType}}</td>
                        <td class="text-truncate" style="max-width: 300px;" title="{{.ErrorMessage}}">
                            {{.ErrorMessage}}
                        </td>
                        <td>
                            <button type="button" class="btn btn-link btn-sm p-0" onclick="showErrorDetails('{{.ID}}', {{.ErrorMessage | json}}, {{.S3UploadError | json}})">
                                <i data-feather="info"></i> Details
                            </button>
                            {{if .LogFilePath}}
                            <a href="/api/backups/log?id={{.ID}}" target="_blank" class="btn btn-link btn-sm p-0 ms-2">
                                <i data-feather="file-text"></i> Log
                            </a>
                            {{end}}
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>
</div>
{{end}}

<!-- Backup Table -->
<div class="card">
    <div class="card-header d-flex justify-content-between align-items-center">
        <span>Backup History</span>
        <span class="text-muted small">Last updated: {{formatTime .Content.LastUpdated}}</span>
    </div>
    <div class="card-body">
        {{if .Content.Backups}}
        <div class="table-responsive">
            <table class="table table-striped table-hover">
                <thead>
                    <tr>
                        <th>ID</th>
                        <th>Server</th>
                        <th>Database</th>
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
                    {{range .Content.Backups}}
                    <tr>
                        <td title="{{.ID}}">
                            <span class="d-inline-block text-truncate" style="max-width: 150px;">{{.ID}}</span>
                        </td>
                        <td>{{.ServerName}}</td>
                        <td>{{.Database}}</td>
                        <td>{{.BackupType}}</td>
                        <td>{{formatTime .CreatedAt}}</td>
                        <td>{{if not .CompletedAt.IsZero}}{{formatTime .CompletedAt}}{{else}}-{{end}}</td>
                                <td>{{formatBytes .Size}}</td>
                        <td>
                            <span class="badge {{if eq .Status "success"}}bg-success{{else if eq .Status "error"}}bg-danger{{else if eq .Status "pending"}}bg-info{{else}}bg-warning{{end}}" 
                                  {{if and (eq .Status "error") .ErrorMessage}}data-bs-toggle="tooltip" data-bs-placement="top" title="{{.ErrorMessage}}"{{end}}>
                                {{.Status}}
                            </span>
                            {{if and (eq .Status "error") .ErrorMessage}}
                            <button type="button" class="btn btn-link btn-sm p-0 ms-1" onclick="showErrorDetails('{{.ID}}', {{.ErrorMessage | json}}, {{.S3UploadError | json}})">
                                <i data-feather="info" style="width: 16px; height: 16px;"></i>
                            </button>
                            {{end}}
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
                                
                                <button class="btn btn-sm btn-outline-danger delete-backup" data-id="{{.ID}}" title="Delete Backup">
                                    <i data-feather="trash-2"></i>
                                </button>
                            </div>
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        {{else}}
        <p class="card-text text-muted">No backups found matching your criteria.</p>
        {{end}}
    </div>
</div>

<!-- JavaScript for actions -->
<script>
document.addEventListener('DOMContentLoaded', function() {
    // Handle manual backup form submission
    document.getElementById('manualBackupForm').addEventListener('submit', function(e) {
        e.preventDefault();
        
        // Disable submit button and show loading state
        var runBackupButton = document.getElementById('runBackupButton');
        var originalButtonContent = runBackupButton.innerHTML;
        runBackupButton.disabled = true;
        runBackupButton.innerHTML = '<span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span> Starting...';
        
        var backupType = document.getElementById('backupType').value;
        
        // Get selected servers (handle multi-select)
        var serverSelect = document.getElementById('backupServer');
        var selectedServers = [];
        for (var i = 0; i < serverSelect.options.length; i++) {
            if (serverSelect.options[i].selected) {
                // Skip the "All Servers" option
                if (serverSelect.options[i].value !== 'all') {
                    selectedServers.push(serverSelect.options[i].value);
                }
            }
        }
        
        // Get selected databases (handle multi-select)
        var databaseSelect = document.getElementById('backupDatabase');
        var selectedDatabases = [];
        for (var i = 0; i < databaseSelect.options.length; i++) {
            if (databaseSelect.options[i].selected) {
                // Skip the "All Databases" option
                if (databaseSelect.options[i].value !== '') {
                    selectedDatabases.push(databaseSelect.options[i].value);
                }
            }
        }
        
        try {
            var url = '/api/backups/run?type=' + encodeURIComponent(backupType);
            
            // Add selected servers
            if (selectedServers.length > 0) {
                url += '&server=' + encodeURIComponent(selectedServers.join(','));
            }
            
            // Add selected databases
            if (selectedDatabases.length > 0) {
                url += '&database=' + encodeURIComponent(selectedDatabases.join(','));
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
                if (fetchResponse.ok) {
                    // Show success alert
                    showAlert('success', result.message + ' Refresh the page in a few moments to see the new backup.');
                    
                    // Reset form
                    document.getElementById('backupType').selectedIndex = 0;
                    document.getElementById('backupDatabase').selectedIndex = 0;
                } else {
                    // Show error alert
                    showAlert('danger', 'Error: ' + (result.message || 'Failed to start backup'));
                }
            })
            .catch(function(error) {
                showAlert('danger', 'Error: ' + error.message);
            })
            .finally(function() {
                // Restore button state
                runBackupButton.disabled = false;
                runBackupButton.innerHTML = originalButtonContent;
            });
        } catch (error) {
            showAlert('danger', 'Error: ' + error.message);
            // Restore button state
            runBackupButton.disabled = false;
            runBackupButton.innerHTML = originalButtonContent;
        }
    });
    
    // Function to show alert message
    function showAlert(type, message) {
        var alertDiv = document.createElement('div');
        alertDiv.className = 'alert alert-' + type + ' alert-dismissible fade show';
        alertDiv.role = 'alert';
        alertDiv.innerHTML = message +
            '<button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>';
        
        // Insert alert before the first card
        var firstCard = document.querySelector('.card');
        firstCard.parentNode.insertBefore(alertDiv, firstCard);
        
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
    
    // Initialize Bootstrap tooltips
    var tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'))
    var tooltipList = tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl)
    });
});

// Function to show error details modal
function showErrorDetails(backupId, errorMessage, s3Error) {
    var modalHtml = '<div class="modal fade" id="errorDetailsModal" tabindex="-1" aria-labelledby="errorDetailsModalLabel" aria-hidden="true">' +
        '<div class="modal-dialog modal-lg">' +
        '<div class="modal-content">' +
        '<div class="modal-header">' +
        '<h5 class="modal-title" id="errorDetailsModalLabel">Error Details - Backup ' + backupId + '</h5>' +
        '<button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>' +
        '</div>' +
        '<div class="modal-body">' +
        '<div class="mb-3">' +
        '<h6>Backup Error:</h6>' +
        '<pre class="bg-light p-3 rounded">' + (errorMessage || 'No error message available') + '</pre>' +
        '</div>';
    
    if (s3Error) {
        modalHtml += '<div class="mb-3">' +
            '<h6>S3 Upload Error:</h6>' +
            '<pre class="bg-light p-3 rounded">' + s3Error + '</pre>' +
            '</div>';
    }
    
    modalHtml += '<div class="mb-3">' +
        '<h6>Actions:</h6>' +
        '<a href="/api/backups/log?id=' + backupId + '" target="_blank" class="btn btn-sm btn-primary">' +
        '<i data-feather="file-text"></i> View Full Log File' +
        '</a>' +
        '</div>' +
        '</div>' +
        '<div class="modal-footer">' +
        '<button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Close</button>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>';
    
    // Remove existing modal if any
    var existingModal = document.getElementById('errorDetailsModal');
    if (existingModal) {
        existingModal.remove();
    }
    
    // Add modal to body
    document.body.insertAdjacentHTML('beforeend', modalHtml);
    
    // Initialize feather icons in modal
    feather.replace();
    
    // Show modal
    var modal = new bootstrap.Modal(document.getElementById('errorDetailsModal'));
    modal.show();
}

// Save filter preferences to localStorage
function saveFilterPreferences() {
    var filters = {
        server: document.getElementById('filterServer').value,
        type: document.getElementById('filterType').value,
        database: document.getElementById('filterDB').value,
        status: document.getElementById('filterStatus').value,
        startDate: document.getElementById('filterStartDate').value,
        endDate: document.getElementById('filterEndDate').value,
        search: document.getElementById('filterSearch').value
    };
    
    localStorage.setItem('gosqlguard_filters', JSON.stringify(filters));
    showAlert('info', 'Filter preferences saved');
}

// Load filter preferences from localStorage
function loadFilterPreferences() {
    var savedFilters = localStorage.getItem('gosqlguard_filters');
    if (savedFilters) {
        var filters = JSON.parse(savedFilters);
        
        // Only apply saved filters if current URL doesn't have any filter parameters
        var urlParams = new URLSearchParams(window.location.search);
        var hasUrlFilters = false;
        for (var pair of urlParams.entries()) {
            if (['server', 'type', 'database', 'status', 'startDate', 'endDate', 'search'].includes(pair[0])) {
                hasUrlFilters = true;
                break;
            }
        }
        
        if (!hasUrlFilters) {
            // Apply saved filters
            if (filters.server) document.getElementById('filterServer').value = filters.server;
            if (filters.type) document.getElementById('filterType').value = filters.type;
            if (filters.database) document.getElementById('filterDB').value = filters.database;
            if (filters.status) document.getElementById('filterStatus').value = filters.status;
            if (filters.startDate) document.getElementById('filterStartDate').value = filters.startDate;
            if (filters.endDate) document.getElementById('filterEndDate').value = filters.endDate;
            if (filters.search) document.getElementById('filterSearch').value = filters.search;
            
            // Auto-submit form to apply filters
            document.getElementById('filterForm').submit();
        }
    }
}

// Call loadFilterPreferences on page load
document.addEventListener('DOMContentLoaded', function() {
    setTimeout(loadFilterPreferences, 100);
});
</script>
{{end}}
`
	var err error
	tmpl, err = tmpl.Parse(contentTemplate)
	if err != nil {
		http.Error(w, "Template parsing error: " + err.Error(), http.StatusInternalServerError)
		return
	}

	// Get filter parameters
	filterType := r.URL.Query().Get("type")
	filterDB := r.URL.Query().Get("database")
	filterServer := r.URL.Query().Get("server")
	filterStatus := r.URL.Query().Get("status")
	filterActive := filterStatus == "active"
	filterStartDate := r.URL.Query().Get("startDate")
	filterEndDate := r.URL.Query().Get("endDate")
	filterSearch := r.URL.Query().Get("search")

	// Get data for the page
	var data BackupStatusPageData
	
	// Initialize backups list
	data.Backups = []types.BackupMeta{}
	
	// Get backups with applied filters
	if metadata.DefaultStore != nil {
		// Get backups with basic filters (don't use activeOnly if we have a specific status filter)
		useActiveOnly := filterActive && filterStatus == ""
		filteredBackups := metadata.DefaultStore.GetBackupsFiltered(filterServer, filterDB, filterType, useActiveOnly)
		
		// Apply additional filters
		var finalBackups []types.BackupMeta
		
		// Parse date filters
		var startDate, endDate time.Time
		if filterStartDate != "" {
			startDate, _ = time.Parse("2006-01-02", filterStartDate)
		}
		if filterEndDate != "" {
			endDate, _ = time.Parse("2006-01-02", filterEndDate)
			// Add 23:59:59 to include the entire end date
			endDate = endDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
		
		for _, backup := range filteredBackups {
			// Apply status filter
			if filterStatus != "" && filterStatus != "active" {
				if string(backup.Status) != filterStatus {
					continue
				}
			}
			
			// Apply date range filter
			if !startDate.IsZero() && backup.CreatedAt.Before(startDate) {
				continue
			}
			if !endDate.IsZero() && backup.CreatedAt.After(endDate) {
				continue
			}
			
			// Apply search filter (case-insensitive)
			if filterSearch != "" {
				searchLower := strings.ToLower(filterSearch)
				if !strings.Contains(strings.ToLower(backup.ID), searchLower) &&
				   !strings.Contains(strings.ToLower(backup.Database), searchLower) &&
				   !strings.Contains(strings.ToLower(backup.ServerName), searchLower) {
					continue
				}
			}
			
			finalBackups = append(finalBackups, backup)
		}
		
		data.Backups = finalBackups
	}
	
	// Set filter values for the form
	data.FilterType = filterType
	data.FilterDB = filterDB
	data.FilterServer = filterServer
	data.FilterActive = filterActive
	data.FilterStatus = filterStatus
	data.FilterStartDate = filterStartDate
	data.FilterEndDate = filterEndDate
	data.FilterSearch = filterSearch
	
	// Collect unique server names for the server dropdown
	serverSet := make(map[string]bool)
	for _, backup := range data.Backups {
		if backup.ServerName != "" {
			serverSet[backup.ServerName] = true
		}
	}
	
	// Also add servers from configuration
	for _, server := range config.CFG.DatabaseServers {
		serverSet[server.Name] = true
	}
	
	// Convert to slice
	data.Servers = make([]string, 0, len(serverSet))
	for server := range serverSet {
		data.Servers = append(data.Servers, server)
	}
	
	// Get database list for dropdown
	if len(config.CFG.MySQL.IncludeDatabases) > 0 {
		// If specific databases are included in the configuration, use those
		data.Databases = config.CFG.MySQL.IncludeDatabases
	} else {
		// Query the MySQL server for the list of databases
		databases, err := mysql.GetAllDatabases()
		if err != nil {
			log.Printf("Error fetching databases: %v", err)
			// Fall back to empty list if there's an error
			data.Databases = []string{}
		} else {
			data.Databases = databases
		}
	}
	
	data.BackupTypes = config.CFG.BackupTypes
	data.LocalEnabled = config.CFG.Local.Enabled
	data.S3Enabled = config.CFG.S3.Enabled
	data.LastUpdated = time.Now()
	
	// Get recent errors (last 10 failed backups) - only if we're not already filtering by error status
	if filterStatus != "error" && metadata.DefaultStore != nil {
		allBackups := metadata.DefaultStore.GetBackupsFiltered("", "", "", false)
		var errors []types.BackupMeta
		for _, backup := range allBackups {
			if backup.Status == types.StatusError && backup.ErrorMessage != "" {
				errors = append(errors, backup)
				if len(errors) >= 10 {
					break
				}
			}
		}
		data.RecentErrors = errors
	}

	// Render the template
	renderTemplate(w, tmpl, "/status/backups", PageData{
		Title:       "Backup Status",
		Description: "Detailed status of all MySQL backups",
		Content:     data,
	})
}
