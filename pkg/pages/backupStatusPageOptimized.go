package pages

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
)

// EnhancedBackupPageData extends BackupStatusPageData with pagination
type EnhancedBackupPageData struct {
	BackupStatusPageData
	// Pagination fields
	IsPaginated    bool
	CurrentPage    int
	PageSize       int
	TotalBackups   int64
	TotalPages     int
	ShowingStart   int
	ShowingEnd     int
	PageNumbers    []int
	PrevPageQuery  template.URL
	NextPageQuery  template.URL
	// Sorting
	SortBy    string
	SortOrder string
}

// BackupStatusPageOptimized renders the optimized backup status page with pagination
func BackupStatusPageOptimized(w http.ResponseWriter, r *http.Request) {
	// Create a new template based on the common template
	tmpl := generateCommonTemplate()
	if tmpl == nil {
		http.Error(w, "Failed to generate template", http.StatusInternalServerError)
		return
	}

	// Add page-specific template with pagination support
	contentTemplate := `
{{define "content"}}
<!-- Manual Backup Form (same as before) -->
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

<!-- Enhanced Filters -->
<div class="card mb-4">
    <div class="card-header">
        Filter Backups
    </div>
    <div class="card-body">
        <form id="filterForm" class="row g-3">
            <!-- Previous filters remain the same -->
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
            <div class="col-md-2">
                <label for="filterStartDate" class="form-label">Start Date</label>
                <input type="date" class="form-control" id="filterStartDate" name="startDate" value="{{.Content.FilterStartDate}}">
            </div>
            <div class="col-md-2">
                <label for="filterEndDate" class="form-label">End Date</label>
                <input type="date" class="form-control" id="filterEndDate" name="endDate" value="{{.Content.FilterEndDate}}">
            </div>
            <div class="col-md-4">
                <label for="filterSearch" class="form-label">Search</label>
                <input type="text" class="form-control" id="filterSearch" name="search" value="{{.Content.FilterSearch}}" placeholder="Search backup ID, server, or database">
            </div>
            
            <!-- Pagination Controls -->
            <div class="col-md-2">
                <label for="pageSize" class="form-label">Page Size</label>
                <select class="form-select" id="pageSize" name="pageSize">
                    <option value="25" {{if eq $.Content.PageSize 25}}selected{{end}}>25</option>
                    <option value="50" {{if eq $.Content.PageSize 50}}selected{{end}}>50</option>
                    <option value="100" {{if eq $.Content.PageSize 100}}selected{{end}}>100</option>
                    <option value="200" {{if eq $.Content.PageSize 200}}selected{{end}}>200</option>
                </select>
            </div>
            
            <div class="col-12 mt-3">
                <button type="submit" class="btn btn-primary">Apply Filters</button>
                <a href="/status/backups" class="btn btn-outline-secondary ms-2">Reset</a>
                <button type="button" class="btn btn-outline-info ms-2" onclick="saveFilterPreferences()">Save Preferences</button>
            </div>
        </form>
    </div>
</div>

<!-- Pagination Info -->
{{if .Content.IsPaginated}}
<div class="row mb-3">
    <div class="col-md-6">
        <p class="text-muted">
            Showing {{.Content.ShowingStart}}-{{.Content.ShowingEnd}} of {{.Content.TotalBackups}} backups
        </p>
    </div>
    <div class="col-md-6 text-end">
        <nav aria-label="Backup pagination">
            <ul class="pagination justify-content-end">
                <li class="page-item {{if le .Content.CurrentPage 1}}disabled{{end}}">
                    <a class="page-link" href="?{{.Content.PrevPageQuery}}" tabindex="-1">Previous</a>
                </li>
                {{range .Content.PageNumbers}}
                <li class="page-item {{if eq . $.Content.CurrentPage}}active{{end}}">
                    <a class="page-link" href="?{{$.Content.GetPageQuery .}}">{{.}}</a>
                </li>
                {{end}}
                <li class="page-item {{if ge .Content.CurrentPage .Content.TotalPages}}disabled{{end}}">
                    <a class="page-link" href="?{{.Content.NextPageQuery}}">Next</a>
                </li>
            </ul>
        </nav>
    </div>
</div>
{{end}}

<!-- Backup Status Table -->
<div class="card">
    <div class="card-header">
        Backup Status
    </div>
    <div class="card-body">
        {{if not .Content.Backups}}
        <div class="alert alert-info" role="alert">
            No backups found matching the current filters.
        </div>
        {{else}}
        <div class="table-responsive">
            <table class="table">
                <thead>
                    <tr>
                        <th><a href="?{{.Content.GetSortQuery "created_at"}}">Created <i data-feather="{{.Content.GetSortIcon "created_at"}}"></i></a></th>
                        <th><a href="?{{.Content.GetSortQuery "server_name"}}">Server <i data-feather="{{.Content.GetSortIcon "server_name"}}"></i></a></th>
                        <th><a href="?{{.Content.GetSortQuery "database_name"}}">Database <i data-feather="{{.Content.GetSortIcon "database_name"}}"></i></a></th>
                        <th><a href="?{{.Content.GetSortQuery "backup_type"}}">Type <i data-feather="{{.Content.GetSortIcon "backup_type"}}"></i></a></th>
                        <th>Duration</th>
                        <th><a href="?{{.Content.GetSortQuery "size"}}">Size <i data-feather="{{.Content.GetSortIcon "size"}}"></i></a></th>
                        <th><a href="?{{.Content.GetSortQuery "status"}}">Status <i data-feather="{{.Content.GetSortIcon "status"}}"></i></a></th>
                        <th>Storage</th>
                        <th>Retention</th>
                        <th>Actions</th>
                    </tr>
                </thead>
                <tbody>
                    {{range .Content.Backups}}
                    <tr>
                        <td>{{formatTime .CreatedAt}}</td>
                        <td>{{.ServerName}}</td>
                        <td>{{.Database}}</td>
                        <td>{{.BackupType}}</td>
                        <td>{{if not .CompletedAt.IsZero}}{{duration .CreatedAt .CompletedAt}}{{else}}-{{end}}</td>
                        <td>{{if gt .Size 0}}{{formatBytes .Size}}{{else}}-{{end}}</td>
                        <td>
                            {{if eq .Status "success"}}
                            <span class="badge bg-success">Success</span>
                            {{else if eq .Status "error"}}
                            <span class="badge bg-danger">Error</span>
                            {{else if eq .Status "pending"}}
                            <span class="badge bg-warning">Pending</span>
                            {{else}}
                            <span class="badge bg-secondary">{{.Status}}</span>
                            {{end}}
                        </td>
                        <td>
                            {{if .LocalPaths}}
                            <span class="badge bg-primary" title="Local storage">L</span>
                            {{end}}
                            {{if eq .S3UploadStatus "success"}}
                            <span class="badge bg-info" title="S3 storage">S3</span>
                            {{else if eq .S3UploadStatus "error"}}
                            <span class="badge bg-danger" title="S3 upload failed">S3!</span>
                            {{else if eq .S3UploadStatus "pending"}}
                            <span class="badge bg-warning" title="S3 upload pending">S3...</span>
                            {{end}}
                        </td>
                        <td><small>{{.RetentionPolicy}}</small></td>
                        <td>
                            {{if eq .Status "error"}}
                            <button type="button" class="btn btn-link btn-sm p-0" onclick="showErrorDetails('{{.ID}}', {{.ErrorMessage | json}}, {{.S3UploadError | json}})">
                                <i data-feather="alert-circle"></i>
                            </button>
                            {{end}}
                            {{if .LogFilePath}}
                            <a href="/api/backups/log?id={{.ID}}" target="_blank" class="btn btn-link btn-sm p-0">
                                <i data-feather="file-text"></i>
                            </a>
                            {{end}}
                            {{if .LocalPaths}}
                            <a href="/api/backups/download/local?id={{.ID}}" class="btn btn-link btn-sm p-0">
                                <i data-feather="download"></i>
                            </a>
                            {{end}}
                            {{if eq .S3UploadStatus "success"}}
                            <a href="/s3download?backup={{.ID}}" class="btn btn-link btn-sm p-0">
                                <i data-feather="cloud-download"></i>
                            </a>
                            {{end}}
                            <button type="button" class="btn btn-link btn-sm p-0 text-danger" onclick="deleteBackup('{{.ID}}')">
                                <i data-feather="trash-2"></i>
                            </button>
                        </td>
                    </tr>
                    {{end}}
                </tbody>
            </table>
        </div>
        {{end}}
    </div>
</div>

<!-- Bottom Pagination -->
{{if .Content.IsPaginated}}
<div class="row mt-3">
    <div class="col-12">
        <nav aria-label="Backup pagination">
            <ul class="pagination justify-content-center">
                <li class="page-item {{if le .Content.CurrentPage 1}}disabled{{end}}">
                    <a class="page-link" href="?{{.Content.PrevPageQuery}}" tabindex="-1">Previous</a>
                </li>
                {{range .Content.PageNumbers}}
                <li class="page-item {{if eq . $.Content.CurrentPage}}active{{end}}">
                    <a class="page-link" href="?{{$.Content.GetPageQuery .}}">{{.}}</a>
                </li>
                {{end}}
                <li class="page-item {{if ge .Content.CurrentPage .Content.TotalPages}}disabled{{end}}">
                    <a class="page-link" href="?{{.Content.NextPageQuery}}">Next</a>
                </li>
            </ul>
        </nav>
    </div>
</div>
{{end}}

<!-- Error Details Modal -->
<div class="modal fade" id="errorDetailsModal" tabindex="-1" aria-labelledby="errorDetailsModalLabel" aria-hidden="true">
    <div class="modal-dialog modal-lg">
        <div class="modal-content">
            <div class="modal-header">
                <h5 class="modal-title" id="errorDetailsModalLabel">Backup Error Details</h5>
                <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
            </div>
            <div class="modal-body" id="errorDetailsContent">
                <!-- Content will be populated by JavaScript -->
            </div>
            <div class="modal-footer">
                <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Close</button>
            </div>
        </div>
    </div>
</div>

<script>
// Enhanced JavaScript for pagination and performance
let currentRequest = null;

function loadBackups(page) {
    // Cancel any pending request
    if (currentRequest) {
        currentRequest.abort();
    }
    
    // Get current filters
    const params = new URLSearchParams(window.location.search);
    params.set('page', page);
    
    // Update URL without reloading
    window.history.pushState({}, '', '?' + params.toString());
    
    // Show loading indicator
    const tableBody = document.querySelector('tbody');
    tableBody.innerHTML = '<tr><td colspan="10" class="text-center"><div class="spinner-border" role="status"><span class="visually-hidden">Loading...</span></div></td></tr>';
    
    // Fetch data
    currentRequest = fetch('/api/backups?' + params.toString())
        .then(response => response.json())
        .then(data => {
            updateBackupTable(data);
            feather.replace();
        })
        .catch(error => {
            if (error.name !== 'AbortError') {
                console.error('Error loading backups:', error);
            }
        });
}

function updateBackupTable(data) {
    // This would update just the table content without full page reload
    // Implementation depends on the exact response format
}

// Performance optimization: Debounce search input
let searchTimeout;
document.getElementById('filterSearch')?.addEventListener('input', function(e) {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
        if (e.target.value.length === 0 || e.target.value.length >= 3) {
            document.getElementById('filterForm').submit();
        }
    }, 500);
});

// Save filter preferences
function saveFilterPreferences() {
    const filters = {
        server: document.getElementById('filterServer').value,
        type: document.getElementById('filterType').value,
        database: document.getElementById('filterDB').value,
        status: document.getElementById('filterStatus').value,
        pageSize: document.getElementById('pageSize').value
    };
    
    localStorage.setItem('backupFilters', JSON.stringify(filters));
    alert('Filter preferences saved!');
}

// Load filter preferences
window.addEventListener('DOMContentLoaded', function() {
    const saved = localStorage.getItem('backupFilters');
    if (saved && !window.location.search) {
        const filters = JSON.parse(saved);
        // Apply saved filters if no URL params present
        Object.keys(filters).forEach(key => {
            const element = document.getElementById('filter' + key.charAt(0).toUpperCase() + key.slice(1));
            if (element && filters[key]) {
                element.value = filters[key];
            }
        });
    }
});
</script>
{{end}}`

	tmpl, err := tmpl.Parse(contentTemplate)
	if err != nil {
		log.Printf("Error parsing backup status template: %v", err)
		http.Error(w, "Template parsing error", http.StatusInternalServerError)
		return
	}

	// Helper methods for template
	data := &EnhancedBackupPageData{
		BackupStatusPageData: BackupStatusPageData{
			BackupTypes:  config.CFG.BackupTypes,
			LocalEnabled: config.CFG.Local.Enabled,
			S3Enabled:    config.CFG.S3.Enabled,
			LastUpdated:  time.Now(),
		},
	}

	// Parse query parameters
	query := r.URL.Query()
	data.FilterType = query.Get("type")
	data.FilterDB = query.Get("database")
	data.FilterServer = query.Get("server")
	data.FilterStatus = query.Get("status")
	data.FilterStartDate = query.Get("startDate")
	data.FilterEndDate = query.Get("endDate")
	data.FilterSearch = query.Get("search")
	data.FilterActive = query.Get("activeOnly") == "true" || data.FilterStatus == "active"

	// Parse pagination parameters
	data.CurrentPage, _ = strconv.Atoi(query.Get("page"))
	if data.CurrentPage < 1 {
		data.CurrentPage = 1
	}
	
	data.PageSize, _ = strconv.Atoi(query.Get("pageSize"))
	if data.PageSize < 1 {
		data.PageSize = 50
	}

	data.SortBy = query.Get("sortBy")
	if data.SortBy == "" {
		data.SortBy = "created_at"
	}
	
	data.SortOrder = query.Get("sortOrder")
	if data.SortOrder == "" {
		data.SortOrder = "desc"
	}

	// Check if we have a paginated store
	if dbStore, ok := metadata.DefaultStore.(*metadata.DBStore); ok {
		// Use paginated query
		opts := metadata.QueryOptions{
			ServerName:   data.FilterServer,
			DatabaseName: data.FilterDB,
			BackupType:   data.FilterType,
			Status:       data.FilterStatus,
			SearchTerm:   data.FilterSearch,
			ActiveOnly:   data.FilterActive,
			Page:         data.CurrentPage,
			PageSize:     data.PageSize,
			SortBy:       data.SortBy,
			SortOrder:    data.SortOrder,
			PreloadPaths: true,
		}

		// Parse dates
		if data.FilterStartDate != "" {
			if t, err := time.Parse("2006-01-02", data.FilterStartDate); err == nil {
				opts.StartDate = &t
			}
		}
		if data.FilterEndDate != "" {
			if t, err := time.Parse("2006-01-02", data.FilterEndDate); err == nil {
				opts.EndDate = &t
			}
		}

		result, err := dbStore.GetBackupsPaginated(opts)
		if err != nil {
			log.Printf("Error getting paginated backups: %v", err)
			// Fall back to non-paginated
			data.Backups = metadata.DefaultStore.GetBackups()
		} else {
			data.IsPaginated = true
			data.Backups = result.Data
			data.TotalBackups = result.Total
			data.TotalPages = result.TotalPages
			
			// Calculate showing range
			data.ShowingStart = (data.CurrentPage-1)*data.PageSize + 1
			data.ShowingEnd = data.ShowingStart + len(data.Backups) - 1
			
			// Generate page numbers (show max 7 pages)
			data.PageNumbers = generatePageNumbers(data.CurrentPage, data.TotalPages)
			
			// Generate query strings for pagination links
			data.PrevPageQuery = generatePageQuery(query, data.CurrentPage-1)
			data.NextPageQuery = generatePageQuery(query, data.CurrentPage+1)
		}
	} else {
		// Use non-paginated query for file-based store
		data.Backups = metadata.DefaultStore.GetBackupsFiltered(
			data.FilterServer,
			data.FilterDB,
			data.FilterType,
			data.FilterActive,
		)
		// Apply additional filters manually
		data.Backups = applyAdditionalFilters(data.Backups, data)
	}

	// Get unique databases and servers
	data.Databases = getUniqueDatabases()
	data.Servers = getUniqueServers()

	// Get recent errors
	data.RecentErrors = getRecentErrors(5)

	// Add helper methods to data
	funcMap := template.FuncMap{
		"GetPageQuery": func(page int) template.URL {
			return generatePageQuery(query, page)
		},
		"GetSortQuery": func(field string) template.URL {
			return generateSortQuery(query, field, data.SortBy, data.SortOrder)
		},
		"GetSortIcon": func(field string) string {
			if data.SortBy != field {
				return "chevron-down"
			}
			if data.SortOrder == "asc" {
				return "chevron-up"
			}
			return "chevron-down"
		},
	}

	// Execute template with enhanced function map
	tmpl = tmpl.Funcs(funcMap)
	
	pageData := PageData{
		Title:   "Backup Status",
		Content: data,
	}

	if err := tmpl.Execute(w, pageData); err != nil {
		log.Printf("Error executing backup status template: %v", err)
		http.Error(w, "Template execution error", http.StatusInternalServerError)
		return
	}
}

