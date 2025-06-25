// Package s3 handles S3 storage operations for MySQL backups.
package s3

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/metrics"
)

// Client represents an S3 client
type Client struct {
	s3Client *s3.Client
	cfg      *config.AppConfig
}

// NewClient creates a new S3 client
func NewClient() (*Client, error) {
	if !config.CFG.S3.Enabled {
		return nil, fmt.Errorf("S3 storage is not enabled in configuration")
	}

	s3Client, err := getS3Client()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	return &Client{
		s3Client: s3Client,
		cfg:      &config.CFG,
	}, nil
}

// getS3Client initializes and returns an S3 client based on configuration
func getS3Client() (*s3.Client, error) {
	ctx := context.Background()

	// Create custom HTTP client with TLS configuration
	httpClient := &http.Client{}

	// Configure TLS settings if needed
	if config.CFG.S3.UseSSL {
		tlsConfig := &tls.Config{}

		// Load custom CA if specified
		if config.CFG.S3.CustomCAPath != "" && !config.CFG.S3.SkipCertValidation {
			rootCAs, _ := x509.SystemCertPool()
			if rootCAs == nil {
				rootCAs = x509.NewCertPool()
			}

			// Read the custom CA certificate
			caCert, err := os.ReadFile(config.CFG.S3.CustomCAPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read custom CA certificate: %w", err)
			}

			// Add the custom CA to the cert pool
			if ok := rootCAs.AppendCertsFromPEM(caCert); !ok {
				return nil, fmt.Errorf("failed to append custom CA certificate")
			}

			tlsConfig.RootCAs = rootCAs
			log.Printf("Using custom CA certificate from %s", config.CFG.S3.CustomCAPath)
		}

		// Skip certificate validation if specified
		if config.CFG.S3.SkipCertValidation {
			tlsConfig.InsecureSkipVerify = true
			log.Printf("Warning: TLS certificate validation is disabled for S3 connections")
		}

		// Set up the custom transport with our TLS config
		transport := &http.Transport{
			TLSClientConfig: tlsConfig,
		}
		httpClient.Transport = transport
	}

	// Set up common AWS SDK options
	sdkOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.CFG.S3.AccessKey, config.CFG.S3.SecretKey, "",
		)),
		awsconfig.WithHTTPClient(httpClient),
	}

	if config.CFG.S3.Endpoint != "" {
		// Custom S3-compatible storage
		// Debug logging for environment variables
		if config.CFG.Debug {
			log.Println("S3 Debug: Environment variables:")
			log.Printf("  AWS_REGION=%s", config.CFG.S3.Region)
			log.Printf("  AWS_ENDPOINT_URL=%s", config.CFG.S3.Endpoint)
			log.Printf("  AWS_S3_FORCE_PATH_STYLE=%v", config.CFG.S3.PathStyle)
		}

		// For custom endpoints, we'll configure the S3 client options directly
		// The endpoint will be set when creating the S3 client
	} else {
		// Standard AWS S3 - add region
		sdkOptions = append(sdkOptions, awsconfig.WithRegion(config.CFG.S3.Region))
	}

	// Create AWS config with all options
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, sdkOptions...)
	if err != nil {
		return nil, fmt.Errorf("AWS SDK config initialization error: %w", err)
	}

	// Create S3 client with custom options
	s3Options := []func(*s3.Options){
		func(o *s3.Options) {
			// Force path-style URLs (bucket name in path, not hostname)
			o.UsePathStyle = true
		},
	}

	// Add custom endpoint if configured
	if config.CFG.S3.Endpoint != "" {
		s3Options = append(s3Options, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(config.CFG.S3.Endpoint)
		})
	}

	s3Client := s3.NewFromConfig(awsCfg, s3Options...)

	return s3Client, nil
}

