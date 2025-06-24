package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/database"
)

func TestPostgreSQLOptionsHandler_GetOptions(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		PostgreSQLDumpOptions: config.PostgreSQLDumpOptionsConfig{
			Format:       "custom",
			Verbose:      false,
			NoOwner:      true,
			NoPrivileges: true,
			Compress:     6,
		},
		DatabaseServers: []config.DatabaseServerConfig{
			{
				Name: "postgres-server-1",
				Type: "postgresql",
				PostgreSQLDumpOptions: config.PostgreSQLDumpOptionsConfig{
					Format:   "plain",
					Verbose:  true,
					Compress: 9,
				},
			},
			{
				Name: "mysql-server-1",
				Type: "mysql",
			},
		},
	}
	
	handler := NewPostgreSQLOptionsHandler(cfg, nil)
	
	// Create request
	req, err := http.NewRequest("GET", "/api/postgresql-options", nil)
	if err != nil {
		t.Fatal(err)
	}
	
	// Record response
	rr := httptest.NewRecorder()
	handler.handlePostgreSQLOptions(rr, req)
	
	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	// Check response body
	var response PostgreSQLOptionsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}
	
	if !response.Success {
		t.Errorf("Expected success=true, got false")
	}
	
	// Check global options
	if response.Global == nil {
		t.Errorf("Expected global options to be present")
	} else {
		if response.Global.Format != "custom" {
			t.Errorf("Expected global Format=custom, got %s", response.Global.Format)
		}
		if !response.Global.NoOwner {
			t.Errorf("Expected global NoOwner=true")
		}
		if response.Global.Compress != 6 {
			t.Errorf("Expected global Compress=6, got %d", response.Global.Compress)
		}
	}
	
	// Check per-server options
	if len(response.PerServer) != 1 {
		t.Errorf("Expected 1 server with PostgreSQL options, got %d", len(response.PerServer))
	}
	
	if serverOpts, ok := response.PerServer["postgres-server-1"]; ok {
		if serverOpts.Format != "plain" {
			t.Errorf("Expected server Format=plain, got %s", serverOpts.Format)
		}
		if !serverOpts.Verbose {
			t.Errorf("Expected server Verbose=true")
		}
	} else {
		t.Errorf("Expected postgres-server-1 in per-server options")
	}
}

func TestPostgreSQLOptionsHandler_UpdateGlobalOptions(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		PostgreSQLDumpOptions: config.PostgreSQLDumpOptionsConfig{},
		MetadataDB: config.MetadataDBConfig{
			Enabled: false, // Disable DB for testing
		},
	}
	
	handler := NewPostgreSQLOptionsHandler(cfg, nil)
	
	// Create request body
	reqBody := PostgreSQLOptionsRequest{
		Global: &database.PostgreSQLDumpOptions{
			Format:       "custom",
			Verbose:      true,
			NoComments:   true,
			Blobs:        true,
			NoOwner:      true,
			NoPrivileges: true,
			Jobs:         4,
			Compress:     9,
		},
	}
	
	body, _ := json.Marshal(reqBody)
	
	// Create request
	req, err := http.NewRequest("PUT", "/api/postgresql-options", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	// Record response
	rr := httptest.NewRecorder()
	handler.handlePostgreSQLOptions(rr, req)
	
	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	// Check that config was updated
	if cfg.PostgreSQLDumpOptions.Format != "custom" {
		t.Errorf("Expected Format to be updated to custom")
	}
	
	if !cfg.PostgreSQLDumpOptions.Verbose {
		t.Errorf("Expected Verbose to be updated to true")
	}
	
	if cfg.PostgreSQLDumpOptions.Jobs != 4 {
		t.Errorf("Expected Jobs to be updated to 4, got %d", cfg.PostgreSQLDumpOptions.Jobs)
	}
}

