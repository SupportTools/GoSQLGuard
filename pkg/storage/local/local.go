// Package local handles local filesystem storage operations for MySQL backups.
package local

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/metrics"
)

// Client represents a local filesystem client
type Client struct {
	cfg *config.AppConfig
}

// NewClient creates a new local storage client
func NewClient() (*Client, error) {
	if !config.CFG.Local.Enabled {
		return nil, fmt.Errorf("local storage is not enabled in configuration")
	}

	return &Client{
		cfg: &config.CFG,
	}, nil
}

// EnsureBackupPath ensures the backup directory exists
func (c *Client) EnsureBackupPath(backupType string) (string, error) {
	backupDir := filepath.Join(c.cfg.Local.BackupDirectory, backupType)

	// Ensure the directory exists
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory %s: %w", backupDir, err)
	}

	return backupDir, nil
}

// GetBackupPath returns the full path for a backup file
func (c *Client) GetBackupPath(backupType, backupFileName string) (string, error) {
	backupDir, err := c.EnsureBackupPath(backupType)
	if err != nil {
		return "", err
	}

	return filepath.Join(backupDir, backupFileName), nil
}

// RecordBackupMetrics records metrics for a local backup
func (c *Client) RecordBackupMetrics(backupPath, backupType, database string) error {
	// Get file size for metrics
	fileInfo, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("failed to stat backup file: %w", err)
	}

	sizeBytes := float64(fileInfo.Size())
	metrics.BackupSize.WithLabelValues(backupType, database, "local").Set(sizeBytes)

	return nil
}

// EnforceRetention implements retention policy for local backups
func (c *Client) EnforceRetention() error {
	for backupType, typeConfig := range c.cfg.BackupTypes {
		// Skip if local backup is not enabled for this type
		if !typeConfig.Local.Enabled {
			if c.cfg.Debug {
				log.Printf("Local backup not enabled for %s, skipping retention enforcement", backupType)
			}
			continue
		}

		// Skip if keep forever is set
		if typeConfig.Local.Retention.Forever {
			if c.cfg.Debug {
				log.Printf("Local backups for %s set to keep forever, skipping retention enforcement", backupType)
			}
			continue
		}

		// Parse duration string
		duration, err := time.ParseDuration(typeConfig.Local.Retention.Duration)
		if err != nil {
			log.Printf("Invalid duration for %s local retention: %v", backupType, err)
			continue
		}

		backupDir := filepath.Join(c.cfg.Local.BackupDirectory, backupType)

		// Find all backups of this type
		files, err := filepath.Glob(filepath.Join(backupDir, "*.sql.gz"))
		if err != nil {
			log.Printf("Error finding backups: %v", err)
			continue
		}

		// Check each file against retention policy
		for _, file := range files {
			fileInfo, err := os.Stat(file)
			if err != nil {
				continue
			}

			fileAge := time.Since(fileInfo.ModTime())

			if fileAge > duration {
				// Delete expired backup
				err = os.Remove(file)
				if err != nil {
					log.Printf("Failed to remove expired backup %s: %v", file, err)
				} else {
					// Extract backup ID from filename
					baseName := filepath.Base(file)
					// Find backup in metadata that matches this path and mark it deleted
					backups := metadata.DefaultStore.GetBackupsFiltered("", "", backupType, true)
					for _, backup := range backups {
						if backup.LocalPath == file || filepath.Base(backup.LocalPath) == baseName {
							if err := metadata.DefaultStore.MarkBackupDeleted(backup.ID); err != nil {
								log.Printf("Warning: Failed to mark backup %s as deleted in metadata: %v", backup.ID, err)
							} else {
								log.Printf("Marked backup %s as deleted in metadata", backup.ID)
							}
							break
						}
					}

					log.Printf("Removed expired local backup: %s", file)
					metrics.BackupRetentionDeletes.WithLabelValues(backupType, "local").Inc()
				}
			}
		}
	}

	return nil
}
