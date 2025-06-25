package adminserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/supporttools/GoSQLGuard/pkg/config"
)

// TestRunBackupHandler_Validation tests the validation logic of the backup handler
func TestRunBackupHandler_Validation(t *testing.T) {
	// Setup config
	config.CFG = config.AppConfig{
		BackupTypes: map[string]config.BackupTypeConfig{
			"daily":  {},
			"weekly": {},
		},
		DatabaseServers: []config.DatabaseServerConfig{
			{Name: "server1", Type: "mysql"},
			{Name: "server2", Type: "postgresql"},
		},
	}

	// Create server without scheduler to test validation only
	server := &Server{
		scheduler: nil,
	}

	tests := []struct {
		name           string
		method         string
		query          string
		expectedStatus int
	}{
		{
			name:           "Invalid method",
			method:         "GET",
			query:          "?type=daily",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Missing type parameter",
			method:         "POST",
			query:          "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid backup type",
			method:         "POST",
			query:          "?type=invalid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid server",
			method:         "POST",
			query:          "?type=daily&server=invalid",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "No scheduler configured",
			method:         "POST",
			query:          "?type=daily",
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset task running state
			taskLock.Lock()
			isTaskRunning = false
			taskLock.Unlock()

			// Create request
			req, err := http.NewRequest(tt.method, "/api/backups/run"+tt.query, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Record response
			rr := httptest.NewRecorder()
			server.runBackupHandler(rr, req)

			// Check status code
			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.expectedStatus)
			}
		})
	}
}

// TestRunRetentionHandler_Validation tests the validation logic of the retention handler
func TestRunRetentionHandler_Validation(t *testing.T) {
	// Create server without scheduler
	server := &Server{
		scheduler: nil,
	}

	tests := []struct {
		name           string
		method         string
		expectedStatus int
	}{
		{
			name:           "Invalid method",
			method:         "GET",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "No scheduler configured",
			method:         "POST",
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset task running state
			taskLock.Lock()
			isTaskRunning = false
			taskLock.Unlock()

			// Create request
			req, err := http.NewRequest(tt.method, "/api/retention/run", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Record response
			rr := httptest.NewRecorder()
			server.runRetentionHandler(rr, req)

			// Check status code
			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.expectedStatus)
			}
		})
	}
}

// TestHealthCheck tests the health check endpoint
func TestHealthCheck(t *testing.T) {
	server := &Server{}

	req, err := http.NewRequest("GET", "/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	server.healthCheckHandler(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check response
	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status='healthy', got %v", response["status"])
	}

	if response["time"] == "" {
		t.Errorf("Expected time field to be present")
	}
}

// TestStatsHandler tests the stats endpoint
func TestStatsHandler(t *testing.T) {
	// Skip if metadata store is not initialized
	t.Skip("Skipping stats handler test - requires metadata store initialization")
}
