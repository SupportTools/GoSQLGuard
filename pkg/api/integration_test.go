// +build integration

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/database"
)

// This file contains integration tests that demonstrate how to use all API endpoints
// Run with: go test -tags=integration ./pkg/api

func TestAPIIntegration_CompleteWorkflow(t *testing.T) {
	// Setup configuration
	cfg := &config.AppConfig{
		MetadataDB: config.MetadataDBConfig{
			Enabled: false, // Disable DB for testing
		},
		DatabaseServers: []config.DatabaseServerConfig{},
		BackupTypes:     map[string]config.BackupTypeConfig{},
	}
	
	// Initialize handlers
	s3Handler := NewS3ConfigHandler(cfg, nil)
	mysqlOptionsHandler := NewMySQLOptionsHandler(cfg, nil)
	postgresqlOptionsHandler := NewPostgreSQLOptionsHandler(cfg, nil)
	
	// Step 1: Configure S3 storage
	t.Run("Configure S3 Storage", func(t *testing.T) {
		s3Config := S3ConfigRequest{
			Enabled:         true,
			Region:          "us-east-1",
			Bucket:          "my-backup-bucket",
			Prefix:          "database-backups",
			Endpoint:        "https://s3.amazonaws.com",
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			UseSSL:          true,
			InsecureSSL:     false,
		}
		
		body, _ := json.Marshal(s3Config)
		req := httptest.NewRequest("PUT", "/api/s3", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		
		s3Handler.handleS3Config(rr, req)
		
		if rr.Code != http.StatusOK {
			t.Errorf("Failed to configure S3: %v", rr.Body.String())
		}
		
		// Verify configuration was saved
		if cfg.S3.Bucket != "my-backup-bucket" {
			t.Errorf("S3 bucket not configured correctly")
		}
	})
	
	// Step 2: Configure global MySQL options
	t.Run("Configure Global MySQL Options", func(t *testing.T) {
		mysqlOptions := MySQLOptionsRequest{
			Global: &database.MySQLDumpOptions{
				SingleTransaction: true,
				Quick:             true,
				SkipLockTables:    true,
				ExtendedInsert:    true,
				Compress:          true,
				Triggers:          true,
				Routines:          true,
				Events:            true,
			},
		}
		
		body, _ := json.Marshal(mysqlOptions)
		req := httptest.NewRequest("PUT", "/api/mysql-options", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		
		mysqlOptionsHandler.handleMySQLOptions(rr, req)
		
		if rr.Code != http.StatusOK {
			t.Errorf("Failed to configure MySQL options: %v", rr.Body.String())
		}
	})
	
	// Step 3: Configure global PostgreSQL options
	t.Run("Configure Global PostgreSQL Options", func(t *testing.T) {
		pgOptions := PostgreSQLOptionsRequest{
			Global: &database.PostgreSQLDumpOptions{
				Format:       "custom",
				Verbose:      false,
				NoOwner:      true,
				NoPrivileges: true,
				Jobs:         4,
				Compress:     6,
			},
		}
		
		body, _ := json.Marshal(pgOptions)
		req := httptest.NewRequest("PUT", "/api/postgresql-options", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		
		postgresqlOptionsHandler.handlePostgreSQLOptions(rr, req)
		
		if rr.Code != http.StatusOK {
			t.Errorf("Failed to configure PostgreSQL options: %v", rr.Body.String())
		}
	})
	
	// Step 4: Test S3 connection (this would fail with fake credentials)
	t.Run("Test S3 Connection", func(t *testing.T) {
		testReq := S3TestRequest{
			Region:          "us-east-1",
			Bucket:          "test-bucket",
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
			UseSSL:          true,
		}
		
		body, _ := json.Marshal(testReq)
		req := httptest.NewRequest("POST", "/api/s3/test", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		
		s3Handler.handleS3Test(rr, req)
		
		// We expect this to succeed (returns 200) but with an error message
		if rr.Code != http.StatusOK {
			t.Errorf("S3 test endpoint returned unexpected status: %v", rr.Code)
		}
		
		var response S3Response
		json.Unmarshal(rr.Body.Bytes(), &response)
		// In real scenario, this would fail due to invalid credentials
		t.Logf("S3 test result: %v", response.Message)
	})
}

// Example of how to test server connection endpoints
func ExampleServerConnectionTest() {
	handler := &ServerHandler{}
	
	// Test MySQL connection
	mysqlServer := serverRequest{
		Type:     "mysql",
		Host:     "localhost",
		Port:     "3306",
		Username: "root",
		Password: "password",
	}
	
	body, _ := json.Marshal(mysqlServer)
	req := httptest.NewRequest("POST", "/api/servers/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	
	handler.handleTestConnection(rr, req)
	
	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)
	
	if response["status"] == "success" {
		fmt.Printf("Connected successfully! Found databases: %v\n", response["databases"])
	} else {
		fmt.Printf("Connection failed: %v\n", response["message"])
	}
}

// Example of how to configure per-server MySQL options
func ExamplePerServerMySQLOptions() {
	cfg := &config.AppConfig{
		DatabaseServers: []config.DatabaseServerConfig{
			{
				Name: "production-mysql",
				Type: "mysql",
			},
		},
		MetadataDB: config.MetadataDBConfig{
			Enabled: false,
		},
	}
	
	handler := NewMySQLOptionsHandler(cfg, nil)
	
	// Configure specific options for production server
	options := database.MySQLDumpOptions{
		SingleTransaction:  true,
		Quick:              true,
		SkipLockTables:     true,
		ExtendedInsert:     false, // Disable for better readability
		Compress:           true,
		Triggers:           true,
		Routines:           true,
		Events:             true,
		CustomOptions:      []string{"--hex-blob", "--skip-tz-utc"},
	}
	
	body, _ := json.Marshal(options)
	req := httptest.NewRequest("PUT", "/api/mysql-options/server?server=production-mysql", 
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	
	handler.handleServerMySQLOptions(rr, req)
	
	if rr.Code == http.StatusOK {
		fmt.Println("MySQL options configured for production-mysql server")
	}
}