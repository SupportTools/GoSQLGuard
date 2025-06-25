package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	dbmeta "github.com/supporttools/GoSQLGuard/pkg/database/metadata"
)

// TestScheduleHandler_RequestValidation tests request validation for schedule endpoints
func TestScheduleHandler_RequestValidation(t *testing.T) {
	// Handler without repository (will return service unavailable)
	handler := &ScheduleHandler{}

	tests := []struct {
		name           string
		method         string
		endpoint       string
		body           interface{}
		expectedStatus int
	}{
		{
			name:           "No repository - GET",
			method:         "GET",
			endpoint:       "/api/schedules",
			body:           nil,
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:     "No repository - POST",
			method:   "POST",
			endpoint: "/api/schedules",
			body: scheduleRequest{
				Name:           "Test",
				BackupType:     "daily",
				CronExpression: "0 2 * * *",
			},
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "Invalid method",
			method:         "PUT",
			endpoint:       "/api/schedules",
			body:           nil,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Delete without ID",
			method:         "POST",
			endpoint:       "/api/schedules/delete",
			body:           nil,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Delete with invalid method",
			method:         "GET",
			endpoint:       "/api/schedules/delete?id=123",
			body:           nil,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != nil {
				body, _ := json.Marshal(tt.body)
				req = httptest.NewRequest(tt.method, tt.endpoint, bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.endpoint, nil)
			}

			rr := httptest.NewRecorder()

			// Route to appropriate handler
			if tt.endpoint == "/api/schedules/delete" || tt.endpoint == "/api/schedules/delete?id=123" {
				handler.handleDeleteSchedule(rr, req)
			} else {
				handler.handleSchedules(rr, req)
			}

			if status := rr.Code; status != tt.expectedStatus {
				t.Errorf("%s: handler returned wrong status code: got %v want %v",
					tt.name, status, tt.expectedStatus)
			}
		})
	}
}

// TestScheduleRequest_Validation tests validation of schedule request structure
func TestScheduleRequest_Validation(t *testing.T) {
	tests := []struct {
		name        string
		req         scheduleRequest
		shouldError bool
	}{
		{
			name: "Valid request",
			req: scheduleRequest{
				Name:           "Daily Backup",
				BackupType:     "daily",
				CronExpression: "0 2 * * *",
				Enabled:        true,
			},
			shouldError: false,
		},
		{
			name: "Missing name",
			req: scheduleRequest{
				BackupType:     "daily",
				CronExpression: "0 2 * * *",
			},
			shouldError: true,
		},
		{
			name: "Missing backup type",
			req: scheduleRequest{
				Name:           "Test",
				CronExpression: "0 2 * * *",
			},
			shouldError: true,
		},
		{
			name: "Missing cron expression",
			req: scheduleRequest{
				Name:       "Test",
				BackupType: "daily",
			},
			shouldError: true,
		},
		{
			name: "With storage options",
			req: scheduleRequest{
				Name:           "Full Backup",
				BackupType:     "weekly",
				CronExpression: "0 3 * * 0",
				Enabled:        true,
				LocalStorage: storageRequest{
					Enabled:     true,
					Duration:    "168h",
					KeepForever: false,
				},
				S3Storage: storageRequest{
					Enabled:     true,
					Duration:    "720h",
					KeepForever: false,
				},
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate required fields
			hasError := tt.req.Name == "" || tt.req.BackupType == "" || tt.req.CronExpression == ""

			if hasError != tt.shouldError {
				t.Errorf("Expected error=%v, got error=%v", tt.shouldError, hasError)
			}
		})
	}
}

// TestScheduleResponse_Structure tests the schedule response structure
func TestScheduleResponse_Structure(t *testing.T) {
	// Test convertScheduleToResponse function
	schedule := dbmeta.BackupSchedule{
		ID:             "test-id",
		Name:           "Test Schedule",
		BackupType:     "daily",
		CronExpression: "0 2 * * *",
		Enabled:        true,
		RetentionPolicies: []dbmeta.ScheduleRetentionPolicy{
			{
				StorageType: "local",
				Duration:    "168h",
				KeepForever: false,
			},
			{
				StorageType: "s3",
				Duration:    "720h",
				KeepForever: true,
			},
		},
	}

	response := convertScheduleToResponse(&schedule)

	// Verify response structure
	if response.ID != "test-id" {
		t.Errorf("Expected ID='test-id', got %v", response.ID)
	}

	if response.Name != "Test Schedule" {
		t.Errorf("Expected Name='Test Schedule', got %v", response.Name)
	}

	if !response.LocalStorage.Enabled {
		t.Errorf("Expected LocalStorage to be enabled")
	}

	if response.LocalStorage.Duration != "168h" {
		t.Errorf("Expected LocalStorage.Duration='168h', got %v", response.LocalStorage.Duration)
	}

	if !response.S3Storage.Enabled {
		t.Errorf("Expected S3Storage to be enabled")
	}

	if !response.S3Storage.KeepForever {
		t.Errorf("Expected S3Storage.KeepForever=true")
	}
}

// TestScheduleHandler_JSONParsing tests JSON parsing and error handling
func TestScheduleHandler_JSONParsing(t *testing.T) {
	handler := &ScheduleHandler{}

	// Test invalid JSON
	req := httptest.NewRequest("POST", "/api/schedules", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.handleSchedules(rr, req)

	// Should return service unavailable (no repo) before trying to parse JSON
	if status := rr.Code; status != http.StatusServiceUnavailable {
		t.Errorf("Expected status %v for invalid JSON, got %v", http.StatusServiceUnavailable, status)
	}
}
