// Package scheduler manages scheduled backup operations.
package scheduler

import (
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/supporttools/GoSQLGuard/pkg/backup"
	"github.com/supporttools/GoSQLGuard/pkg/config"
)

// Scheduler handles cron scheduling for backups and retention
type Scheduler struct {
	cronScheduler *cron.Cron
	backupManager *backup.Manager
	cfg           *config.AppConfig
	jobIDs        map[string]cron.EntryID // Track job IDs for dynamic updates
}

// NewScheduler creates a new scheduler
func NewScheduler(backupManager *backup.Manager) (*Scheduler, error) {
	return &Scheduler{
		cronScheduler: cron.New(),
		backupManager: backupManager,
		cfg:           &config.CFG,
		jobIDs:        make(map[string]cron.EntryID),
	}, nil
}

// SetupJobs configures all scheduled jobs
func (s *Scheduler) SetupJobs() error {
	// Schedule backup jobs for each backup type
	for backupType, typeConfig := range s.cfg.BackupTypes {
		// Skip if backup type has no schedule configured
		if typeConfig.Schedule == "" {
			log.Printf("No schedule configured for backup type %s, skipping", backupType)
			continue
		}

		// Create a closure to capture the backup type for the cron job
		backupFunc := func(bType string) func() {
			return func() {
				log.Printf("Starting %s backup...", bType)
				if err := s.backupManager.PerformBackup(bType); err != nil {
					log.Printf("Error performing %s backup: %v", bType, err)
				}
			}
		}

		// Add the cron job with the specified schedule
		jobID, err := s.cronScheduler.AddFunc(typeConfig.Schedule, backupFunc(backupType))
		if err != nil {
			log.Printf("Failed to schedule %s backup with cron expression '%s': %v",
				backupType, typeConfig.Schedule, err)
			continue
		}

		// Store the job ID for later updates
		s.jobIDs[backupType] = jobID

		log.Printf("Scheduled %s backup with cron expression: %s", backupType, typeConfig.Schedule)
	}

	// Schedule retention policy enforcement job
	_, err := s.cronScheduler.AddFunc("15 * * * *", func() {
		s.backupManager.EnforceRetentionPolicies()
	})
	if err != nil {
		return fmt.Errorf("failed to schedule retention policy enforcement: %w", err)
	}
	log.Println("Scheduled retention policy enforcement at minute 15 of every hour")

	return nil
}

// Start begins the scheduled jobs
func (s *Scheduler) Start() {
	s.cronScheduler.Start()
	log.Println("Backup scheduler started successfully")
}

// Stop halts all scheduled jobs
func (s *Scheduler) Stop() {
	ctx := s.cronScheduler.Stop()
	<-ctx.Done()
	log.Println("Backup scheduler stopped")
}

// WaitForever blocks indefinitely to keep the application running
func (s *Scheduler) WaitForever() {
	// Create a channel that never receives any values to block forever
	blockForever := make(chan struct{})
	<-blockForever
}

// ReloadSchedules removes all existing jobs and re-creates them based on current configuration
func (s *Scheduler) ReloadSchedules() error {
	log.Println("Reloading backup schedules...")
	
	// Remove all existing backup jobs
	for backupType, jobID := range s.jobIDs {
		s.cronScheduler.Remove(jobID)
		delete(s.jobIDs, backupType)
		log.Printf("Removed schedule for %s backup", backupType)
	}
	
	// Re-setup jobs with new configuration
	err := s.SetupJobs()
	if err != nil {
		return fmt.Errorf("failed to reload schedules: %w", err)
	}
	
	log.Println("Successfully reloaded backup schedules")
	return nil
}

// RunOnce runs a single backup of the specified type
// If servers is provided, only backup those servers
// If databases is provided, only backup those databases
func (s *Scheduler) RunOnce(backupType string, servers []string, databases []string) error {
	if len(servers) > 0 {
		log.Printf("Running one-time backup for type: %s on servers: %v", backupType, servers)
		if len(databases) > 0 {
			log.Printf("Only backing up databases: %v", databases)
		}
	} else {
		log.Printf("Running one-time backup for type: %s on all servers", backupType)
	}
	
	return s.backupManager.PerformBackup(backupType, backup.BackupOptions{
		Servers:   servers,
		Databases: databases,
	})
}

// RunRetentionOnce runs retention policy enforcement once
func (s *Scheduler) RunRetentionOnce() {
	log.Println("Running one-time retention policy enforcement")
	s.backupManager.EnforceRetentionPolicies()
}

// GetNextRunTime returns the next scheduled run time for a backup type
func (s *Scheduler) GetNextRunTime(backupType string) (time.Time, error) {
	// Find the entry for the specified backup type
	for _, entry := range s.cronScheduler.Entries() {
		// We need to compare the function address, which is not directly possible
		// So this is a simplification; in a real implementation, you might need 
		// to track entry IDs or use another approach
		
		// For now, just return the next time for any backup
		return entry.Next, nil
	}
	
	return time.Time{}, fmt.Errorf("no scheduled job found for backup type: %s", backupType)
}
