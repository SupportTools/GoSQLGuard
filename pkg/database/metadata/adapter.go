package metadata

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
)

// DBMetadataStore is a database-backed implementation of the metadata store interface
type DBMetadataStore struct {
	repo        *Repository
	mutex       sync.RWMutex
	filepath    string              // For backward compatibility
	s3Key       string              // For backward compatibility
	fileStore   types.MetadataStore // Optional fallback for reading existing metadata
	initialized bool
}

// NewDBMetadataStore creates a new database-backed metadata store
func NewDBMetadataStore(repo *Repository) *DBMetadataStore {
	store := &DBMetadataStore{
		repo:        repo,
		initialized: true,
	}

	// Set file paths for compatibility with original code
	if config.CFG.Local.Enabled {
		store.filepath = filepath.Join(config.CFG.Local.BackupDirectory, "metadata.json")
	}

	// Set S3 key
	if config.CFG.S3.Enabled {
		store.s3Key = filepath.Join(config.CFG.S3.Prefix, "metadata.json")
	}

	return store
}

// SetFallbackStore sets a fallback file-based store for migration purposes
func (s *DBMetadataStore) SetFallbackStore(fileStore types.MetadataStore) {
	s.fileStore = fileStore
}

// MigrateFromFileStore migrates all records from the file-based store to the database
func (s *DBMetadataStore) MigrateFromFileStore() error {
	if s.fileStore == nil {
		return fmt.Errorf("no file store set for migration")
	}

	// Get all backups from file store
	fileBackups := s.fileStore.GetBackups()
	log.Printf("Migrating %d backup records from file-based store to database", len(fileBackups))

	// Convert file backups to DB model
	var dbBackups []Backup
	for _, fb := range fileBackups {
		// Create database model backup
		backup := Backup{
			ID:              fb.ID,
			ServerName:      fb.ServerName,
			ServerType:      fb.ServerType,
			DatabaseName:    fb.Database,
			BackupType:      fb.BackupType,
			CreatedAt:       fb.CreatedAt,
			Size:            fb.Size,
			Status:          string(fb.Status),
			ErrorMessage:    fb.ErrorMessage,
			RetentionPolicy: fb.RetentionPolicy,
			LogFilePath:     fb.LogFilePath,
			S3UploadStatus:  string(fb.S3UploadStatus),
			S3UploadError:   fb.S3UploadError,
		}

		// Handle times (some might be zero)
		if !fb.CompletedAt.IsZero() {
			completedAt := fb.CompletedAt
			backup.CompletedAt = &completedAt
		}

		if !fb.ExpiresAt.IsZero() {
			expiresAt := fb.ExpiresAt
			backup.ExpiresAt = &expiresAt
		}

		if !fb.S3UploadComplete.IsZero() {
			s3UploadComplete := fb.S3UploadComplete
			backup.S3UploadComplete = &s3UploadComplete
		}

		// Handle related data
		for org, path := range fb.LocalPaths {
			backup.LocalPaths = append(backup.LocalPaths, LocalPath{
				BackupID:     fb.ID,
				Organization: org,
				Path:         path,
			})
		}

		for org, key := range fb.S3Keys {
			backup.S3Keys = append(backup.S3Keys, S3Key{
				BackupID:     fb.ID,
				Organization: org,
				Key:          key,
			})
		}

		dbBackups = append(dbBackups, backup)
	}

	// Import backups to database
	if err := s.repo.ImportBackups(dbBackups); err != nil {
		return fmt.Errorf("failed to import backups to database: %w", err)
	}

	// Recalculate stats
	if err := s.repo.RecalculateStats(); err != nil {
		log.Printf("Warning: Failed to recalculate stats after migration: %v", err)
	}

	log.Printf("Successfully migrated %d backup records to database", len(dbBackups))
	return nil
}

