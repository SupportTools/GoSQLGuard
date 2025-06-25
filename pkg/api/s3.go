package api

import (
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	_ "github.com/go-sql-driver/mysql" // MySQL driver for database connections
	"github.com/sirupsen/logrus"
	"github.com/supporttools/GoSQLGuard/pkg/config"
)

// S3ConfigHandler handles S3 storage configuration API endpoints
type S3ConfigHandler struct {
	Config *config.AppConfig
	Logger *logrus.Logger
}

// S3ConfigRequest represents a request to configure S3 storage settings
type S3ConfigRequest struct {
	Enabled         bool   `json:"enabled"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	Prefix          string `json:"prefix"`
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"access_key"`
	SecretAccessKey string `json:"secret_key"`
	UseSSL          bool   `json:"use_ssl"`
	InsecureSSL     bool   `json:"insecure_ssl"`
}

// S3TestRequest represents a request to test S3 connectivity
type S3TestRequest struct {
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"access_key"`
	SecretAccessKey string `json:"secret_key"`
	UseSSL          bool   `json:"use_ssl"`
	InsecureSSL     bool   `json:"insecure_ssl"`
}

// S3Response represents the response for S3 operations
type S3Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewS3ConfigHandler creates a new handler for S3 configuration endpoints
func NewS3ConfigHandler(cfg *config.AppConfig, logger *logrus.Logger) *S3ConfigHandler {
	return &S3ConfigHandler{
		Config: cfg,
		Logger: logger,
	}
}

// RegisterRoutes registers the S3 configuration API routes
func (h *S3ConfigHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/s3", h.handleS3Config)
	mux.HandleFunc("/api/s3/test", h.handleS3Test)
}

func (h *S3ConfigHandler) handleS3Config(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getS3Config(w, r)
	case http.MethodPut, http.MethodPost:
		h.updateS3Config(w, r)
	default:
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *S3ConfigHandler) getS3Config(w http.ResponseWriter, r *http.Request) {
	response := S3Response{
		Success: true,
		Data: map[string]interface{}{
			"enabled":              h.Config.S3.Enabled,
			"region":               h.Config.S3.Region,
			"bucket":               h.Config.S3.Bucket,
			"prefix":               h.Config.S3.Prefix,
			"endpoint":             h.Config.S3.Endpoint,
			"access_key_id":        h.Config.S3.AccessKey,
			"use_ssl":              h.Config.S3.UseSSL,
			"skip_cert_validation": h.Config.S3.SkipCertValidation,
		},
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *S3ConfigHandler) updateS3Config(w http.ResponseWriter, r *http.Request) {
	var req S3ConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Update configuration
	h.Config.S3.Enabled = req.Enabled
	h.Config.S3.Region = req.Region
	h.Config.S3.Bucket = req.Bucket
	h.Config.S3.Prefix = req.Prefix
	h.Config.S3.Endpoint = req.Endpoint
	h.Config.S3.AccessKey = req.AccessKeyID
	if req.SecretAccessKey != "" {
		h.Config.S3.SecretKey = req.SecretAccessKey
	}
	h.Config.S3.UseSSL = req.UseSSL
	h.Config.S3.SkipCertValidation = req.InsecureSSL

	// Save configuration if using MySQL config
	if os.Getenv("CONFIG_SOURCE") == "mysql" {
		if err := h.saveS3ConfigToMySQL(); err != nil {
			h.sendError(w, fmt.Sprintf("Failed to save S3 configuration: %v", err), http.StatusInternalServerError)
			return
		}
	}

	response := S3Response{
		Success: true,
		Message: "S3 configuration updated successfully",
		Data:    req,
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *S3ConfigHandler) handleS3Test(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req S3TestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Debug log the request
	if h.Logger != nil {
		h.Logger.Debugf("S3 test request: Region=%s, Bucket=%s, Endpoint=%s, AccessKey=%s, UseSSL=%v",
			req.Region, req.Bucket, req.Endpoint, req.AccessKeyID, req.UseSSL)
	}
	// Also use standard log
	log.Printf("S3 test request decoded - Region=%s, Bucket=%s, Endpoint=%s, AccessKeyID=%s (length=%d), SecretAccessKey=****** (length=%d), UseSSL=%v",
		req.Region, req.Bucket, req.Endpoint, req.AccessKeyID, len(req.AccessKeyID), len(req.SecretAccessKey), req.UseSSL)

	// Debug: print the raw JSON
	rawJSON, _ := json.Marshal(req)
	log.Printf("S3 test request struct as JSON: %s", string(rawJSON))

	// Test S3 connection
	if err := h.testS3Connection(req); err != nil {
		h.sendError(w, fmt.Sprintf("S3 connection test failed: %v", err), http.StatusOK)
		return
	}

	response := S3Response{
		Success: true,
		Message: "S3 connection test successful",
	}

	h.sendJSON(w, response, http.StatusOK)
}

func (h *S3ConfigHandler) testS3Connection(req S3TestRequest) error {
	// Debug log credentials
	log.Printf("Creating AWS credentials with AccessKey: %s (length=%d), SecretKey: ****** (length=%d)",
		req.AccessKeyID, len(req.AccessKeyID), len(req.SecretAccessKey))

	// Create AWS config
	awsConfig := &aws.Config{
		Region:           aws.String(req.Region),
		Credentials:      credentials.NewStaticCredentials(req.AccessKeyID, req.SecretAccessKey, ""),
		S3ForcePathStyle: aws.Bool(true),
	}

	if req.Endpoint != "" {
		awsConfig.Endpoint = aws.String(req.Endpoint)
		awsConfig.DisableSSL = aws.Bool(!req.UseSSL)

		// For HTTPS endpoints, we might need a custom HTTP client
		if req.UseSSL {
			// Create a custom HTTP client with TLS configuration
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion:         tls.VersionTLS12,
					InsecureSkipVerify: req.InsecureSSL, // #nosec G402 - Only skip if explicitly requested by user
				},
			}
			httpClient := &http.Client{
				Transport: tr,
				Timeout:   30 * time.Second,
			}
			awsConfig.HTTPClient = httpClient
			if req.InsecureSSL {
				log.Printf("WARNING: Using InsecureSkipVerify=true for endpoint: %s", req.Endpoint)
			}
		}
	}

	// Create session
	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return fmt.Errorf("failed to create AWS session: %w", err)
	}

	// Create S3 client
	svc := s3.New(sess)

	// Test by checking if bucket exists
	_, err = svc.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(req.Bucket),
	})

	if err != nil {
		// Try to list objects as a fallback test
		listInput := &s3.ListObjectsV2Input{
			Bucket:  aws.String(req.Bucket),
			MaxKeys: aws.Int64(1),
		}
		_, err = svc.ListObjectsV2(listInput)
		if err != nil {
			return fmt.Errorf("failed to access bucket: %w", err)
		}
	}

	return nil
}

func (h *S3ConfigHandler) saveS3ConfigToMySQL() error {
	// Connect to the config database directly
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true",
		os.Getenv("CONFIG_MYSQL_USER"),
		os.Getenv("CONFIG_MYSQL_PASSWORD"),
		os.Getenv("CONFIG_MYSQL_HOST"),
		os.Getenv("CONFIG_MYSQL_PORT"),
		os.Getenv("CONFIG_MYSQL_DATABASE"))

	if os.Getenv("CONFIG_MYSQL_USER") == "" {
		// Use defaults
		dsn = "gosqlguard:config_password@tcp(config-mysql:3306)/gosqlguard_config?charset=utf8mb4&parseTime=true"
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to config database: %w", err)
	}
	defer db.Close()

	query := `
		INSERT INTO storage_configs (name, type, config, enabled, created_at, updated_at)
		VALUES ('s3-primary', 's3', ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			config = VALUES(config),
			enabled = VALUES(enabled),
			updated_at = NOW()
	`

	configJSON, err := json.Marshal(map[string]interface{}{
		"enabled":              h.Config.S3.Enabled,
		"region":               h.Config.S3.Region,
		"bucket":               h.Config.S3.Bucket,
		"prefix":               h.Config.S3.Prefix,
		"endpoint":             h.Config.S3.Endpoint,
		"access_key":           h.Config.S3.AccessKey,
		"secret_key":           h.Config.S3.SecretKey,
		"use_ssl":              h.Config.S3.UseSSL,
		"skip_cert_validation": h.Config.S3.SkipCertValidation,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal S3 config: %w", err)
	}

	_, err = db.Exec(query, string(configJSON), h.Config.S3.Enabled)
	if err != nil {
		return fmt.Errorf("failed to save S3 config to database: %w", err)
	}

	// Increment config version to trigger reload
	_, err = db.Exec("UPDATE config_versions SET version = version + 1, updated_at = NOW() WHERE active = TRUE")
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warnf("Failed to increment config version: %v", err)
		}
	}

	return nil
}

func (h *S3ConfigHandler) sendJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		if h.Logger != nil {
			h.Logger.Errorf("Failed to encode JSON response: %v", err)
		}
	}
}

func (h *S3ConfigHandler) sendError(w http.ResponseWriter, message string, status int) {
	response := S3Response{
		Success: false,
		Message: message,
	}
	h.sendJSON(w, response, status)
}
