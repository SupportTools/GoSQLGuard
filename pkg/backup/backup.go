// Package backup implements MySQL backup operations.
package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/supporttools/GoSQLGuard/pkg/backup/database/mysql"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/database/common"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/metrics"
	"github.com/supporttools/GoSQLGuard/pkg/storage/local"
	"github.com/supporttools/GoSQLGuard/pkg/storage/s3"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

const (
	// backupTypeKey is the context key for backup type
	backupTypeKey contextKey = "backupType"
)

// BackupOptions defines options for a backup operation
type BackupOptions struct {
	Servers   []string // List of server names to back up, empty means all servers
	Databases []string // List of databases to back up, empty means all databases
}

// Manager handles backup operations
type Manager struct {
	cfg        *config.AppConfig
	localStore *local.Client
	s3Store    *s3.Client
}

// constructBackupPaths creates the paths for backup files in both by-server and by-type organizations
func constructBackupPaths(serverName, backupType, dbName, timestamp string) (map[string]string, map[string]string) {
	localPaths := make(map[string]string)
	s3Keys := make(map[string]string)

	// Base file name (without server prefix)
	filename := fmt.Sprintf("%s-%s.sql.gz", dbName, timestamp)

	// Server-prefixed filename for by-type organization
	serverPrefixedFilename := fmt.Sprintf("%s_%s", serverName, filename)

	// Local paths for combined organization
	if config.CFG.Local.Enabled {
		// By-server organization
		localPaths["by-server"] = filepath.Join(
			config.CFG.Local.BackupDirectory,
			"by-server",
			serverName,
			backupType,
			filename,
		)

		// By-type organization
		localPaths["by-type"] = filepath.Join(
			config.CFG.Local.BackupDirectory,
			"by-type",
			backupType,
			serverPrefixedFilename,
		)
	}

	// S3 keys for combined organization
	if config.CFG.S3.Enabled {
		prefix := config.CFG.S3.Prefix
		if prefix != "" && !strings.HasSuffix(prefix, "/") {
			prefix = prefix + "/"
		}

		// By-server organization
		s3Keys["by-server"] = fmt.Sprintf(
			"%sby-server/%s/%s/%s",
			prefix,
			serverName,
			backupType,
			filename,
		)

		// By-type organization
		s3Keys["by-type"] = fmt.Sprintf(
			"%sby-type/%s/%s",
			prefix,
			backupType,
			serverPrefixedFilename,
		)
	}

	return localPaths, s3Keys
}

// NewManager creates a new backup manager
func NewManager() (*Manager, error) {
	manager := &Manager{
		cfg: &config.CFG,
	}

	// Initialize local storage client if enabled
	if config.CFG.Local.Enabled {
		localClient, err := local.NewClient()
		if err != nil {
			log.Printf("Warning: Failed to initialize local storage: %v", err)
		} else {
			manager.localStore = localClient
		}
	}

	// Initialize S3 storage client if enabled
	if config.CFG.S3.Enabled {
		s3Client, err := s3.NewClient()
		if err != nil {
			log.Printf("Warning: Failed to initialize S3 storage: %v", err)
		} else {
			manager.s3Store = s3Client
		}
	}

	return manager, nil
}