// UploadBackupWithKey uploads a backup file to S3 with the provided full object key
func (c *Client) UploadBackupWithKey(backupPath, objectKey string) error {
	startTime := time.Now()

	// Extract backup type and database from the object key for metrics
	// Assume format like: prefix/by-server/servername/daily/db-timestamp.sql.gz
	// or: prefix/by-type/daily/servername_db-timestamp.sql.gz
	parts := strings.Split(objectKey, "/")
	var backupType, database string

	// Try to get the backup type - usually 3rd or 4th part depending on prefix
	if len(parts) >= 3 {
		// Handle by-server or by-type formats
		if parts[len(parts)-3] == "by-server" && len(parts) >= 4 {
			backupType = parts[len(parts)-2]
		} else if parts[len(parts)-2] == "by-type" {
			backupType = parts[len(parts)-1]
		}
	}

	// Try to extract database name from filename
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		// Format is usually db-timestamp.sql.gz or servername_db-timestamp.sql.gz
		filenameParts := strings.Split(filename, "-")
		if len(filenameParts) > 0 {
			dbPart := filenameParts[0]
			// If using by-type format, remove server prefix
			if strings.Contains(dbPart, "_") {
				dbPart = strings.Split(dbPart, "_")[1]
			}
			database = dbPart
		}
	}

	if config.CFG.Debug {
		log.Printf("S3 Debug: Starting upload of file %s to key %s", backupPath, objectKey)
	}

	// Open file for reading
	file, err := os.Open(backupPath)
	if err != nil {
		metrics.S3UploadCount.WithLabelValues(backupType, database, "error").Inc()
		log.Printf("S3 Debug: Error opening file for upload: %v", err)
		return fmt.Errorf("failed to open backup file for S3 upload: %w", err)
	}
	defer file.Close()

	// Get file size for logging
	fileInfo, err := os.Stat(backupPath)
	if err == nil && config.CFG.Debug {
		log.Printf("S3 Debug: Uploading file of size %.2f MB", float64(fileInfo.Size())/(1024*1024))
	}

	if config.CFG.Debug {
		log.Printf("S3 Debug: Uploading to bucket=%s key=%s", c.cfg.S3.Bucket, objectKey)
	}

	// Upload to S3
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	putInput := &s3.PutObjectInput{
		Bucket: aws.String(c.cfg.S3.Bucket),
		Key:    aws.String(objectKey),
		Body:   file,
	}

	if config.CFG.Debug {
		log.Printf("S3 Debug: PutObject input: bucket=%s, key=%s",
			*putInput.Bucket, *putInput.Key)
	}

	_, err = c.s3Client.PutObject(ctx, putInput)

	if err != nil {
		metrics.S3UploadCount.WithLabelValues(backupType, database, "error").Inc()

		// Detailed error logging
		log.Printf("S3 Debug: Error during upload: %v", err)
		log.Printf("S3 Debug: Error type: %T", err)

		// Try to unwrap AWS errors for more details
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			log.Printf("S3 Debug: URL error: %v, URL: %v, Op: %v",
				urlErr.Err, urlErr.URL, urlErr.Op)
		}

		return fmt.Errorf("failed to upload backup to S3: %w", err)
	}

	// Record metrics
	duration := time.Since(startTime)
	metrics.S3UploadDuration.WithLabelValues(backupType, database).Observe(duration.Seconds())
	metrics.S3UploadCount.WithLabelValues(backupType, database, "success").Inc()

	// Get file size for metrics if we don't have it already
	if fileInfo == nil {
		fileInfo, err = os.Stat(backupPath)
	}

	if err == nil && fileInfo != nil {
		sizeBytes := float64(fileInfo.Size())
		metrics.BackupSize.WithLabelValues(backupType, database, "s3").Set(sizeBytes)
	}

	log.Printf("Successfully uploaded backup to S3: s3://%s/%s", c.cfg.S3.Bucket, objectKey)
	return nil
}

// UploadBackup uploads a backup file to S3
func (c *Client) UploadBackup(backupPath, backupType, database, backupFileName string) error {
	startTime := time.Now()

	if config.CFG.Debug {
		log.Printf("S3 Debug: Starting upload of file %s", backupPath)
	}

	// Open file for reading
	file, err := os.Open(backupPath)
	if err != nil {
		metrics.S3UploadCount.WithLabelValues(backupType, database, "error").Inc()
		log.Printf("S3 Debug: Error opening file for upload: %v", err)
		return fmt.Errorf("failed to open backup file for S3 upload: %w", err)
	}
	defer file.Close()

	// Get file size for logging
	fileInfo, err := os.Stat(backupPath)
	if err == nil && config.CFG.Debug {
		log.Printf("S3 Debug: Uploading file of size %.2f MB", float64(fileInfo.Size())/(1024*1024))
	}

	// Create the S3 object key
	objectKey := buildObjectKey(c.cfg.S3.Prefix, backupType, backupFileName)

	if config.CFG.Debug {
		log.Printf("S3 Debug: Uploading to bucket=%s key=%s", c.cfg.S3.Bucket, objectKey)
	}

	// Upload to S3
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	putInput := &s3.PutObjectInput{
		Bucket: aws.String(c.cfg.S3.Bucket),
		Key:    aws.String(objectKey),
		Body:   file,
	}

	if config.CFG.Debug {
		log.Printf("S3 Debug: PutObject input: bucket=%s, key=%s",
			*putInput.Bucket, *putInput.Key)
	}

	_, err = c.s3Client.PutObject(ctx, putInput)

	if err != nil {
		metrics.S3UploadCount.WithLabelValues(backupType, database, "error").Inc()

		// Detailed error logging
		log.Printf("S3 Debug: Error during upload: %v", err)
		log.Printf("S3 Debug: Error type: %T", err)

		// Try to unwrap AWS errors for more details
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			log.Printf("S3 Debug: URL error: %v, URL: %v, Op: %v",
				urlErr.Err, urlErr.URL, urlErr.Op)
		}

		return fmt.Errorf("failed to upload backup to S3: %w", err)
	}

	// Record metrics
	duration := time.Since(startTime)
	metrics.S3UploadDuration.WithLabelValues(backupType, database).Observe(duration.Seconds())
	metrics.S3UploadCount.WithLabelValues(backupType, database, "success").Inc()

	// Get file size for metrics if we don't have it already
	if fileInfo == nil {
		fileInfo, err = os.Stat(backupPath)
	}

	if err == nil && fileInfo != nil {
		sizeBytes := float64(fileInfo.Size())
		metrics.BackupSize.WithLabelValues(backupType, database, "s3").Set(sizeBytes)
	}

	log.Printf("Successfully uploaded backup to S3: s3://%s/%s", c.cfg.S3.Bucket, objectKey)
	return nil
}