// Helper functions for pagination

func generatePageNumbers(current, total int) []int {
	if total <= 7 {
		numbers := make([]int, total)
		for i := 0; i < total; i++ {
			numbers[i] = i + 1
		}
		return numbers
	}

	numbers := []int{}
	
	// Always show first page
	numbers = append(numbers, 1)
	
	// Calculate range around current page
	start := current - 2
	if start < 2 {
		start = 2
	}
	
	end := current + 2
	if end > total-1 {
		end = total - 1
	}
	
	// Add ellipsis if needed
	if start > 2 {
		numbers = append(numbers, -1) // -1 represents ellipsis
	}
	
	// Add middle pages
	for i := start; i <= end; i++ {
		numbers = append(numbers, i)
	}
	
	// Add ellipsis if needed
	if end < total-1 {
		numbers = append(numbers, -1) // -1 represents ellipsis
	}
	
	// Always show last page
	if total > 1 {
		numbers = append(numbers, total)
	}
	
	return numbers
}

func generatePageQuery(query map[string][]string, page int) template.URL {
	q := make(map[string][]string)
	for k, v := range query {
		if k != "page" {
			q[k] = v
		}
	}
	q["page"] = []string{strconv.Itoa(page)}
	
	params := ""
	for k, values := range q {
		for _, v := range values {
			if params != "" {
				params += "&"
			}
			params += fmt.Sprintf("%s=%s", k, v)
		}
	}
	
	return template.URL(params)
}