// PerformBackup executes a backup operation for the specified type
// If options.Servers is provided, only back up those servers
// If options.Databases is provided, only back up those databases
func (m *Manager) PerformBackup(backupType string, options ...BackupOptions) error {
	// Process optional parameters
	var opts BackupOptions
	if len(options) > 0 {
		opts = options[0]
	}

	// Convert server list to a lookup map for faster filtering
	serverFilter := make(map[string]bool)
	if len(opts.Servers) > 0 {
		for _, server := range opts.Servers {
			serverFilter[server] = true
		}
		log.Printf("Filtering backup to specific servers: %v", opts.Servers)
	}

	// Convert database list to a lookup map for faster filtering
	dbFilter := make(map[string]bool)
	if len(opts.Databases) > 0 {
		for _, db := range opts.Databases {
			dbFilter[db] = true
		}
		log.Printf("Filtering backup to specific databases: %v", opts.Databases)
	}
	// Check if this backup type is configured
	typeConfig, exists := m.cfg.BackupTypes[backupType]
	if !exists {
		return fmt.Errorf("no configuration found for backup type: %s", backupType)
	}

	// Check if this backup type is enabled for local storage
	localBackupEnabled := m.cfg.Local.Enabled && typeConfig.Local.Enabled && m.localStore != nil

	// Skip if neither local nor S3 backup is enabled for this type
	s3BackupEnabled := m.cfg.S3.Enabled && typeConfig.S3.Enabled && m.s3Store != nil
	if !localBackupEnabled && !s3BackupEnabled {
		return fmt.Errorf("backup type %s is not enabled for any storage destination", backupType)
	}

	// For backward compatibility, check if we're using legacy MySQL config
	if len(m.cfg.DatabaseServers) == 0 && m.cfg.MySQL.Host != "" {
		// Process the legacy MySQL server using default name
		log.Println("Using legacy MySQL configuration as default server")

		// Get appropriate database provider to allow us to query the MySQL server
		dbProvider, err := getActiveDatabaseProvider()
		if err != nil {
			return fmt.Errorf("failed to get database provider: %w", err)
		}

		// Get databases to backup
		var databases []string

		// If includeDatabases is set, use only those
		if len(m.cfg.MySQL.IncludeDatabases) > 0 {
			databases = m.cfg.MySQL.IncludeDatabases
		} else {
			// We need to actually query the database server
			log.Println("No included databases specified, attempting to query database server...")

			// Get all databases from the MySQL server
			allDatabases, err := queryAllDatabases(dbProvider)
			if err != nil {
				log.Printf("Error querying database server: %v", err)
				return fmt.Errorf("failed to query database server: %w", err)
			}

			// Apply exclude list filtering if one exists
			if len(m.cfg.MySQL.ExcludeDatabases) > 0 {
				log.Printf("Filtering %d databases against exclude list (%d entries)",
					len(allDatabases), len(m.cfg.MySQL.ExcludeDatabases))

				// Build a map for faster lookups
				excludeMap := make(map[string]bool)
				for _, db := range m.cfg.MySQL.ExcludeDatabases {
					excludeMap[db] = true
				}

				// Filter the databases
				for _, db := range allDatabases {
					if !excludeMap[db] {
						databases = append(databases, db)
					}
				}

				log.Printf("Found %d databases to back up after applying exclude filters", len(databases))
			} else {
				// No exclude list, use all the databases
				databases = allDatabases
				log.Printf("Found %d databases to back up", len(databases))
			}

			// If after filtering we have no databases, warn the user
			if len(databases) == 0 {
				log.Println("Warning: No databases to back up after filtering. Please check your include/exclude configuration.")
				return fmt.Errorf("no databases available to back up")
			}
		}

		// Process each database
		for _, database := range databases {
			if err := m.backupDatabase("default", "mysql", database, backupType, typeConfig); err != nil {
				log.Printf("Failed to backup database %s: %v", database, err)
				continue
			}
		}

		return nil
	}

	// Using multi-server configuration
	for _, server := range m.cfg.DatabaseServers {
		log.Printf("Processing server: %s (%s)", server.Name, server.Type)

		// Get databases to backup for this server
		var databases []string

		if len(server.IncludeDatabases) > 0 {
			// Use the explicitly included databases
			databases = server.IncludeDatabases
			log.Printf("Using explicitly configured databases for server %s: %v", server.Name, databases)
		} else {
			// We need to query the server for available databases
			log.Printf("No included databases specified for server %s, querying server...", server.Name)

			switch server.Type {
			case "mysql":
				// Connect to MySQL server to list databases
				portNum, _ := strconv.Atoi(server.Port)
				if portNum == 0 {
					portNum = 3306 // Default MySQL port
				}

				provider := &mysql.Provider{
					Host:     server.Host,
					Port:     portNum,
					User:     server.Username,
					Password: server.Password,
				}

				// Connect to the server
				if err := provider.Connect(context.Background()); err != nil {
					log.Printf("Error connecting to MySQL server %s: %v", server.Name, err)
					continue
				}

				// List all databases
				allDatabases, err := provider.ListDatabases(context.Background())
				if err != nil {
					log.Printf("Error listing databases on server %s: %v", server.Name, err)
					provider.Close()
					continue
				}

				// Close the connection
				provider.Close()

				// Apply exclude list filtering if one exists
				if len(server.ExcludeDatabases) > 0 {
					log.Printf("Filtering %d databases against exclude list (%d entries) for server %s",
						len(allDatabases), len(server.ExcludeDatabases), server.Name)

					// Build a map for faster lookups
					excludeMap := make(map[string]bool)
					for _, db := range server.ExcludeDatabases {
						excludeMap[db] = true
					}

					// Filter the databases
					for _, db := range allDatabases {
						if !excludeMap[db] {
							databases = append(databases, db)
						}
					}

					log.Printf("Found %d databases to back up for server %s after applying exclude filters",
						len(databases), server.Name)
				} else {
					// No exclude list, use all the databases
					databases = allDatabases
					log.Printf("Found %d databases to back up for server %s", len(databases), server.Name)
				}

			case "postgresql":
				// TODO: Add PostgreSQL database listing
				log.Printf("PostgreSQL support not yet implemented for server %s", server.Name)
				continue

			default:
				log.Printf("Unsupported database type '%s' for server %s", server.Type, server.Name)
				continue
			}

			// If after filtering we have no databases, warn the user
			if len(databases) == 0 {
				log.Printf("Warning: No databases to back up for server %s after filtering. Check configuration.",
					server.Name)
				continue
			}
		}

		// Process each database for this server
		for _, database := range databases {
			if err := m.backupDatabase(server.Name, server.Type, database, backupType, typeConfig); err != nil {
				log.Printf("Failed to backup database %s on server %s: %v", database, server.Name, err)
				continue
			}
		}
	}

	return nil
}