func TestPostgreSQLOptionsHandler_ServerSpecificOptions(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		DatabaseServers: []config.DatabaseServerConfig{
			{
				Name: "postgres-server-1",
				Type: "postgresql",
				PostgreSQLDumpOptions: config.PostgreSQLDumpOptionsConfig{},
			},
		},
		MetadataDB: config.MetadataDBConfig{
			Enabled: false,
		},
	}
	
	handler := NewPostgreSQLOptionsHandler(cfg, nil)
	
	// Test GET server options
	req, _ := http.NewRequest("GET", "/api/postgresql-options/server?server=postgres-server-1", nil)
	rr := httptest.NewRecorder()
	handler.handleServerPostgreSQLOptions(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("GET server options returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	// Test UPDATE server options
	updateBody := database.PostgreSQLDumpOptions{
		Format:   "tar",
		Verbose:  true,
		Jobs:     2,
		Compress: 9,
	}
	
	body, _ := json.Marshal(updateBody)
	req, _ = http.NewRequest("PUT", "/api/postgresql-options/server?server=postgres-server-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	
	rr = httptest.NewRecorder()
	handler.handleServerPostgreSQLOptions(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("UPDATE server options returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	
	// Check that server options were updated
	if cfg.DatabaseServers[0].PostgreSQLDumpOptions.Format != "tar" {
		t.Errorf("Expected server Format to be updated to tar")
	}
	
	if cfg.DatabaseServers[0].PostgreSQLDumpOptions.Jobs != 2 {
		t.Errorf("Expected server Jobs to be updated to 2")
	}
	
	// Test DELETE server options
	req, _ = http.NewRequest("DELETE", "/api/postgresql-options/server?server=postgres-server-1", nil)
	rr = httptest.NewRecorder()
	handler.handleServerPostgreSQLOptions(rr, req)
	
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("DELETE server options returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestPostgreSQLOptionsHandler_InvalidFormat(t *testing.T) {
	cfg := &config.AppConfig{
		PostgreSQLDumpOptions: config.PostgreSQLDumpOptionsConfig{},
		MetadataDB: config.MetadataDBConfig{
			Enabled: false,
		},
	}
	
	handler := NewPostgreSQLOptionsHandler(cfg, nil)
	
	// Test with various format values (all should be accepted)
	formats := []string{"plain", "custom", "directory", "tar"}
	
	for _, format := range formats {
		reqBody := PostgreSQLOptionsRequest{
			Global: &database.PostgreSQLDumpOptions{
				Format: format,
			},
		}
		
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("PUT", "/api/postgresql-options", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		
		rr := httptest.NewRecorder()
		handler.handlePostgreSQLOptions(rr, req)
		
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected 200 for format %s: got %v", format, status)
		}
		
		if cfg.PostgreSQLDumpOptions.Format != format {
			t.Errorf("Expected format to be %s, got %s", format, cfg.PostgreSQLDumpOptions.Format)
		}
	}
}

func TestPostgreSQLOptionsHandler_NonPostgreSQLServer(t *testing.T) {
	cfg := &config.AppConfig{
		DatabaseServers: []config.DatabaseServerConfig{
			{
				Name: "mysql-server",
				Type: "mysql",
			},
		},
	}
	
	handler := NewPostgreSQLOptionsHandler(cfg, nil)
	
	// Test GET options for non-PostgreSQL server
	req, _ := http.NewRequest("GET", "/api/postgresql-options/server?server=mysql-server", nil)
	rr := httptest.NewRecorder()
	handler.handleServerPostgreSQLOptions(rr, req)
	
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected 400 for non-PostgreSQL server: got %v", status)
	}
}

func TestPostgreSQLOptionsHandler_CompressionLevels(t *testing.T) {
	cfg := &config.AppConfig{
		PostgreSQLDumpOptions: config.PostgreSQLDumpOptionsConfig{},
		MetadataDB: config.MetadataDBConfig{
			Enabled: false,
		},
	}
	
	handler := NewPostgreSQLOptionsHandler(cfg, nil)
	
	// Test valid compression levels (0-9)
	validLevels := []int{0, 1, 5, 9}
	
	for _, level := range validLevels {
		reqBody := PostgreSQLOptionsRequest{
			Global: &database.PostgreSQLDumpOptions{
				Compress: level,
			},
		}
		
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequest("PUT", "/api/postgresql-options", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		
		rr := httptest.NewRecorder()
		handler.handlePostgreSQLOptions(rr, req)
		
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Expected 200 for compression level %d: got %v", level, status)
		}
		
		if cfg.PostgreSQLDumpOptions.Compress != level {
			t.Errorf("Expected compression level to be %d, got %d", level, cfg.PostgreSQLDumpOptions.Compress)
		}
	}
}