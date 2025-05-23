// Package types defines common metadata types and interfaces
package types

import (
	"time"
)

// BackupStatus represents the current status of a backup
type BackupStatus string

const (
	// StatusPending indicates a backup is in progress
	StatusPending BackupStatus = "pending"
	// StatusSuccess indicates a successful backup
	StatusSuccess BackupStatus = "success"
	// StatusError indicates a failed backup
	StatusError BackupStatus = "error"
	// StatusDeleted indicates a backup that was deleted by retention policy
	StatusDeleted BackupStatus = "deleted"
)

// BackupMeta represents metadata for a single backup
type BackupMeta struct {
	ID               string              `json:"id"`                // Unique identifier (typically timestamp-based)
	ServerName       string              `json:"serverName"`        // Server name (for multi-server support)
	ServerType       string              `json:"serverType"`        // mysql or postgresql
	Database         string              `json:"database"`          // Database name
	BackupType       string              `json:"backupType"`        // hourly, daily, weekly, etc.
	CreatedAt        time.Time           `json:"createdAt"`         // When backup was created
	Size             int64               `json:"size"`              // Size in bytes
	LocalPaths       map[string]string   `json:"localPaths"`        // Paths in local storage by organization (by-server, by-type)
	S3Keys           map[string]string   `json:"s3Keys"`            // S3 object keys by organization (by-server, by-type)
	RetentionPolicy  string              `json:"retentionPolicy"`   // Human readable retention
	ExpiresAt        time.Time           `json:"expiresAt"`         // When backup will be deleted
	Status           BackupStatus        `json:"status"`            // success, error
	ErrorMessage     string              `json:"errorMessage"`      // Error details if any
	LogFilePath      string              `json:"logFilePath"`       // Path to the log file (if available)
	S3UploadStatus   BackupStatus        `json:"s3UploadStatus"`    // success, pending, error
	S3UploadError    string              `json:"s3UploadError"`     // S3 upload error if any
	CompletedAt      time.Time           `json:"completedAt"`       // When backup completed
	S3UploadComplete time.Time           `json:"s3UploadComplete"`  // When S3 upload completed
	
	// For backward compatibility - these will be populated from the maps above
	LocalPath        string              `json:"localPath"`         // Legacy field - primary local path
	S3Key            string              `json:"s3Key"`             // Legacy field - primary S3 key
}

// MetadataStore defines the interface for metadata operations
type MetadataStore interface {
	// CreateBackupMeta creates a new backup metadata entry
	CreateBackupMeta(serverName, serverType, database, backupType string) *BackupMeta
	
	// UpdateBackupStatus updates the status of a backup
	UpdateBackupStatus(id string, status BackupStatus, localPaths map[string]string, size int64, errorMsg string) error
	
	// UpdateS3UploadStatus updates the S3 upload status of a backup
	UpdateS3UploadStatus(id string, status BackupStatus, s3Keys map[string]string, errorMsg string) error
	
	// GetBackups returns all backups
	GetBackups() []BackupMeta
	
	// GetBackupsFiltered returns backups filtered by server, database and/or type
	GetBackupsFiltered(serverName, database, backupType string, activeOnly bool) []BackupMeta
	
	// GetBackupByID returns a specific backup by ID
	GetBackupByID(id string) (BackupMeta, bool)
	
	// MarkBackupDeleted marks a backup as deleted
	MarkBackupDeleted(id string) error
	
	// GetStats returns statistics about the backups
	GetStats() map[string]interface{}
	
	// UpdateLogFilePath updates the log file path for a backup
	UpdateLogFilePath(id string, logFilePath string) error
	
	// PurgeDeletedBackups removes backup entries that have been marked as deleted
	// and are older than the specified duration
	PurgeDeletedBackups(olderThan time.Duration) int
	
	// Load loads the metadata
	Load() error
	
	// Save persists the metadata
	Save() error
}
