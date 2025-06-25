// Package pages provides HTML pages for the admin UI
package pages

import (
	"net/http"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/database"
)

// MySQLOptionsPageData holds data for the MySQL options page
type MySQLOptionsPageData struct {
	GlobalOptions     database.MySQLDumpOptions
	BackupTypeOptions map[string]database.MySQLDumpOptions
	ServerOptions     map[string]database.MySQLDumpOptions
	LastUpdated       time.Time
}

// MySQLOptionsPage renders the MySQL dump options configuration page
func MySQLOptionsPage(w http.ResponseWriter, r *http.Request) {
	// Create a new template based on the common template
	tmpl := generateCommonTemplate()
	if tmpl == nil {
		http.Error(w, "Failed to generate template", http.StatusInternalServerError)
		return
	}

	// Add page-specific template
	contentTemplate := `
{{define "content"}}
<!-- MySQL Dump Options Configuration -->
<div class="row mb-4">
    <div class="col-12">
        <div class="card">
            <div class="card-header">
                <i data-feather="settings"></i> MySQL Dump Options Configuration
            </div>
            <div class="card-body">
                <p class="text-muted">Configure options passed to mysqldump command during backup operations. Options can be set globally, per backup type, or per server.</p>
                
                <!-- Nav tabs -->
                <ul class="nav nav-tabs" id="optionsTabs" role="tablist">
                    <li class="nav-item" role="presentation">
                        <button class="nav-link active" id="global-tab" data-bs-toggle="tab" data-bs-target="#global" type="button" role="tab">
                            Global Defaults
                        </button>
                    </li>
                    <li class="nav-item" role="presentation">
                        <button class="nav-link" id="backup-types-tab" data-bs-toggle="tab" data-bs-target="#backup-types" type="button" role="tab">
                            Per Backup Type
                        </button>
                    </li>
                    <li class="nav-item" role="presentation">
                        <button class="nav-link" id="servers-tab" data-bs-toggle="tab" data-bs-target="#servers" type="button" role="tab">
                            Per Server
                        </button>
                    </li>
                </ul>
                
                <!-- Tab content -->
                <div class="tab-content mt-3" id="optionsTabContent">
                    <!-- Global Options Tab -->
                    <div class="tab-pane fade show active" id="global" role="tabpanel">
                        <h5>Global Default Options</h5>
                        <p class="text-muted">These options apply to all backups unless overridden.</p>
                        
                        <form id="globalOptionsForm">
                            <div class="row">
                                <div class="col-md-6">
                                    <h6>Transaction Options</h6>
                                    <div class="form-check">
                                        <input class="form-check-input" type="checkbox" id="global_single_transaction" name="singleTransaction" {{if .Content.GlobalOptions.SingleTransaction}}checked{{end}}>
                                        <label class="form-check-label" for="global_single_transaction">
                                            --single-transaction
                                            <small class="text-muted d-block">Consistent backup without locking tables (InnoDB)</small>
                                        </label>
                                    </div>
                                    <div class="form-check">
                                        <input class="form-check-input" type="checkbox" id="global_lock_tables" name="lockTables" {{if .Content.GlobalOptions.LockTables}}checked{{end}}>
                                        <label class="form-check-label" for="global_lock_tables">
                                            --lock-tables
                                            <small class="text-muted d-block">Lock all tables (MyISAM)</small>
                                        </label>
                                    </div>
                                    <div class="form-check">
                                        <input class="form-check-input" type="checkbox" id="global_skip_lock_tables" name="skipLockTables" {{if .Content.GlobalOptions.SkipLockTables}}checked{{end}}>
                                        <label class="form-check-label" for="global_skip_lock_tables">
                                            --skip-lock-tables
                                            <small class="text-muted d-block">Don't lock tables</small>
                                        </label>
                                    </div>
                                    
                                    <h6 class="mt-3">Performance Options</h6>
                                    <div class="form-check">
                                        <input class="form-check-input" type="checkbox" id="global_quick" name="quick" {{if .Content.GlobalOptions.Quick}}checked{{end}}>
                                        <label class="form-check-label" for="global_quick">
                                            --quick
                                            <small class="text-muted d-block">Don't buffer query, dump directly to output</small>
                                        </label>
                                    </div>
                                    <div class="form-check">
                                        <input class="form-check-input" type="checkbox" id="global_extended_insert" name="extendedInsert" {{if .Content.GlobalOptions.ExtendedInsert}}checked{{end}}>
                                        <label class="form-check-label" for="global_extended_insert">
                                            --extended-insert
                                            <small class="text-muted d-block">Use multiple-row INSERT syntax</small>
                                        </label>
                                    </div>
                                    <div class="form-check">
                                        <input class="form-check-input" type="checkbox" id="global_compress" name="compress" {{if .Content.GlobalOptions.Compress}}checked{{end}}>
                                        <label class="form-check-label" for="global_compress">
                                            --compress
                                            <small class="text-muted d-block">Compress client/server protocol</small>
                                        </label>
                                    </div>
                                </div>
                                
                                <div class="col-md-6">
                                    <h6>Data Options</h6>
                                    <div class="form-check">
                                        <input class="form-check-input" type="checkbox" id="global_complete_insert" name="completeInsert" {{if .Content.GlobalOptions.CompleteInsert}}checked{{end}}>
                                        <label class="form-check-label" for="global_complete_insert">
                                            --complete-insert
                                            <small class="text-muted d-block">Use complete INSERT statements with column names</small>
                                        </label>
                                    </div>
                                    <div class="form-check">
                                        <input class="form-check-input" type="checkbox" id="global_skip_comments" name="skipComments" {{if .Content.GlobalOptions.SkipComments}}checked{{end}}>
                                        <label class="form-check-label" for="global_skip_comments">
                                            --skip-comments
                                            <small class="text-muted d-block">Don't write comments in dump</small>
                                        </label>
                                    </div>
                                    <div class="form-check">
                                        <input class="form-check-input" type="checkbox" id="global_skip_add_locks" name="skipAddLocks" {{if .Content.GlobalOptions.SkipAddLocks}}checked{{end}}>
                                        <label class="form-check-label" for="global_skip_add_locks">
                                            --skip-add-locks
                                            <small class="text-muted d-block">Don't add locks around INSERT statements</small>
                                        </label>
                                    </div>
                                    
                                    <h6 class="mt-3">Schema Options</h6>
                                    <div class="form-check">
                                        <input class="form-check-input" type="checkbox" id="global_triggers" name="triggers" {{if .Content.GlobalOptions.Triggers}}checked{{end}}>
                                        <label class="form-check-label" for="global_triggers">
                                            --triggers
                                            <small class="text-muted d-block">Include triggers</small>
                                        </label>
                                    </div>
                                    <div class="form-check">
                                        <input class="form-check-input" type="checkbox" id="global_routines" name="routines" {{if .Content.GlobalOptions.Routines}}checked{{end}}>
                                        <label class="form-check-label" for="global_routines">
                                            --routines
                                            <small class="text-muted d-block">Include stored procedures and functions</small>
                                        </label>
                                    </div>
                                    <div class="form-check">
                                        <input class="form-check-input" type="checkbox" id="global_events" name="events" {{if .Content.GlobalOptions.Events}}checked{{end}}>
                                        <label class="form-check-label" for="global_events">
                                            --events
                                            <small class="text-muted d-block">Include events</small>
                                        </label>
                                    </div>
                                </div>
                            </div>
                            
                            <div class="row mt-3">
                                <div class="col-12">
                                    <h6>Custom Options</h6>
                                    <textarea class="form-control" id="global_custom_options" name="customOptions" rows="3" placeholder="Additional options, one per line">{{range .Content.GlobalOptions.CustomOptions}}{{.}}
{{end}}</textarea>
                                    <small class="text-muted">Enter additional mysqldump options, one per line (e.g., --hex-blob)</small>
                                </div>
                            </div>
                            
                            <div class="mt-3">
                                <button type="submit" class="btn btn-primary">Save Global Options</button>
                                <button type="button" class="btn btn-secondary ms-2" onclick="previewCommand('global')">Preview Command</button>
                            </div>
                        </form>
                    </div>
                    
                    <!-- Backup Types Tab -->
                    <div class="tab-pane fade" id="backup-types" role="tabpanel">
                        <h5>Per Backup Type Options</h5>
                        <p class="text-muted">Override global options for specific backup types.</p>
                        
                        <div class="accordion" id="backupTypesAccordion">
                            {{range $type, $config := .Content.BackupTypeOptions}}
                            <div class="accordion-item">
                                <h2 class="accordion-header">
                                    <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#collapse_{{$type}}">
                                        {{$type}} Backup Options
                                    </button>
                                </h2>
                                <div id="collapse_{{$type}}" class="accordion-collapse collapse" data-bs-parent="#backupTypesAccordion">
                                    <div class="accordion-body">
                                        <p class="text-info small">
                                            <i data-feather="info"></i> Leave unchecked to inherit from global defaults
                                        </p>
                                        <!-- Similar form structure as global options -->
                                        <button class="btn btn-sm btn-primary">Save {{$type}} Options</button>
                                    </div>
                                </div>
                            </div>
                            {{end}}
                        </div>
                    </div>
                    
                    <!-- Servers Tab -->
                    <div class="tab-pane fade" id="servers" role="tabpanel">
                        <h5>Per Server Options</h5>
                        <p class="text-muted">Override options for specific database servers.</p>
                        
                        <div class="accordion" id="serversAccordion">
                            {{range $server, $config := .Content.ServerOptions}}
                            <div class="accordion-item">
                                <h2 class="accordion-header">
                                    <button class="accordion-button collapsed" type="button" data-bs-toggle="collapse" data-bs-target="#collapse_server_{{$server}}">
                                        {{$server}} Server Options
                                    </button>
                                </h2>
                                <div id="collapse_server_{{$server}}" class="accordion-collapse collapse" data-bs-parent="#serversAccordion">
                                    <div class="accordion-body">
                                        <p class="text-info small">
                                            <i data-feather="info"></i> Leave unchecked to inherit from global/backup type defaults
                                        </p>
                                        <!-- Similar form structure as global options -->
                                        <button class="btn btn-sm btn-primary">Save {{$server}} Options</button>
                                    </div>
                                </div>
                            </div>
                            {{end}}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</div>

<!-- Command Preview Modal -->
<div class="modal fade" id="commandPreviewModal" tabindex="-1">
    <div class="modal-dialog modal-lg">
        <div class="modal-content">
            <div class="modal-header">
                <h5 class="modal-title">MySQL Dump Command Preview</h5>
                <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
            </div>
            <div class="modal-body">
                <pre id="commandPreview" class="bg-light p-3"></pre>
            </div>
        </div>
    </div>
</div>

<script>
document.addEventListener('DOMContentLoaded', function() {
    feather.replace();
    
    // Handle global options form submission
    document.getElementById('globalOptionsForm').addEventListener('submit', function(e) {
        e.preventDefault();
        saveOptions('global', this);
    });
});

function saveOptions(level, form) {
    const formData = new FormData(form);
    const options = {
        singleTransaction: formData.get('singleTransaction') === 'on',
        lockTables: formData.get('lockTables') === 'on',
        skipLockTables: formData.get('skipLockTables') === 'on',
        quick: formData.get('quick') === 'on',
        extendedInsert: formData.get('extendedInsert') === 'on',
        compress: formData.get('compress') === 'on',
        completeInsert: formData.get('completeInsert') === 'on',
        skipComments: formData.get('skipComments') === 'on',
        skipAddLocks: formData.get('skipAddLocks') === 'on',
        triggers: formData.get('triggers') === 'on',
        routines: formData.get('routines') === 'on',
        events: formData.get('events') === 'on',
        customOptions: formData.get('customOptions').split('\n').filter(opt => opt.trim())
    };
    
    // Send to API
    fetch('/api/mysql-options/' + level, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(options)
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            showAlert('success', 'Options saved successfully');
        } else {
            showAlert('danger', 'Failed to save options: ' + data.error);
        }
    })
    .catch(error => {
        showAlert('danger', 'Error saving options: ' + error.message);
    });
}

function previewCommand(level) {
    // Collect current form values
    const form = document.querySelector('#' + level + 'OptionsForm') || document.querySelector('form');
    const formData = new FormData(form);
    
    let command = 'mysqldump';
    
    // Build command based on selected options
    if (formData.get('singleTransaction') === 'on') command += ' --single-transaction';
    if (formData.get('lockTables') === 'on') command += ' --lock-tables';
    if (formData.get('skipLockTables') === 'on') command += ' --skip-lock-tables';
    if (formData.get('quick') === 'on') command += ' --quick';
    if (formData.get('extendedInsert') === 'on') command += ' --extended-insert';
    if (formData.get('compress') === 'on') command += ' --compress';
    if (formData.get('completeInsert') === 'on') command += ' --complete-insert';
    if (formData.get('skipComments') === 'on') command += ' --skip-comments';
    if (formData.get('skipAddLocks') === 'on') command += ' --skip-add-locks';
    if (formData.get('triggers') === 'on') command += ' --triggers';
    if (formData.get('routines') === 'on') command += ' --routines';
    if (formData.get('events') === 'on') command += ' --events';
    
    // Add custom options
    const customOptions = formData.get('customOptions');
    if (customOptions) {
        customOptions.split('\n').forEach(opt => {
            if (opt.trim()) command += ' ' + opt.trim();
        });
    }
    
    // Add standard options that are always included
    command += ' -h $HOST -P $PORT -u $USER -p$PASSWORD $DATABASE';
    
    document.getElementById('commandPreview').textContent = command;
    new bootstrap.Modal(document.getElementById('commandPreviewModal')).show();
}

function showAlert(type, message) {
    const alertDiv = document.createElement('div');
    alertDiv.className = 'alert alert-' + type + ' alert-dismissible fade show mt-3';
    alertDiv.innerHTML = message + '<button type="button" class="btn-close" data-bs-dismiss="alert"></button>';
    document.querySelector('.card-body').insertBefore(alertDiv, document.querySelector('.card-body').firstChild);
}
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
	var data MySQLOptionsPageData

	// Get global options (for now, use defaults as config isn't properly integrated)
	data.GlobalOptions = database.MySQLDumpOptions{
		SingleTransaction: true,
		Quick:             true,
		Triggers:          true,
		Routines:          true,
		Events:            true,
		ExtendedInsert:    true,
	}

	// Initialize maps for backup types and servers
	data.BackupTypeOptions = make(map[string]database.MySQLDumpOptions)
	data.ServerOptions = make(map[string]database.MySQLDumpOptions)

	// Add backup types
	for typeName := range config.CFG.BackupTypes {
		data.BackupTypeOptions[typeName] = database.MySQLDumpOptions{}
	}

	// Add servers
	for _, server := range config.CFG.DatabaseServers {
		data.ServerOptions[server.Name] = database.MySQLDumpOptions{}
	}

	data.LastUpdated = time.Now()

	// Render the template
	renderTemplate(w, tmpl, "/mysql-options", PageData{
		Title:       "MySQL Dump Options",
		Description: "Configure MySQL dump options for backups",
		Content:     data,
	})
}
