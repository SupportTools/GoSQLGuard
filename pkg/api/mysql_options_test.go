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

func TestMySQLOptionsHandler_GetOptions(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		MySQLDumpOptions: config.MySQLDumpOptionsConfig{
			SingleTransaction: true,
			Quick:             true,
			SkipLockTables:    false,
			ExtendedInsert:    true,
		},
		DatabaseServers: []config.DatabaseServerConfig{
			{
				Name: "mysql-server-1",
				Type: "mysql",
				MySQLDumpOptions: config.MySQLDumpOptionsConfig{
					SingleTransaction: false,
					Quick:             true,
					Compress:          true,
				},
			},
			{
				Name: "postgres-server-1",
				Type: "postgresql",
			},
		},
	}

	handler := NewMySQLOptionsHandler(cfg, nil)

	// Create request
	req, err := http.NewRequest("GET", "/api/mysql-options", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Record response
	rr := httptest.NewRecorder()
	handler.handleMySQLOptions(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check response body
	var response MySQLOptionsResponse
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
		if !response.Global.SingleTransaction {
			t.Errorf("Expected global SingleTransaction=true")
		}
		if !response.Global.ExtendedInsert {
			t.Errorf("Expected global ExtendedInsert=true")
		}
	}

	// Check per-server options
	if len(response.PerServer) != 1 {
		t.Errorf("Expected 1 server with MySQL options, got %d", len(response.PerServer))
	}

	if serverOpts, ok := response.PerServer["mysql-server-1"]; ok {
		if serverOpts.SingleTransaction {
			t.Errorf("Expected server SingleTransaction=false")
		}
		if !serverOpts.Compress {
			t.Errorf("Expected server Compress=true")
		}
	} else {
		t.Errorf("Expected mysql-server-1 in per-server options")
	}
}

func TestMySQLOptionsHandler_UpdateGlobalOptions(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		MySQLDumpOptions: config.MySQLDumpOptionsConfig{},
		MetadataDB: config.MetadataDBConfig{
			Enabled: false, // Disable DB for testing
		},
	}

	handler := NewMySQLOptionsHandler(cfg, nil)

	// Create request body
	reqBody := MySQLOptionsRequest{
		Global: &database.MySQLDumpOptions{
			SingleTransaction: true,
			Quick:             true,
			SkipComments:      true,
			Compress:          true,
			Triggers:          true,
			Routines:          true,
			Events:            true,
		},
	}

	body, _ := json.Marshal(reqBody)

	// Create request
	req, err := http.NewRequest("PUT", "/api/mysql-options", bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Record response
	rr := httptest.NewRecorder()
	handler.handleMySQLOptions(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check that config was updated
	if !cfg.MySQLDumpOptions.SingleTransaction {
		t.Errorf("Expected SingleTransaction to be updated to true")
	}

	if !cfg.MySQLDumpOptions.SkipComments {
		t.Errorf("Expected SkipComments to be updated to true")
	}

	if !cfg.MySQLDumpOptions.Compress {
		t.Errorf("Expected Compress to be updated to true")
	}
}

func TestMySQLOptionsHandler_ServerSpecificOptions(t *testing.T) {
	// Setup
	cfg := &config.AppConfig{
		DatabaseServers: []config.DatabaseServerConfig{
			{
				Name:             "mysql-server-1",
				Type:             "mysql",
				MySQLDumpOptions: config.MySQLDumpOptionsConfig{},
			},
		},
		MetadataDB: config.MetadataDBConfig{
			Enabled: false,
		},
	}

	handler := NewMySQLOptionsHandler(cfg, nil)

	// Test GET server options
	req, _ := http.NewRequest("GET", "/api/mysql-options/server?server=mysql-server-1", nil)
	rr := httptest.NewRecorder()
	handler.handleServerMySQLOptions(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("GET server options returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Test UPDATE server options
	updateBody := database.MySQLDumpOptions{
		SingleTransaction: true,
		Quick:             true,
		Compress:          true,
	}

	body, _ := json.Marshal(updateBody)
	req, _ = http.NewRequest("PUT", "/api/mysql-options/server?server=mysql-server-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr = httptest.NewRecorder()
	handler.handleServerMySQLOptions(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("UPDATE server options returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check that server options were updated
	if !cfg.DatabaseServers[0].MySQLDumpOptions.SingleTransaction {
		t.Errorf("Expected server SingleTransaction to be updated to true")
	}

	// Test DELETE server options
	req, _ = http.NewRequest("DELETE", "/api/mysql-options/server?server=mysql-server-1", nil)
	rr = httptest.NewRecorder()
	handler.handleServerMySQLOptions(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("DELETE server options returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestMySQLOptionsHandler_ServerNotFound(t *testing.T) {
	cfg := &config.AppConfig{
		DatabaseServers: []config.DatabaseServerConfig{},
	}

	handler := NewMySQLOptionsHandler(cfg, nil)

	// Test GET non-existent server
	req, _ := http.NewRequest("GET", "/api/mysql-options/server?server=non-existent", nil)
	rr := httptest.NewRecorder()
	handler.handleServerMySQLOptions(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Expected 404 for non-existent server: got %v", status)
	}
}

func TestMySQLOptionsHandler_NonMySQLServer(t *testing.T) {
	cfg := &config.AppConfig{
		DatabaseServers: []config.DatabaseServerConfig{
			{
				Name: "postgres-server",
				Type: "postgresql",
			},
		},
	}

	handler := NewMySQLOptionsHandler(cfg, nil)

	// Test GET options for non-MySQL server
	req, _ := http.NewRequest("GET", "/api/mysql-options/server?server=postgres-server", nil)
	rr := httptest.NewRecorder()
	handler.handleServerMySQLOptions(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected 400 for non-MySQL server: got %v", status)
	}
}

func TestMySQLOptionsHandler_MissingServerName(t *testing.T) {
	handler := NewMySQLOptionsHandler(&config.AppConfig{}, nil)

	// Test without server parameter
	req, _ := http.NewRequest("GET", "/api/mysql-options/server", nil)
	rr := httptest.NewRecorder()
	handler.handleServerMySQLOptions(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing server name: got %v", status)
	}
}
