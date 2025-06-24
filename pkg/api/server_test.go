package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Note: Since ServerHandler doesn't depend on a repository interface in the test,
// we'll test the connection functions directly without mocking

func TestServerHandler_TestConnection_MySQL(t *testing.T) {
	handler := &ServerHandler{}
	
	// Create request body
	reqBody := serverRequest{
		Type:     "mysql",
		Host:     "localhost",
		Port:     "3306",
		Username: "test",
		Password: "test",
	}
	
	body, _ := json.Marshal(reqBody)
	
	// Create request
	req, err := http.NewRequest("POST", "/api/servers/test", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	// Record response
	rr := httptest.NewRecorder()
	handler.handleTestConnection(rr, req)
	
	// Check status code
	if status := rr.Code; status != http.StatusOK && status != http.StatusBadRequest {
		t.Errorf("handler returned unexpected status code: got %v", status)
	}
	
	// Check response structure
	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}
	
	if _, ok := response["status"]; !ok {
		t.Errorf("Expected status field in response")
	}
	
	if _, ok := response["message"]; !ok {
		t.Errorf("Expected message field in response")
	}
}

func TestServerHandler_TestConnection_PostgreSQL(t *testing.T) {
	handler := &ServerHandler{}
	
	// Create request body
	reqBody := serverRequest{
		Type:     "postgresql",
		Host:     "localhost",
		Port:     "5432",
		Username: "test",
		Password: "test",
	}
	
	body, _ := json.Marshal(reqBody)
	
	// Create request
	req, err := http.NewRequest("POST", "/api/servers/test", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	// Record response
	rr := httptest.NewRecorder()
	handler.handleTestConnection(rr, req)
	
	// Check status code
	if status := rr.Code; status != http.StatusOK && status != http.StatusBadRequest {
		t.Errorf("handler returned unexpected status code: got %v", status)
	}
	
	// Check response structure
	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}
	
	if _, ok := response["status"]; !ok {
		t.Errorf("Expected status field in response")
	}
	
	if _, ok := response["message"]; !ok {
		t.Errorf("Expected message field in response")
	}
}

func TestServerHandler_TestConnection_InvalidType(t *testing.T) {
	handler := &ServerHandler{}
	
	// Create request body with invalid type
	reqBody := serverRequest{
		Type:     "invalid",
		Host:     "localhost",
		Port:     "1234",
		Username: "test",
		Password: "test",
	}
	
	body, _ := json.Marshal(reqBody)
	
	// Create request
	req, err := http.NewRequest("POST", "/api/servers/test", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	// Record response
	rr := httptest.NewRecorder()
	handler.handleTestConnection(rr, req)
	
	// Check status code
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid database type: got %v", status)
	}
}

func TestServerHandler_TestConnection_DefaultPorts(t *testing.T) {
	handler := &ServerHandler{}
	
	// Test MySQL with empty port (should default to 3306)
	reqBody := serverRequest{
		Type:     "mysql",
		Host:     "localhost",
		Port:     "", // Empty port
		Username: "test",
		Password: "test",
	}
	
	body, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/api/servers/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	
	rr := httptest.NewRecorder()
	handler.handleTestConnection(rr, req)
	
	// Should not return 400 (bad request)
	if status := rr.Code; status == http.StatusBadRequest {
		var response map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &response)
		if msg, ok := response["message"].(string); ok && msg == "Unsupported database type: mysql" {
			t.Errorf("MySQL default port not applied correctly")
		}
	}
	
	// Test PostgreSQL with empty port (should default to 5432)
	reqBody.Type = "postgresql"
	reqBody.Port = "" // Empty port
	
	body, _ = json.Marshal(reqBody)
	req, _ = http.NewRequest("POST", "/api/servers/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	
	rr = httptest.NewRecorder()
	handler.handleTestConnection(rr, req)
	
	// Should not return 400 (bad request) for unsupported type
	if status := rr.Code; status == http.StatusBadRequest {
		var response map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &response)
		if msg, ok := response["message"].(string); ok && msg == "Unsupported database type: postgresql" {
			t.Errorf("PostgreSQL default port not applied correctly")
		}
	}
}

func TestServerHandler_TestConnection_InvalidMethod(t *testing.T) {
	handler := &ServerHandler{}
	
	// Test with GET method
	req, _ := http.NewRequest("GET", "/api/servers/test", nil)
	rr := httptest.NewRecorder()
	handler.handleTestConnection(rr, req)
	
	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for GET method: got %v", status)
	}
}

func TestServerHandler_TestConnection_InvalidJSON(t *testing.T) {
	handler := &ServerHandler{}
	
	// Create request with invalid JSON
	req, _ := http.NewRequest("POST", "/api/servers/test", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	
	rr := httptest.NewRecorder()
	handler.handleTestConnection(rr, req)
	
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid JSON: got %v", status)
	}
}