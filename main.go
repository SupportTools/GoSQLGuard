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
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/scheduler"
)

func main() {
	log.Println("Starting GoSQLGuard...")

	// Load and validate configuration
	config.LoadConfiguration()
	if err := config.ValidateConfig(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
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