// createLogFile creates a log file for a backup operation
func (m *Manager) createLogFile(id string) (string, *os.File, error) {
	// Determine log directory
	var logDir string
	if m.cfg.Local.Enabled {
		// Use logs subdirectory under backup directory
		logDir = filepath.Join(m.cfg.Local.BackupDirectory, "logs")
	} else {
		// Create temp directory for logs
		tempDir, err := os.MkdirTemp("", "gosqlguard-logs")
		if err != nil {
			return "", nil, fmt.Errorf("failed to create temp log directory: %w", err)
		}
		logDir = tempDir
	}

	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file name based on backup ID
	logFilePath := filepath.Join(logDir, fmt.Sprintf("%s.log", id))

	// Create log file
	logFile, err := os.Create(logFilePath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create log file: %w", err)
	}

	return logFilePath, logFile, nil
}

// backupDatabase handles the backup process for a single database
func (m *Manager) backupDatabase(serverName, serverType, database, backupType string, typeConfig config.BackupTypeConfig) error {
	startTime := time.Now()
	timestamp := startTime.Format("2006-01-02-15-04-05")

	// Create the backup paths using the combined organization strategy
	localPaths, s3Keys := constructBackupPaths(serverName, backupType, database, timestamp)

	// Create metadata entry for this backup
	meta := metadata.DefaultStore.CreateBackupMeta(serverName, serverType, database, backupType)

	// Create log file for this backup
	logFilePath, logFile, err := m.createLogFile(meta.ID)
	if err != nil {
		log.Printf("Warning: Failed to create log file: %v", err)
		// Continue without log file
	} else {
		defer logFile.Close()

		// Update metadata with log file path
		err = metadata.DefaultStore.UpdateLogFilePath(meta.ID, logFilePath)
		if err != nil {
			log.Printf("Warning: Failed to update log file path in metadata: %v", err)
		}

		// Write header to log file
		fmt.Fprintf(logFile, "Backup started at: %s\n", startTime.Format(time.RFC3339))
		fmt.Fprintf(logFile, "Server: %s\n", serverName)
		fmt.Fprintf(logFile, "Database: %s\n", database)
		fmt.Fprintf(logFile, "Backup type: %s\n", backupType)
		fmt.Fprintf(logFile, "Backup ID: %s\n\n", meta.ID)
		fmt.Fprintf(logFile, "--- Command output ---\n\n")
	}

	// Determine which backup path to use as primary path for the backup
	var primaryBackupPath string
	var tempDir string

	// Local backup setup
	localBackupEnabled := m.cfg.Local.Enabled && typeConfig.Local.Enabled && m.localStore != nil

	if localBackupEnabled {
		// Use the by-server path as the primary path
		primaryBackupPath = localPaths["by-server"]

		// Ensure the directory exists
		err := os.MkdirAll(filepath.Dir(primaryBackupPath), 0755)
		if err != nil {
			errMsg := fmt.Sprintf("failed to create backup directory: %v", err)
			if logFile != nil {
				fmt.Fprintf(logFile, "ERROR: %s\n", errMsg)
			}
			metrics.BackupCount.WithLabelValues(backupType, database, "error").Inc()
			metadata.DefaultStore.UpdateBackupStatus(meta.ID, metadata.StatusError, map[string]string{}, 0, errMsg)
			return fmt.Errorf("failed to create backup directory: %w", err)
		}

		if logFile != nil {
			fmt.Fprintf(logFile, "Backup paths:\n")
			for org, path := range localPaths {
				fmt.Fprintf(logFile, "  %s: %s\n", org, path)
			}
			fmt.Fprintf(logFile, "\n")
		}
	} else {
		// Create temp directory for S3-only backups
		tempDir, err = os.MkdirTemp("", "mysql-backup")
		if err != nil {
			errMsg := fmt.Sprintf("failed to create temp directory: %v", err)
			if logFile != nil {
				fmt.Fprintf(logFile, "ERROR: %s\n", errMsg)
			}
			metrics.BackupCount.WithLabelValues(backupType, database, "error").Inc()
			metadata.DefaultStore.UpdateBackupStatus(meta.ID, metadata.StatusError, map[string]string{}, 0, errMsg)
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer os.RemoveAll(tempDir)

		// Create a filename for the temp file
		filename := fmt.Sprintf("%s-%s.sql.gz", database, timestamp)
		primaryBackupPath = filepath.Join(tempDir, filename)

		if logFile != nil {
			fmt.Fprintf(logFile, "Temp backup path: %s\n\n", primaryBackupPath)
		}
	}

	// Get the appropriate database provider based on server type
	var provider common.Provider

	switch serverType {
	case "mysql":
		// Create a MySQL provider to handle the dump with configured options
		var portNum int

		// Find the appropriate server config for connection details
		var serverConfig *config.DatabaseServerConfig
		for i, s := range m.cfg.DatabaseServers {
			if s.Name == serverName {
				serverConfig = &m.cfg.DatabaseServers[i]
				break
			}
		}

		// If we can't find the server config, use legacy config (for default server)
		if serverConfig == nil {
			// Using the legacy config
			portNum, _ = strconv.Atoi(m.cfg.MySQL.Port)
			if portNum == 0 {
				portNum = 3306 // Default MySQL port
			}

			mysqlProvider := &mysql.Provider{
				Host:     m.cfg.MySQL.Host,
				Port:     portNum,
				User:     m.cfg.MySQL.Username,
				Password: m.cfg.MySQL.Password,
			}
			provider = mysqlProvider
		} else {
			// Using the server-specific config
			portNum, _ = strconv.Atoi(serverConfig.Port)
			if portNum == 0 {
				portNum = 3306 // Default MySQL port
			}

			mysqlProvider := &mysql.Provider{
				Host:     serverConfig.Host,
				Port:     portNum,
				User:     serverConfig.Username,
				Password: serverConfig.Password,
			}
			provider = mysqlProvider
		}

	case "postgresql":
		// TODO: Add PostgreSQL provider creation
		errMsg := "PostgreSQL support not yet implemented"
		if logFile != nil {
			fmt.Fprintf(logFile, "ERROR: %s\n", errMsg)
		}
		metrics.BackupCount.WithLabelValues(backupType, database, "error").Inc()
		metadata.DefaultStore.UpdateBackupStatus(meta.ID, metadata.StatusError, map[string]string{}, 0, errMsg)
		return errors.New(errMsg)

	default:
		errMsg := fmt.Sprintf("unsupported database type: %s", serverType)
		if logFile != nil {
			fmt.Fprintf(logFile, "ERROR: %s\n", errMsg)
		}
		metrics.BackupCount.WithLabelValues(backupType, database, "error").Inc()
		metadata.DefaultStore.UpdateBackupStatus(meta.ID, metadata.StatusError, map[string]string{}, 0, errMsg)
		return fmt.Errorf("unsupported database type: %s", serverType)
	}

	// Create context with backup type
	ctx := context.WithValue(context.Background(), backupTypeKey, backupType)

	// Define backup options
	backupOpts := common.BackupOptions{
		TransactionMode: true, // Ensure consistent backup
		IncludeSchema:   true,
		SchemaOnly:      false,
	}

	// Log the command that will be executed
	if logFile != nil {
		// Create a sanitized version of the command for logging

		// Start with a simplified command description to avoid character-by-character masking
		var maskedCmd string

		// Extract the important parts of the command to show in logs
		// This avoids the complete command string which may be getting over-sanitized
		if serverType == "mysql" {
			// For MySQL, construct a simplified representation of the command
			host := ""
			port := ""

			// Get the correct host/port based on server configuration
			if serverName == "default" && m.cfg.MySQL.Host != "" {
				host = m.cfg.MySQL.Host
				port = m.cfg.MySQL.Port
			} else {
				// Find the server in the config
				for _, s := range m.cfg.DatabaseServers {
					if s.Name == serverName {
						host = s.Host
						port = s.Port
						break
					}
				}
			}

			// Build a clean description of the command
			maskedCmd = fmt.Sprintf("mysqldump -h %s -P %s -u <user> -p<masked> %s",
				host, port, database)

			// Add the key flags being used
			var flags []string

			// Add hardcoded MySQL dump options used by provider
			flags = append(flags, "--single-transaction")
			flags = append(flags, "--quick")
			flags = append(flags, "--triggers")
			flags = append(flags, "--routines")
			flags = append(flags, "--events")
			flags = append(flags, "--set-gtid-purged=OFF")

			// Add the flags to the command description
			if len(flags) > 0 {
				maskedCmd += " " + strings.Join(flags, " ")
			}

			// Write a clean log entry that won't be character-by-character masked
			fmt.Fprintf(logFile, "Running command: %s | gzip > %s\n\n", maskedCmd, primaryBackupPath)

			// Extra details about the options being used (for better troubleshooting)
			fmt.Fprintf(logFile, "MySQL hardcoded options applied:\n")
			fmt.Fprintf(logFile, "- Single transaction: true\n")
			fmt.Fprintf(logFile, "- Quick: true\n") 
			fmt.Fprintf(logFile, "- Triggers: true\n")
			fmt.Fprintf(logFile, "- Routines: true\n")
			fmt.Fprintf(logFile, "- Events: true\n")
			fmt.Fprintf(logFile, "- Set GTID purged: OFF\n\n")
		} else {
			// For other database types, just use a generic description
			maskedCmd = fmt.Sprintf("Backing up %s database: %s", serverType, database)
			fmt.Fprintf(logFile, "Running command: %s\n\n", maskedCmd)
		}
	}

	// Set up the output file with gzip compression
	outputFile, err := os.Create(primaryBackupPath)
	if err != nil {
		errMsg := fmt.Sprintf("failed to create backup file: %v", err)
		if logFile != nil {
			fmt.Fprintf(logFile, "ERROR: %s\n", errMsg)
		}
		metrics.BackupCount.WithLabelValues(backupType, database, "error").Inc()
		metadata.DefaultStore.UpdateBackupStatus(meta.ID, metadata.StatusError, map[string]string{}, 0, errMsg)
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer outputFile.Close()

	// Set up gzip writer
	gzipWriter := gzip.NewWriter(outputFile)
	defer gzipWriter.Close()

	// Capture stderr output
	var stderr bytes.Buffer

	// Set up stderr capture (since we don't have a dedicated method for this)
	cmd := exec.Command("cat") // placeholder command
	cmd.Stderr = &stderr

	// Execute the backup
	err = provider.Backup(ctx, database, gzipWriter, backupOpts)

	// Log stderr output if available
	if logFile != nil && stderr.Len() > 0 {
		fmt.Fprintf(logFile, "Command stderr output:\n%s\n", stderr.String())
	}

	if err != nil {
		// Get error details
		errOutput := stderr.String()
		var errMsg string
		if errOutput != "" {
			errMsg = fmt.Sprintf("database backup failed: %v - %s", err, errOutput)
		} else {
			errMsg = fmt.Sprintf("database backup failed: %v", err)
		}

		if logFile != nil {
			fmt.Fprintf(logFile, "ERROR: Backup failed: %s\n", errMsg)
		}

		metrics.BackupCount.WithLabelValues(backupType, database, "error").Inc()
		metadata.DefaultStore.UpdateBackupStatus(meta.ID, metadata.StatusError, map[string]string{}, 0, errMsg)
		return fmt.Errorf("database backup failed: %w", err)
	}

	if logFile != nil {
		fmt.Fprintf(logFile, "Backup command completed successfully\n")
	}

	// If using local backup with multiple paths, copy the file to all paths
	if localBackupEnabled && len(localPaths) > 1 {
		// We already created the primary file (by-server), now copy to by-type
		if byTypePath, ok := localPaths["by-type"]; ok {
			// Create parent directory
			if err := os.MkdirAll(filepath.Dir(byTypePath), 0755); err != nil {
				log.Printf("Warning: Failed to create directory for by-type backup: %v", err)
			} else {
				// Copy the file
				srcFile, err := os.Open(primaryBackupPath)
				if err != nil {
					log.Printf("Warning: Failed to open source file for copying: %v", err)
				} else {
					defer srcFile.Close()

					dstFile, err := os.Create(byTypePath)
					if err != nil {
						log.Printf("Warning: Failed to create by-type backup file: %v", err)
					} else {
						defer dstFile.Close()

						if _, err := io.Copy(dstFile, srcFile); err != nil {
							log.Printf("Warning: Failed to copy backup file: %v", err)
						} else {
							if logFile != nil {
								fmt.Fprintf(logFile, "Successfully copied backup to by-type path: %s\n", byTypePath)
							}
						}
					}
				}
			}
		}
	}

	// Record backup duration
	duration := time.Since(startTime)
	metrics.BackupDuration.WithLabelValues(backupType, database).Observe(duration.Seconds())

	// Record local storage metrics
	if localBackupEnabled {
		if err := m.localStore.RecordBackupMetrics(primaryBackupPath, backupType, database); err != nil {
			log.Printf("Warning: Failed to record local backup metrics: %v", err)
		}
	}

	// Get file size for updating metadata and logging
	var fileSize int64
	fileInfo, err := os.Stat(primaryBackupPath)
	if err == nil {
		fileSize = fileInfo.Size()
		log.Printf("Successfully created backup for database %s on server %s (%.2f MB)",
			database, serverName, float64(fileSize)/(1024*1024))
	} else {
		log.Printf("Successfully created backup for database %s on server %s (size unknown)",
			database, serverName)
	}

	// Record success in metadata
	metadata.DefaultStore.UpdateBackupStatus(meta.ID, metadata.StatusSuccess, localPaths, fileSize, "")

	// Record success in metrics
	metrics.BackupCount.WithLabelValues(backupType, database, "success").Inc()
	metrics.LastBackupTimestamp.WithLabelValues(backupType, database).Set(float64(time.Now().Unix()))

	// Upload to S3 if enabled for this backup type
	s3BackupEnabled := m.cfg.S3.Enabled && typeConfig.S3.Enabled && m.s3Store != nil
	if s3BackupEnabled {
		// For S3 upload, we need to handle each path separately
		s3UploadSuccessful := true
		uploadErrors := make([]string, 0)

		// Upload the file to all S3 paths
		for organization, s3Key := range s3Keys {
			if err := m.s3Store.UploadBackupWithKey(primaryBackupPath, s3Key); err != nil {
				log.Printf("Failed to upload backup to S3 (%s path): %v", organization, err)
				uploadErrors = append(uploadErrors, fmt.Sprintf("%s path: %v", organization, err))
				s3UploadSuccessful = false
			} else if logFile != nil {
				fmt.Fprintf(logFile, "Successfully uploaded backup to S3 (%s path): %s\n", organization, s3Key)
			}
		}

		// Update metadata based on overall success
		if s3UploadSuccessful {
			metadata.DefaultStore.UpdateS3UploadStatus(meta.ID, metadata.StatusSuccess, s3Keys, "")
		} else {
			errMsg := fmt.Sprintf("S3 upload failed: %s", strings.Join(uploadErrors, "; "))
			metadata.DefaultStore.UpdateS3UploadStatus(meta.ID, metadata.StatusError, map[string]string{}, errMsg)
		}
	} else if m.cfg.Debug {
		log.Printf("S3 upload not enabled for backup type %s, skipping upload", backupType)
		metadata.DefaultStore.UpdateS3UploadStatus(meta.ID, metadata.StatusError, map[string]string{},
			"S3 upload not enabled for this backup type")
	}

	return nil
}

// getActiveDatabaseProvider returns an appropriate database provider for MySQL
func getActiveDatabaseProvider() (*sql.DB, error) {
	// Connect to MySQL
	cfg := config.CFG.MySQL
	portInt, _ := strconv.Atoi(cfg.Port)
	if portInt == 0 {
		portInt = 3306 // Default MySQL port
	}

	// Build DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/",
		cfg.Username, cfg.Password, cfg.Host, portInt)

	// Open connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping MySQL server: %w", err)
	}

	return db, nil
}

