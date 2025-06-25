package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/supporttools/GoSQLGuard/pkg/config"
)

func TestS3ConfigHandler_GetS3Config(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		S3: config.S3Config{
			Enabled:            true,
			Region:             "us-east-1",
			Bucket:             "test-bucket",
			Prefix:             "backups",
			Endpoint:           "https://s3.amazonaws.com",
			AccessKey:          "test-access-key",
			SecretKey:          "test-secret-key",
			UseSSL:             true,
			SkipCertValidation: false,
		},
	}

	handler := NewS3ConfigHandler(cfg, nil)

	// Create request
	req, err := http.NewRequest("GET", "/api/s3", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Record response
	rr := httptest.NewRecorder()
	handler.handleS3Config(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check response body
	var response S3Response
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	if !response.Success {
		t.Errorf("Expected success=true, got false")
	}

	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Errorf("Expected data to be a map")
	}

	if data["enabled"] != true {
		t.Errorf("Expected enabled=true, got %v", data["enabled"])
	}

	if data["bucket"] != "test-bucket" {
		t.Errorf("Expected bucket=test-bucket, got %v", data["bucket"])
	}
}

func TestS3ConfigHandler_UpdateS3Config(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		S3: config.S3Config{},
		MetadataDB: config.MetadataDBConfig{
			Enabled: false, // Disable DB for testing
		},
	}

	handler := NewS3ConfigHandler(cfg, nil)

	// Create request body
	reqBody := S3ConfigRequest{
		Enabled:         true,
		Region:          "us-west-2",
		Bucket:          "new-bucket",
		Prefix:          "new-prefix",
		Endpoint:        "https://minio.local",
		AccessKeyID:     "new-access-key",
		SecretAccessKey: "new-secret-key",
		UseSSL:          true,
		InsecureSSL:     false,
	}

	body, _ := json.Marshal(reqBody)

	// Create request
	req, err := http.NewRequest("PUT", "/api/s3", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Record response
	rr := httptest.NewRecorder()
	handler.handleS3Config(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check that config was updated
	if cfg.S3.Bucket != "new-bucket" {
		t.Errorf("Expected bucket to be updated to new-bucket, got %v", cfg.S3.Bucket)
	}

	if cfg.S3.Region != "us-west-2" {
		t.Errorf("Expected region to be updated to us-west-2, got %v", cfg.S3.Region)
	}

	if cfg.S3.AccessKey != "new-access-key" {
		t.Errorf("Expected access key to be updated, got %v", cfg.S3.AccessKey)
	}
}

func TestS3ConfigHandler_TestConnection(t *testing.T) {
	// Setup
	handler := NewS3ConfigHandler(&config.AppConfig{}, nil)

	// Create request body
	reqBody := S3TestRequest{
		Region:          "us-east-1",
		Bucket:          "test-bucket",
		Endpoint:        "", // Use AWS
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
		UseSSL:          true,
		InsecureSSL:     false,
	}

	body, _ := json.Marshal(reqBody)

	// Create request
	req, err := http.NewRequest("POST", "/api/s3/test", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Record response
	rr := httptest.NewRecorder()
	handler.handleS3Test(rr, req)

	// Check status code (should be OK even if connection fails - error is in response)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check response structure
	var response S3Response
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	// Note: In real test, this might fail due to invalid credentials
	// We're just testing the endpoint structure
}

func TestS3ConfigHandler_InvalidMethod(t *testing.T) {
	handler := NewS3ConfigHandler(&config.AppConfig{}, nil)

	// Test invalid method on main endpoint
	req, _ := http.NewRequest("DELETE", "/api/s3", nil)
	rr := httptest.NewRecorder()
	handler.handleS3Config(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code for DELETE: got %v want %v",
			status, http.StatusMethodNotAllowed)
	}

	// Test invalid method on test endpoint
	req, _ = http.NewRequest("GET", "/api/s3/test", nil)
	rr = httptest.NewRecorder()
	handler.handleS3Test(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code for GET on test: got %v want %v",
			status, http.StatusMethodNotAllowed)
	}
}

func TestS3ConfigHandler_InvalidJSON(t *testing.T) {
	handler := NewS3ConfigHandler(&config.AppConfig{}, nil)

	// Create request with invalid JSON
	req, _ := http.NewRequest("PUT", "/api/s3", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.handleS3Config(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for invalid JSON: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Check error response
	var response S3Response
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse error response: %v", err)
	}

	if response.Success {
		t.Errorf("Expected success=false for invalid JSON")
	}
}