// CreateBackupMeta creates a new backup metadata entry
func (s *DBMetadataStore) CreateBackupMeta(serverName, serverType, database, backupType string) *types.BackupMeta {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Create unique ID using server, database, and timestamp
	id := fmt.Sprintf("%s-%s-%s-%s", serverName, database, backupType, time.Now().Format("20060102-150405"))

	// Get retention policy information
	var retentionText string
	var expiresAt time.Time

	if typeConfig, exists := config.CFG.BackupTypes[backupType]; exists {
		if typeConfig.Local.Enabled && typeConfig.Local.Retention.Forever {
			retentionText = "Keep forever"
		} else if typeConfig.Local.Enabled {
			duration, err := time.ParseDuration(typeConfig.Local.Retention.Duration)
			if err == nil {
				expiresAt = time.Now().Add(duration)
				retentionText = fmt.Sprintf("Keep for %s (until %s)",
					typeConfig.Local.Retention.Duration,
					expiresAt.Format("2006-01-02 15:04:05"))
			} else {
				retentionText = "Unknown retention policy"
			}
		}
	}

	// Create backup in database
	dbBackup := &Backup{
		ID:              id,
		ServerName:      serverName,
		ServerType:      serverType,
		DatabaseName:    database,
		BackupType:      backupType,
		CreatedAt:       time.Now(),
		Status:          string(types.StatusPending),
		S3UploadStatus:  string(types.StatusPending),
		RetentionPolicy: retentionText,
	}

	if !expiresAt.IsZero() {
		dbBackup.ExpiresAt = &expiresAt
	}

	err := s.repo.CreateBackup(dbBackup)
	if err != nil {
		log.Printf("Error creating backup record in database: %v", err)
	}

	// Convert back to original BackupMeta for compatibility
	meta := &types.BackupMeta{
		ID:              id,
		ServerName:      serverName,
		ServerType:      serverType,
		Database:        database,
		BackupType:      backupType,
		CreatedAt:       time.Now(),
		Status:          types.StatusPending,
		S3UploadStatus:  types.StatusPending,
		RetentionPolicy: retentionText,
		ExpiresAt:       expiresAt,
		LocalPaths:      make(map[string]string),
		S3Keys:          make(map[string]string),
	}

	return meta
}