// EnforceRetention implements retention policy for S3 backups
func (c *Client) EnforceRetention() error {
	for backupType, typeConfig := range c.cfg.BackupTypes {
		// Skip if S3 backup is not enabled for this type
		if !typeConfig.S3.Enabled {
			if c.cfg.Debug {
				log.Printf("S3 backup not enabled for %s, skipping retention enforcement", backupType)
			}
			continue
		}

		// Skip if keep forever is set
		if typeConfig.S3.Retention.Forever {
			if c.cfg.Debug {
				log.Printf("S3 backups for %s set to keep forever, skipping retention enforcement", backupType)
			}
			continue
		}

		// Parse duration string
		duration, err := time.ParseDuration(typeConfig.S3.Retention.Duration)
		if err != nil {
			log.Printf("Invalid duration for %s S3 retention: %v", backupType, err)
			continue
		}

		// List objects with prefix
		prefix := buildPrefix(c.cfg.S3.Prefix, backupType)

		ctx := context.Background()
		paginator := s3.NewListObjectsV2Paginator(c.s3Client, &s3.ListObjectsV2Input{
			Bucket: aws.String(c.cfg.S3.Bucket),
			Prefix: aws.String(prefix),
		})

		expirationTime := time.Now().Add(-duration)

		// Process each page of results
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				log.Printf("Failed to list S3 objects: %v", err)
				break
			}

			for _, obj := range page.Contents {
				// Check if object is older than retention period
				if obj.LastModified.Before(expirationTime) {
					_, err := c.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
						Bucket: aws.String(c.cfg.S3.Bucket),
						Key:    obj.Key,
					})

					if err != nil {
						log.Printf("Failed to delete expired S3 backup %s: %v", *obj.Key, err)
					} else {
						// Look for corresponding metadata entry
						backups := metadata.DefaultStore.GetBackupsFiltered("", "", backupType, true)
						for _, backup := range backups {
							if backup.S3Key == *obj.Key || strings.HasSuffix(backup.S3Key, *obj.Key) {
								if err := metadata.DefaultStore.MarkBackupDeleted(backup.ID); err != nil {
									log.Printf("Warning: Failed to mark backup %s as deleted in metadata: %v", backup.ID, err)
								} else {
									log.Printf("Marked backup %s as deleted in metadata", backup.ID)
								}
								break
							}
						}

						log.Printf("Removed expired S3 backup: %s", *obj.Key)
						metrics.BackupRetentionDeletes.WithLabelValues(backupType, "s3").Inc()
					}
				}
			}
		}
	}
	return nil
}

// Helper function to build consistent S3 object keys
func buildObjectKey(prefix, backupType, backupFileName string) string {
	if prefix != "" {
		return fmt.Sprintf("%s/%s/%s",
			strings.TrimSuffix(prefix, "/"),
			backupType,
			backupFileName)
	}
	return fmt.Sprintf("%s/%s", backupType, backupFileName)
}

// Helper function to build prefix for listing objects
func buildPrefix(prefix, backupType string) string {
	if prefix != "" {
		return fmt.Sprintf("%s/%s/",
			strings.TrimSuffix(prefix, "/"),
			backupType)
	}
	return fmt.Sprintf("%s/", backupType)
}