// queryAllDatabases connects to MySQL and returns a list of all databases
func queryAllDatabases(db *sql.DB) ([]string, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Query all databases
	rows, err := db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return nil, fmt.Errorf("failed to scan database name: %w", err)
		}

		// Skip system databases by default
		if dbName == "information_schema" || dbName == "mysql" ||
			dbName == "performance_schema" || dbName == "sys" {
			continue
		}

		databases = append(databases, dbName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating database rows: %w", err)
	}

	if len(databases) == 0 {
		log.Println("Warning: No user databases found on the MySQL server")
	} else {
		log.Printf("Found %d user databases on the MySQL server", len(databases))
	}

	return databases, nil
}

// EnforceRetentionPolicies enforces retention policies across all storage types
func (m *Manager) EnforceRetentionPolicies() {
	log.Println("Enforcing retention policies...")

	// Purge metadata records for backups deleted more than 7 days ago
	purgedCount := metadata.DefaultStore.PurgeDeletedBackups(7 * 24 * time.Hour)
	if purgedCount > 0 {
		log.Printf("Purged %d deleted backup records from metadata", purgedCount)
	}

	// Enforce local retention if enabled
	if m.cfg.Local.Enabled && m.localStore != nil {
		if err := m.localStore.EnforceRetention(); err != nil {
			log.Printf("Error enforcing local retention policies: %v", err)
		}
	}

	// Enforce S3 retention if enabled
	if m.cfg.S3.Enabled && m.s3Store != nil {
		if err := m.s3Store.EnforceRetention(); err != nil {
			log.Printf("Error enforcing S3 retention policies: %v", err)
		}
	}
}
