// Package pages provides HTML pages for the admin UI.
package pages

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
	"github.com/supporttools/GoSQLGuard/pkg/storage/s3"
)

// S3DownloadPageData holds data for the S3 download info page
type S3DownloadPageData struct {
	BackupID      string
	BackupInfo    types.BackupMeta
	PresignedURL  string
	ExpiresIn     string
	Filename      string
	CurlCommand   string
	WgetCommand   string
	S3Bucket      string
	S3Key         string
	GeneratedAt   time.Time
	LocalEnabled  bool
	S3Enabled     bool
}

// S3DownloadPage renders the S3 download info page
func S3DownloadPage(w http.ResponseWriter, r *http.Request) {
	// Create a new template based on the common template
	tmpl := generateCommonTemplate()
	if tmpl == nil {
		http.Error(w, "Failed to generate template", http.StatusInternalServerError)
		return
	}

	// Add page-specific template
	contentTemplate := `
{{define "content"}}
<div class="container">
    <div class="row mb-4">
        <div class="col-12">
            <nav aria-label="breadcrumb">
                <ol class="breadcrumb">
                    <li class="breadcrumb-item"><a href="/databases">Databases</a></li>
                    <li class="breadcrumb-item"><a href="/status/backups">Backup Status</a></li>
                    <li class="breadcrumb-item active" aria-current="page">S3 Download</li>
                </ol>
            </nav>
        </div>
    </div>
    
    <div class="row mb-4">
        <div class="col-12">
            <div class="card">
                <div class="card-header bg-primary text-white">
                    <h5 class="mb-0">S3 Backup Download Information</h5>
                </div>
                <div class="card-body">
                    <div class="row mb-4">
                        <div class="col-md-6">
                            <h6>Backup Details</h6>
                            <table class="table table-sm">
                                <tr>
                                    <th style="width: 150px;">Database:</th>
                                    <td>{{.Content.BackupInfo.Database}}</td>
                                </tr>
                                <tr>
                                    <th>Backup Type:</th>
                                    <td>{{.Content.BackupInfo.BackupType}}</td>
                                </tr>
                                <tr>
                                    <th>Created:</th>
                                    <td>{{formatTime .Content.BackupInfo.CreatedAt}}</td>
                                </tr>
                                <tr>
                                    <th>Size:</th>
                                    <td>{{formatBytes .Content.BackupInfo.Size}}</td>
                                </tr>
                                <tr>
                                    <th>S3 Bucket:</th>
                                    <td>{{.Content.S3Bucket}}</td>
                                </tr>
                                <tr>
                                    <th>S3 Key:</th>
                                    <td><small class="text-muted">{{.Content.S3Key}}</small></td>
                                </tr>
                            </table>
                        </div>
                        <div class="col-md-6">
                            <div class="card border-info mb-3">
                                <div class="card-header bg-info text-white">Download Information</div>
                                <div class="card-body">
                                    <p>This download link will expire in <strong>{{.Content.ExpiresIn}}</strong>.</p>
                                    <p>Generated at: {{formatTime .Content.GeneratedAt}}</p>
                                    <p>Filename: <code>{{.Content.Filename}}</code></p>
                                    <div class="d-grid gap-2">
                                        <a href="{{.Content.PresignedURL}}" class="btn btn-primary btn-lg" target="_blank">
                                            <i data-feather="download-cloud" class="me-2"></i> Download from S3
                                        </a>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                    
                    <div class="row">
                        <div class="col-12">
                            <h6>Command Line Examples</h6>
                            <div class="card mb-3">
                                <div class="card-header">
                                    <span class="badge bg-secondary me-2">curl</span> Command Example
                                </div>
                                <div class="card-body">
                                    <div class="input-group">
                                        <input type="text" class="form-control font-monospace" readonly value="{{.Content.CurlCommand}}">
                                        <button class="btn btn-outline-secondary copy-cmd" type="button" data-clipboard-text="{{.Content.CurlCommand}}">
                                            <i data-feather="copy"></i> Copy
                                        </button>
                                    </div>
                                </div>
                            </div>
                            <div class="card">
                                <div class="card-header">
                                    <span class="badge bg-secondary me-2">wget</span> Command Example
                                </div>
                                <div class="card-body">
                                    <div class="input-group">
                                        <input type="text" class="form-control font-monospace" readonly value="{{.Content.WgetCommand}}">
                                        <button class="btn btn-outline-secondary copy-cmd" type="button" data-clipboard-text="{{.Content.WgetCommand}}">
                                            <i data-feather="copy"></i> Copy
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</div>

<script>
document.addEventListener('DOMContentLoaded', function() {
    // Initialize clipboard functionality
    document.querySelectorAll('.copy-cmd').forEach(function(button) {
        button.addEventListener('click', function() {
            var text = this.getAttribute('data-clipboard-text');
            navigator.clipboard.writeText(text).then(function() {
                // Change button text temporarily
                var originalHTML = button.innerHTML;
                button.innerHTML = '<i data-feather="check"></i> Copied!';
                feather.replace();
                
                // Reset button after 2 seconds
                setTimeout(function() {
                    button.innerHTML = originalHTML;
                    feather.replace();
                }, 2000);
            }).catch(function(err) {
                console.error('Could not copy text: ', err);
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
		http.Error(w, "Template parsing error: " + err.Error(), http.StatusInternalServerError)
		return
	}

	// Get backup ID from query parameters
	backupID := r.URL.Query().Get("id")
	if backupID == "" {
		http.Error(w, "Missing required parameter: id", http.StatusBadRequest)
		return
	}

	// Check if the backup exists
	backup, exists := metadata.DefaultStore.GetBackupByID(backupID)
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
	expirationDuration := 15 * time.Minute
	presignedURL, err := s3Client.GeneratePresignedURL(backup.S3Key, expirationDuration)
	if err != nil {
		log.Printf("Error generating presigned URL: %v", err)
		http.Error(w, "Failed to generate download link", http.StatusInternalServerError)
		return
	}

	// Get filename for the download
	filename := fmt.Sprintf("%s-%s-%s.sql.gz", backup.Database, backup.BackupType, backup.CreatedAt.Format("2006-01-02-15-04-05"))

	// Create data for the page
	var data S3DownloadPageData
	data.BackupID = backupID
	data.BackupInfo = backup
	data.PresignedURL = presignedURL
	data.ExpiresIn = "15 minutes"
	data.Filename = filename
	data.S3Bucket = config.CFG.S3.Bucket
	data.S3Key = backup.S3Key
	data.GeneratedAt = time.Now()

	// Create curl command with URL escaping
	data.CurlCommand = fmt.Sprintf("curl -o %s '%s'", filename, presignedURL)
	
	// Create wget command with URL escaping
	data.WgetCommand = fmt.Sprintf("wget -O %s '%s'", filename, presignedURL)

	// Render the template
	renderTemplate(w, tmpl, "/s3download", PageData{
		Title:       "S3 Backup Download",
		Description: "Download information for S3 backup",
		Content:     data,
	})
}
