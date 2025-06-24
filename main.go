package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/supporttools/GoSQLGuard/pkg/adminserver"
	"github.com/supporttools/GoSQLGuard/pkg/backup"
	_ "github.com/supporttools/GoSQLGuard/pkg/backup/database/mysql"
	_ "github.com/supporttools/GoSQLGuard/pkg/backup/database/postgresql"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	dbmeta "github.com/supporttools/GoSQLGuard/pkg/database/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/scheduler"
)

func main() {
	log.Println("Starting GoSQLGuard...")

	// Load and validate configuration
	config.LoadConfiguration()
	
	// Skip validation if metadata database is enabled - we'll load config from DB
	if !config.CFG.MetadataDB.Enabled {
		if err := config.ValidateConfig(); err != nil {
			log.Fatalf("Configuration validation failed: %v", err)
		}
	}

	if config.CFG.Debug {
		log.Println("Configuration loaded and validated successfully")
	}

	// Initialize metadata store (try database first, fall back to file-based)
	var metadataErr error
	if config.CFG.MetadataDB.Enabled {
		// Try to initialize the database-backed metadata store
		metadataErr = metadata.InitializeMetadataDatabase()
	} else {
		// Fall back to file-based if database not enabled
		metadataErr = metadata.Initialize()
	}

	// Handle metadata initialization error
	if metadataErr != nil {
		log.Fatalf("Failed to initialize metadata store: %v", metadataErr)
	}

	// Initialize backup manager
	backupManager, err := backup.NewManager()
	if err != nil {
		log.Fatalf("Failed to initialize backup manager: %v", err)
	}

	// Initialize scheduler
	sched, err := scheduler.NewScheduler(backupManager)
	if err != nil {
		log.Fatalf("Failed to initialize scheduler: %v", err)
	}

	// Load configuration from database if metadata database is enabled
	if config.CFG.MetadataDB.Enabled && metadata.DB != nil {
		loadConfigurationFromDatabase()
		loadSchedulesFromDatabase()
		
		// Now validate the complete configuration
		if err := config.ValidateConfig(); err != nil {
			log.Fatalf("Configuration validation failed after loading from database: %v", err)
		}
	}

	// Setup scheduled jobs
	if err := sched.SetupJobs(); err != nil {
		log.Fatalf("Failed to setup scheduled jobs: %v", err)
	}

	// Start the scheduler
	sched.Start()

	// Start the admin server
	adminSrv := adminserver.NewServer(backupManager, sched)
	httpServer := adminSrv.Start()

	// Setup signal handling for graceful shutdown
	setupSignalHandling(sched, httpServer)

	// Block indefinitely
	log.Println("GoSQLGuard is running. Press Ctrl+C to exit.")
	sched.WaitForever()
}

// setupSignalHandling configures graceful shutdown on SIGINT or SIGTERM
func setupSignalHandling(sched *scheduler.Scheduler, httpServer *http.Server) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-c
		fmt.Printf("Received signal %s, shutting down...\n", sig)
		sched.Stop()

		// Shutdown the HTTP server
		if httpServer != nil {
			if err := httpServer.Close(); err != nil {
				log.Printf("Error shutting down HTTP server: %v", err)
			}
		}

		os.Exit(0)
	}()
}

// loadConfigurationFromDatabase loads database server configurations from the database
func loadConfigurationFromDatabase() {
	log.Println("Loading database server configurations from database...")
	
	// Initialize server repository
	serverRepo := dbmeta.NewServerRepository(metadata.DB)
	
	// Get all servers from the database
	servers, err := serverRepo.GetAllServers()
	if err != nil {
		log.Printf("Failed to load server configurations from database: %v", err)
		return
	}
	
	// Convert database servers to config format
	var databaseServers []config.DatabaseServerConfig
	
	for _, server := range servers {
		// Parse included/excluded databases from filters
		var includeDatabases, excludeDatabases []string
		
		for _, filter := range server.DatabaseFilters {
			if filter.FilterType == "include" {
				includeDatabases = append(includeDatabases, filter.DatabaseName)
			} else if filter.FilterType == "exclude" {
				excludeDatabases = append(excludeDatabases, filter.DatabaseName)
			}
		}
		
		// Create server config
		serverConfig := config.DatabaseServerConfig{
			Name:             server.Name,
			Type:             server.Type,
			Host:             server.Host,
			Port:             server.Port,
			Username:         server.Username,
			Password:         server.Password,
			AuthPlugin:       server.AuthPlugin,
			IncludeDatabases: includeDatabases,
			ExcludeDatabases: excludeDatabases,
		}
		
		databaseServers = append(databaseServers, serverConfig)
	}
	
	// Update the global configuration
	if len(databaseServers) > 0 {
		config.CFG.DatabaseServers = databaseServers
		log.Printf("Successfully loaded %d server configurations from the database", len(databaseServers))
		
		// Set legacy MySQL config for backwards compatibility if we have a MySQL server
		for _, server := range databaseServers {
			if server.Type == "mysql" {
				config.CFG.MySQL = config.MySQLConfig{
					Host:             server.Host,
					Port:             server.Port,
					Username:         server.Username,
					Password:         server.Password,
					IncludeDatabases: server.IncludeDatabases,
					ExcludeDatabases: server.ExcludeDatabases,
				}
				break
			}
		}
	} else {
		log.Println("No database servers found in database")
	}
}

// loadSchedulesFromDatabase loads schedule configurations from the database
func loadSchedulesFromDatabase() {
	log.Println("Loading schedules from database...")
	
	// Initialize schedule repository
	scheduleRepo := dbmeta.NewScheduleRepository(metadata.DB)
	
	// Get all schedules from the database
	schedules, err := scheduleRepo.GetAllSchedules()
	if err != nil {
		log.Printf("Failed to load schedule configurations from database: %v", err)
		return
	}
	
	// Convert database schedules to config format
	backupTypes := make(map[string]config.BackupTypeConfig)
	
	for _, schedule := range schedules {
		if !schedule.Enabled {
			continue // Skip disabled schedules
		}
		
		// Create backup type config
		backupType := config.BackupTypeConfig{
			Schedule: schedule.CronExpression,
			Local: config.LocalBackupConfig{
				Enabled: false,
				Retention: config.RetentionRule{
					Duration: "24h",
					Forever:  false,
				},
			},
			S3: config.S3BackupConfig{
				Enabled: false,
				Retention: config.RetentionRule{
					Duration: "24h",
					Forever:  false,
				},
			},
		}
		
		// Process retention policies
		for _, policy := range schedule.RetentionPolicies {
			if policy.StorageType == "local" {
				backupType.Local.Enabled = true
				backupType.Local.Retention.Duration = policy.Duration
				backupType.Local.Retention.Forever = policy.KeepForever
			} else if policy.StorageType == "s3" {
				backupType.S3.Enabled = true
				backupType.S3.Retention.Duration = policy.Duration
				backupType.S3.Retention.Forever = policy.KeepForever
			}
		}
		
		// Add to map
		backupTypes[schedule.BackupType] = backupType
	}
	
	// Update the global configuration
	if len(backupTypes) > 0 {
		config.CFG.BackupTypes = backupTypes
		log.Printf("Successfully loaded %d schedule configurations from the database", len(schedules))
	} else {
		log.Println("No schedules found in database, using configuration file schedules")
	}
}
