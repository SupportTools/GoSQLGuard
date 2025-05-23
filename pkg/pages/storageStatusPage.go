package pages

import (
	"net/http"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
)

// StorageStatusPageData holds data for the storage status page
type StorageStatusPageData struct {
	Stats        map[string]interface{}
	LocalStorage struct {
		Enabled    bool
		Path       string
		TotalSize  int64
		BackupCount int
	}
	S3Storage struct {
		Enabled    bool
		Bucket     string
		Region     string
		Endpoint   string
		Prefix     string
		TotalSize  int64
		BackupCount int
	}
	BackupTypes map[string]config.BackupTypeConfig
	LastUpdated time.Time
}

// StorageStatusPage renders the storage status page
func StorageStatusPage(w http.ResponseWriter, r *http.Request) {
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
    <!-- Local Storage Card -->
    <div class="col-md-6">
        <div class="card {{if .Content.LocalStorage.Enabled}}border-success{{else}}border-secondary{{end}} mb-4">
            <div class="card-header">
                <h5 class="card-title mb-0">
                    <i data-feather="hard-drive"></i> Local Storage
                    {{if .Content.LocalStorage.Enabled}}
                    <span class="badge bg-success">Enabled</span>
                    {{else}}
                    <span class="badge bg-secondary">Disabled</span>
                    {{end}}
                </h5>
            </div>
            <div class="card-body">
                {{if .Content.LocalStorage.Enabled}}
                <div class="mb-3">
                    <strong>Path:</strong> {{.Content.LocalStorage.Path}}
                </div>
                <div class="mb-3">
                    <strong>Total Size:</strong> {{formatBytes .Content.LocalStorage.TotalSize}}
                </div>
                <div class="mb-3">
                    <strong>Backup Count:</strong> {{.Content.LocalStorage.BackupCount}}
                </div>
                <div class="progress mb-3">
                    <div class="progress-bar bg-success" role="progressbar" style="width: 100%;" aria-valuenow="100" aria-valuemin="0" aria-valuemax="100">100%</div>
                </div>
                {{else}}
                <p class="card-text text-muted">Local storage is disabled in the configuration.</p>
                {{end}}
            </div>
        </div>
    </div>

    <!-- S3 Storage Card -->
    <div class="col-md-6">
        <div class="card {{if .Content.S3Storage.Enabled}}border-info{{else}}border-secondary{{end}} mb-4">
            <div class="card-header">
                <h5 class="card-title mb-0">
                    <i data-feather="cloud"></i> S3 Storage
                    {{if .Content.S3Storage.Enabled}}
                    <span class="badge bg-info">Enabled</span>
                    {{else}}
                    <span class="badge bg-secondary">Disabled</span>
                    {{end}}
                </h5>
            </div>
            <div class="card-body">
                {{if .Content.S3Storage.Enabled}}
                <div class="mb-3">
                    <strong>Bucket:</strong> {{.Content.S3Storage.Bucket}}
                </div>
                <div class="mb-3">
                    <strong>Region:</strong> {{.Content.S3Storage.Region}}
                </div>
                {{if .Content.S3Storage.Endpoint}}
                <div class="mb-3">
                    <strong>Endpoint:</strong> {{.Content.S3Storage.Endpoint}}
                </div>
                {{end}}
                <div class="mb-3">
                    <strong>Prefix:</strong> {{.Content.S3Storage.Prefix}}
                </div>
                <div class="mb-3">
                    <strong>Total Size:</strong> {{formatBytes .Content.S3Storage.TotalSize}}
                </div>
                <div class="mb-3">
                    <strong>Backup Count:</strong> {{.Content.S3Storage.BackupCount}}
                </div>
                <div class="progress mb-3">
                    <div class="progress-bar bg-info" role="progressbar" style="width: 100%;" aria-valuenow="100" aria-valuemin="0" aria-valuemax="100">100%</div>
                </div>
                {{else}}
                <p class="card-text text-muted">S3 storage is disabled in the configuration.</p>
                {{end}}
            </div>
        </div>
    </div>
</div>

<!-- Backup Types -->
<div class="card mb-4">
    <div class="card-header">
        <h5 class="card-title mb-0">Backup Types and Retention Policies</h5>
    </div>
    <div class="card-body">
        <div class="table-responsive">
            <table class="table table-striped">
                <thead>
                    <tr>
                        <th>Type</th>
                        <th>Schedule</th>
                        <th>Local Storage</th>
                        <th>Local Retention</th>
                        <th>S3 Storage</th>
                        <th>S3 Retention</th>
                    </tr>
                </thead>
                <tbody>
                    {{range $typeName, $typeConfig := .Content.BackupTypes}}
                    <tr>
                        <td>{{$typeName}}</td>
                        <td><code>{{$typeConfig.Schedule}}</code></td>
                        <td>
                            {{if $typeConfig.Local.Enabled}}
                            <span class="badge bg-success">Enabled</span>
                            {{else}}
                            <span class="badge bg-secondary">Disabled</span>
                            {{end}}
                        </td>
                        <td>
                            {{if $typeConfig.Local.Enabled}}
                                {{if $typeConfig.Local.Retention.Forever}}
                                Keep forever
                                {{else}}
                                {{$typeConfig.Local.Retention.Duration}}
                                {{end}}
                            {{else}}
                            -
                            {{end}}
                        </td>
                        <td>
                            {{if $typeConfig.S3.Enabled}}
                            <span class="badge bg-success">Enabled</span>
                            {{else}}
                            <span class="badge bg-secondary">Disabled</span>
                            {{end}}
                        </td>
                        <td>
                            {{if $typeConfig.S3.Enabled}}
                                {{if $typeConfig.S3.Retention.Forever}}
                                Keep forever
                                {{else}}
                                {{$typeConfig.S3.Retention.Duration}}
                                {{end}}
                            {{else}}
                            -
                            {{end}}
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>
</div>

<!-- Storage Actions -->
<div class="card">
    <div class="card-header">
        <h5 class="card-title mb-0">Storage Actions</h5>
    </div>
    <div class="card-body">
        <div class="d-flex gap-2 flex-wrap">
            <button class="btn btn-outline-primary" id="run-retention">
                Run Retention Policies
            </button>
            <button class="btn btn-outline-info" id="refresh-metadata">
                Refresh Storage Info
            </button>
        </div>
    </div>
</div>

<!-- JavaScript for actions -->
<script>
document.addEventListener('DOMContentLoaded', () => {
    // Handle retention policy button
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
            button.textContent = 'Run Retention Policies';
        }
    });
    
    // Handle refresh button
    document.getElementById('refresh-metadata').addEventListener('click', () => {
        window.location.reload();
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
		http.Error(w, "Template parsing error", http.StatusInternalServerError)
		return
	}

	// Get data for the page
	var data StorageStatusPageData
	
	// Initialize stats with defaults
	data.Stats = make(map[string]interface{})
	data.Stats["totalCount"] = 0
	data.Stats["totalLocalSize"] = int64(0)
	data.Stats["totalS3Size"] = int64(0)
	data.Stats["statusCounts"] = map[string]int{
		"success": 0,
		"pending": 0,
		"error":   0,
		"deleted": 0,
	}
	
	// Get statistics
	if metadata.DefaultStore != nil {
		stats := metadata.DefaultStore.GetStats()
		if stats != nil {
			data.Stats = stats
		}
	}
	
	// Set up local storage info
	data.LocalStorage.Enabled = config.CFG.Local.Enabled
	data.LocalStorage.Path = config.CFG.Local.BackupDirectory
	if data.Stats != nil {
		data.LocalStorage.TotalSize = data.Stats["totalLocalSize"].(int64)
		if statusCounts, ok := data.Stats["statusCounts"].(map[string]int); ok {
			data.LocalStorage.BackupCount = statusCounts["success"]
		}
	}
	
	// Set up S3 storage info
	data.S3Storage.Enabled = config.CFG.S3.Enabled
	data.S3Storage.Bucket = config.CFG.S3.Bucket
	data.S3Storage.Region = config.CFG.S3.Region
	data.S3Storage.Endpoint = config.CFG.S3.Endpoint
	data.S3Storage.Prefix = config.CFG.S3.Prefix
	if data.Stats != nil {
		data.S3Storage.TotalSize = data.Stats["totalS3Size"].(int64)
		// Count successful S3 uploads
		backups := metadata.DefaultStore.GetBackups()
		s3Count := 0
		for _, backup := range backups {
			if backup.S3UploadStatus == types.StatusSuccess {
				s3Count++
			}
		}
		data.S3Storage.BackupCount = s3Count
	}
	
	// Set backup types info
	data.BackupTypes = config.CFG.BackupTypes
	data.LastUpdated = time.Now()

	// Render the template
	renderTemplate(w, tmpl, "/status/storage", PageData{
		Title:       "Storage Status",
		Description: "Current status of backup storage destinations",
		Content:     data,
	})
}
