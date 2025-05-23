package pages

import (
	"net/http"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
)

// DashboardData holds data for the dashboard page
type DashboardData struct {
	Stats          map[string]interface{}
	RecentBackups  []types.BackupMeta
	Databases      []string
	BackupTypes    map[string]config.BackupTypeConfig
	LocalEnabled   bool
	S3Enabled      bool
	LastUpdated    time.Time
}

// DefaultPage renders the main dashboard
func DefaultPage(w http.ResponseWriter, r *http.Request) {
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
    <!-- Summary Cards -->
    <div class="col-md-4">
        <div class="card bg-light">
            <div class="card-body">
                <h5 class="card-title">Backups</h5>
                <p class="display-4">{{.Content.Stats.totalCount}}</p>
                <div class="text-muted">Total backups</div>
            </div>
        </div>
    </div>
    <div class="col-md-4">
        <div class="card bg-light">
            <div class="card-body">
                <h5 class="card-title">Storage</h5>
                <p class="display-4">{{formatBytes .Content.Stats.totalLocalSize}}</p>
                <div class="text-muted">Total backup size</div>
            </div>
        </div>
    </div>
    <div class="col-md-4">
        <div class="card bg-light">
            <div class="card-body">
                <h5 class="card-title">Last Backup</h5>
                {{if .Content.Stats.lastBackupTime}}
                <p class="card-text">{{timeAgo .Content.Stats.lastBackupTime}}</p>
                <div class="text-muted">{{formatTime .Content.Stats.lastBackupTime}}</div>
                {{else}}
                <p class="card-text text-muted">No backups yet</p>
                {{end}}
            </div>
        </div>
    </div>
</div>

<!-- Status Counts -->
<div class="row mt-4">
    {{range $status, $count := .Content.Stats.statusCounts}}
    <div class="col-md-3">
        <div class="card">
            <div class="card-body {{if eq $status "success"}}bg-success-light{{else if eq $status "error"}}bg-danger-light{{else if eq $status "pending"}}bg-info-light{{else}}bg-warning-light{{end}}">
                <h5 class="card-title">{{$status}}</h5>
                <p class="display-4">{{$count}}</p>
            </div>
        </div>
    </div>
    {{end}}
</div>

<!-- Server Statistics -->
{{if .Content.Stats.serverDistribution}}
<div class="row mt-4">
    <div class="col-12">
        <div class="card">
            <div class="card-header">
                <i data-feather="server"></i> Server Statistics
            </div>
            <div class="card-body">
                <div class="row">
                    {{range $server, $count := .Content.Stats.serverDistribution}}
                    <div class="col-md-3 mb-3">
                        <div class="card bg-light">
                            <div class="card-body text-center">
                                <h6 class="card-title">{{$server}}</h6>
                                <p class="display-6">{{$count}}</p>
                                <small class="text-muted">backups</small>
                            </div>
                        </div>
                    </div>
                    {{end}}
                </div>
                <div class="text-end mt-2">
                    <a href="/servers" class="btn btn-sm btn-outline-primary">View Server Details</a>
                </div>
            </div>
        </div>
    </div>
</div>
{{end}}

<!-- Recent Backups -->
<div class="row mt-4">
    <div class="col-12">
        <div class="card">
            <div class="card-header">
                Recent Backups
            </div>
            <div class="card-body">
                {{if .Content.RecentBackups}}
                <div class="table-responsive">
                    <table class="table table-striped table-hover">
                        <thead>
                            <tr>
                                <th>Server</th>
                                <th>Database</th>
                                <th>Type</th>
                                <th>Created</th>
                                <th>Size</th>
                                <th>Status</th>
                            </tr>
                        </thead>
                        <tbody>
                            {{range .Content.RecentBackups}}
                            <tr>
                                <td>{{.ServerName}}</td>
                                <td>{{.Database}}</td>
                                <td>{{.BackupType}}</td>
                                <td>{{formatTime .CreatedAt}}</td>
                                <td>{{formatBytes .Size}}</td>
                                <td>
                                    <span class="badge {{if eq .Status "success"}}bg-success{{else if eq .Status "error"}}bg-danger{{else if eq .Status "pending"}}bg-info{{else}}bg-warning{{end}}">
                                        {{.Status}}
                                    </span>
                                </td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                </div>
                <a href="/status/backups" class="btn btn-sm btn-primary">View All Backups</a>
                {{else}}
                <p class="card-text text-muted">No backups have been created yet.</p>
                {{end}}
            </div>
        </div>
    </div>
</div>

<!-- Quick Actions -->
<div class="row mt-4">
    <div class="col-12">
        <div class="card">
            <div class="card-header">
                Quick Actions
            </div>
            <div class="card-body">
                <div class="d-flex gap-2 flex-wrap">
                    {{range $type, $config := .Content.BackupTypes}}
                    <button class="btn btn-outline-primary btn-sm trigger-backup" data-type="{{$type}}">
                        Run {{$type}} Backup
                    </button>
                    {{end}}
                    <button class="btn btn-outline-warning btn-sm" id="run-retention">
                        Run Retention
                    </button>
                </div>
            </div>
        </div>
    </div>
</div>

<!-- JavaScript for actions -->
<script>
document.addEventListener('DOMContentLoaded', () => {
    // Handle backup triggers
    document.querySelectorAll('.trigger-backup').forEach(button => {
        button.addEventListener('click', async (e) => {
            const type = e.target.dataset.type;
            if (!confirm('Are you sure you want to run a ' + type + ' backup?')) {
                return;
            }
            
            button.disabled = true;
            button.textContent = 'Running...';
            
            try {
                const response = await fetch('/api/backups/run?type=' + type, {
                    method: 'POST'
                });
                
                const result = await response.json();
                alert(result.message);
            } catch (error) {
                alert('Error: ' + error.message);
            } finally {
                button.disabled = false;
                button.textContent = 'Run ' + type + ' Backup';
            }
        });
    });
    
    // Handle retention trigger
    document.getElementById('run-retention').addEventListener('click', async () => {
        if (!confirm('Are you sure you want to run retention policy enforcement?')) {
            return;
        }
        
        const button = document.getElementById('run-retention');
        button.disabled = true;
        button.textContent = 'Running...';
        
        try {
            const response = await fetch('/api/retention/run', {
                method: 'POST'
            });
            
            const result = await response.json();
            alert(result.message);
        } catch (error) {
            alert('Error: ' + error.message);
        } finally {
            button.disabled = false;
            button.textContent = 'Run Retention';
        }
    });
});
</script>
{{end}}
`
	var err error
	tmpl, err = tmpl.Parse(contentTemplate)
	if err != nil {
		http.Error(w, "Template parsing error", http.StatusInternalServerError)
		return
	}

	// Get data for the dashboard
	var dashboardData DashboardData
	
	// Get statistics
	dashboardData.Stats = make(map[string]interface{})
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
		// Sort by creation time (most recent first)
		// This is a simple approach; for a real implementation, you'd want to sort the backups
		if len(allBackups) > 5 {
			dashboardData.RecentBackups = allBackups[len(allBackups)-5:]
		} else {
			dashboardData.RecentBackups = allBackups
		}
	}
	
	// Get configuration info
	// Use includeDatabases first if available, otherwise use an empty list since
	// we're moving away from the static database list approach
	if len(config.CFG.MySQL.IncludeDatabases) > 0 {
		dashboardData.Databases = config.CFG.MySQL.IncludeDatabases
	} else {
		// In a real implementation, we would query the database server here
		dashboardData.Databases = []string{}
		
		// For development/testing, add some placeholder databases
		if len(dashboardData.Databases) == 0 {
			dashboardData.Databases = []string{"db1", "db2", "db3"}
		}
	}
	
	dashboardData.BackupTypes = config.CFG.BackupTypes
	dashboardData.LocalEnabled = config.CFG.Local.Enabled
	dashboardData.S3Enabled = config.CFG.S3.Enabled
	dashboardData.LastUpdated = time.Now()

	// Render the template
	renderTemplate(w, tmpl, "/", PageData{
		Title:       "Dashboard",
		Description: "Current status and summary of MySQL backups",
		Content:     dashboardData,
	})
}