// UpdateBackupStatus updates the status of a backup
func (s *DBMetadataStore) UpdateBackupStatus(id string, status types.BackupStatus, localPaths map[string]string, size int64, errorMsg string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Update status in database
	err := s.repo.UpdateBackupStatus(id, string(status), errorMsg, size, time.Now())
	if err != nil {
		return err
	}

	// Update local paths if provided
	if len(localPaths) > 0 {
		err = s.repo.UpdateBackupLocalPaths(id, localPaths)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateS3UploadStatus updates the S3 upload status of a backup
func (s *DBMetadataStore) UpdateS3UploadStatus(id string, status types.BackupStatus, s3Keys map[string]string, errorMsg string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Update S3 status
	err := s.repo.UpdateS3UploadStatus(id, string(status), errorMsg, time.Now())
	if err != nil {
		return err
	}

	// Update S3 keys if provided
	if len(s3Keys) > 0 {
		err = s.repo.UpdateBackupS3Keys(id, s3Keys)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateLogFilePath updates the log file path for a backup
func (s *DBMetadataStore) UpdateLogFilePath(id string, logFilePath string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.repo.UpdateLogFilePath(id, logFilePath)
}

// GetBackups returns all backups
func (s *DBMetadataStore) GetBackups() []types.BackupMeta {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Get all backups from database
	dbBackups, err := s.repo.GetAllBackups(true)
	if err != nil {
		log.Printf("Error retrieving backups from database: %v", err)
		return []types.BackupMeta{}
	}

	// Convert to original format
	return s.convertToOriginalFormat(dbBackups)
}

// GetBackupsFiltered returns backups filtered by server, database and/or type
func (s *DBMetadataStore) GetBackupsFiltered(serverName, database, backupType string, activeOnly bool) []types.BackupMeta {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Determine status filter
	status := ""
	if activeOnly {
		status = string(types.StatusSuccess)
	}

	// Get filtered backups from database
	dbBackups, err := s.repo.GetBackupsFiltered(serverName, database, backupType, status)
	if err != nil {
		log.Printf("Error retrieving filtered backups from database: %v", err)
		return []types.BackupMeta{}
	}

	// Convert to original format
	return s.convertToOriginalFormat(dbBackups)
}

// GetBackupByID returns a specific backup by ID
func (s *DBMetadataStore) GetBackupByID(id string) (types.BackupMeta, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Get backup from database
	dbBackup, err := s.repo.GetBackupByID(id)
	if err != nil {
		log.Printf("Error retrieving backup from database: %v", err)
		return types.BackupMeta{}, false
	}

	// If not found
	if dbBackup == nil {
		return types.BackupMeta{}, false
	}

	// Convert to original format
	origBackups := s.convertToOriginalFormat([]Backup{*dbBackup})
	if len(origBackups) == 0 {
		return types.BackupMeta{}, false
	}

	return origBackups[0], true
}

// MarkBackupDeleted marks a backup as deleted
func (s *DBMetadataStore) MarkBackupDeleted(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.repo.MarkBackupDeleted(id)
}

// GetStats returns statistics about the backups
func (s *DBMetadataStore) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Get stats from database
	_, detailedStats, err := s.repo.GetBackupStats()
	if err != nil {
		log.Printf("Error retrieving backup stats from database: %v", err)
		return map[string]interface{}{
			"totalCount":     0,
			"totalLocalSize": 0,
			"totalS3Size":    0,
			"statusCounts": map[string]int{
				"success": 0,
				"pending": 0,
				"error":   0,
				"deleted": 0,
			},
		}
	}

	return detailedStats
}

// PurgeDeletedBackups removes backup entries that have been marked as deleted
// and are older than the specified duration
func (s *DBMetadataStore) PurgeDeletedBackups(olderThan time.Duration) int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Purge from database
	count, err := s.repo.PurgeDeletedBackups(olderThan)
	if err != nil {
		log.Printf("Error purging deleted backups from database: %v", err)
		return 0
	}

	return int(count)
}

// Load loads the metadata (not needed for DB implementation, but kept for compatibility)
func (s *DBMetadataStore) Load() error {
	// No-op for database implementation
	return nil
}

// Save persists the metadata (not needed for DB implementation, but kept for compatibility)
func (s *DBMetadataStore) Save() error {
	// No-op for database implementation
	return nil
}

// Helper function to convert database backups to original format
func (s *DBMetadataStore) convertToOriginalFormat(dbBackups []Backup) []types.BackupMeta {
	origBackups := make([]types.BackupMeta, 0, len(dbBackups))

	for _, db := range dbBackups {
		// Create basic backup meta
		backup := types.BackupMeta{
			ID:              db.ID,
			ServerName:      db.ServerName,
			ServerType:      db.ServerType,
			Database:        db.DatabaseName,
			BackupType:      db.BackupType,
			CreatedAt:       db.CreatedAt,
			Size:            db.Size,
			Status:          types.BackupStatus(db.Status),
			ErrorMessage:    db.ErrorMessage,
			RetentionPolicy: db.RetentionPolicy,
			LogFilePath:     db.LogFilePath,
			S3UploadStatus:  types.BackupStatus(db.S3UploadStatus),
			S3UploadError:   db.S3UploadError,
			LocalPaths:      make(map[string]string),
			S3Keys:          make(map[string]string),
		}

		// Handle optional time fields
		if db.CompletedAt != nil {
			backup.CompletedAt = *db.CompletedAt
		}

		if db.ExpiresAt != nil {
			backup.ExpiresAt = *db.ExpiresAt
		}

		if db.S3UploadComplete != nil {
			backup.S3UploadComplete = *db.S3UploadComplete
		}

		// Add local paths
		for _, path := range db.LocalPaths {
			backup.LocalPaths[path.Organization] = path.Path
		}

		// Add S3 keys
		for _, key := range db.S3Keys {
			backup.S3Keys[key.Organization] = key.Key
		}

		// Set legacy fields for backward compatibility
		if len(backup.LocalPaths) > 0 {
			if byServer, ok := backup.LocalPaths["by-server"]; ok {
				backup.LocalPath = byServer
			} else {
				// Just grab the first one
				for _, path := range backup.LocalPaths {
					backup.LocalPath = path
					break
				}
			}
		}

		if len(backup.S3Keys) > 0 {
			if byServer, ok := backup.S3Keys["by-server"]; ok {
				backup.S3Key = byServer
			} else {
				// Just grab the first one
				for _, key := range backup.S3Keys {
					backup.S3Key = key
					break
				}
			}
		}

		origBackups = append(origBackups, backup)
	}

	return origBackups
}