func generateSortQuery(query map[string][]string, field, currentSort, currentOrder string) template.URL {
	q := make(map[string][]string)
	for k, v := range query {
		if k != "sortBy" && k != "sortOrder" && k != "page" {
			q[k] = v
		}
	}
	
	q["sortBy"] = []string{field}
	
	// Toggle order if clicking the same field
	if field == currentSort && currentOrder == "desc" {
		q["sortOrder"] = []string{"asc"}
	} else {
		q["sortOrder"] = []string{"desc"}
	}
	
	params := ""
	for k, values := range q {
		for _, v := range values {
			if params != "" {
				params += "&"
			}
			params += fmt.Sprintf("%s=%s", k, v)
		}
	}
	
	return template.URL(params)
}

func applyAdditionalFilters(backups []types.BackupMeta, data *EnhancedBackupPageData) []types.BackupMeta {
	filtered := backups

	// Apply date filters
	if data.FilterStartDate != "" {
		if startDate, err := time.Parse("2006-01-02", data.FilterStartDate); err == nil {
			var temp []types.BackupMeta
			for _, b := range filtered {
				if b.CreatedAt.After(startDate) || b.CreatedAt.Equal(startDate) {
					temp = append(temp, b)
				}
			}
			filtered = temp
		}
	}

	if data.FilterEndDate != "" {
		if endDate, err := time.Parse("2006-01-02", data.FilterEndDate); err == nil {
			endDate = endDate.AddDate(0, 0, 1) // Include entire end date
			var temp []types.BackupMeta
			for _, b := range filtered {
				if b.CreatedAt.Before(endDate) {
					temp = append(temp, b)
				}
			}
			filtered = temp
		}
	}

	// Apply search filter
	if data.FilterSearch != "" {
		var temp []types.BackupMeta
		search := data.FilterSearch
		for _, b := range filtered {
			if contains(b.ID, search) || contains(b.ServerName, search) || contains(b.Database, search) {
				temp = append(temp, b)
			}
		}
		filtered = temp
	}

	// Apply status filter
	if data.FilterStatus != "" && data.FilterStatus != "active" {
		var temp []types.BackupMeta
		for _, b := range filtered {
			if string(b.Status) == data.FilterStatus {
				temp = append(temp, b)
			}
		}
		filtered = temp
	}

	return filtered
}

func contains(s, substr string) bool {
	if len(s) == 0 || len(substr) == 0 {
		return false
	}
	// Simple case-insensitive contains
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}

func getRecentErrors(limit int) []types.BackupMeta {
	var errors []types.BackupMeta
	backups := metadata.DefaultStore.GetBackups()
	
	for _, b := range backups {
		if b.Status == types.StatusError {
			errors = append(errors, b)
			if len(errors) >= limit {
				break
			}
		}
	}
	
	return errors
}

func getUniqueDatabases() []string {
	databaseMap := make(map[string]bool)
	backups := metadata.DefaultStore.GetBackups()
	
	for _, b := range backups {
		if b.Database != "" {
			databaseMap[b.Database] = true
		}
	}
	
	databases := make([]string, 0, len(databaseMap))
	for db := range databaseMap {
		databases = append(databases, db)
	}
	
	return databases
}

func getUniqueServers() []string {
	serverMap := make(map[string]bool)
	backups := metadata.DefaultStore.GetBackups()
	
	for _, b := range backups {
		if b.ServerName != "" {
			serverMap[b.ServerName] = true
		}
	}
	
	servers := make([]string, 0, len(serverMap))
	for server := range serverMap {
		servers = append(servers, server)
	}
	
	return servers
}