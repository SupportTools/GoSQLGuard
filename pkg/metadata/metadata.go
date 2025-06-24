// Package metadata manages tracking and persistence of backup metadata.
package metadata

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
)

// Re-export types from the types package for backward compatibility
type (
	// BackupMeta represents metadata for a single backup
	BackupMeta = types.BackupMeta
	// BackupStatus represents the status of a backup
	BackupStatus = types.BackupStatus
)

const (
	// StatusPending indicates a backup is in progress
	StatusPending = types.StatusPending
	// StatusSuccess indicates a successful backup
	StatusSuccess = types.StatusSuccess
	// StatusError indicates a failed backup
	StatusError = types.StatusError
	// StatusDeleted indicates a backup that was deleted by retention policy
	StatusDeleted = types.StatusDeleted
)

// MetadataStore manages backup metadata
type MetadataStore struct {
	Backups          []types.BackupMeta `json:"backups"`
	LastUpdated      time.Time          `json:"lastUpdated"`
	TotalLocalSize   int64              `json:"totalLocalSize"`
	TotalS3Size      int64              `json:"totalS3Size"`
	Version          string             `json:"version"`
}

// Store is the global metadata store instance
type Store struct {
	metadata MetadataStore
	mutex    sync.RWMutex
	filepath string
	s3Key    string
	s3Client interface{} // We'll define this interface when connecting to S3
}

// DefaultStore is the global metadata store instance
var DefaultStore types.MetadataStore

// GetActiveStore returns the currently active metadata store
// It returns the MySQL-backed store if available, otherwise the file-based store
func GetActiveStore() types.MetadataStore {
	// If we have a DBStore in DefaultStore, use it
	if DefaultStore != nil {
		return DefaultStore
	}
	
	// No store available
	return nil
}

// Initialize creates and initializes the metadata store
func Initialize() error {
	if DefaultStore != nil {
		return nil // Already initialized
	}

	// Check if we should use database metadata
	if config.CFG.MetadataDB.Enabled {
		return initializeDatabaseMetadata()
	}

	// Create file-based store
	store := &Store{
		metadata: MetadataStore{
			Backups:     make([]types.BackupMeta, 0),
			LastUpdated: time.Now(),
			Version:     "1.0",
		},
	}

	// Set metadata file path
	if config.CFG.Local.Enabled {
		store.filepath = filepath.Join(config.CFG.Local.BackupDirectory, "metadata.json")
	} else {
		// Use a temporary location if local storage is disabled
		tmpDir, err := os.MkdirTemp("", "gosqlguard-metadata")
		if err != nil {
			return fmt.Errorf("failed to create temp directory for metadata: %w", err)
		}
		store.filepath = filepath.Join(tmpDir, "metadata.json")
	}

	// Set S3 key
	if config.CFG.S3.Enabled {
		store.s3Key = filepath.Join(config.CFG.S3.Prefix, "metadata.json")
	}

	// Set the global store
	DefaultStore = store

	// Try to load existing metadata
	err := DefaultStore.Load()
	if err != nil {
		log.Printf("Warning: Could not load existing metadata, starting fresh: %v", err)
	}

	return nil
}

// initializeDatabaseMetadata initializes the database-backed metadata store
func initializeDatabaseMetadata() error {
	// This function is implemented in mysql_store.go
	return InitializeMetadataDatabase()
}

// Load loads the metadata from file
func (s *Store) Load() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if file exists
	if _, err := os.Stat(s.filepath); os.IsNotExist(err) {
		log.Printf("Metadata file does not exist at %s, will create new", s.filepath)
		return s.save() // Create empty metadata file
	}

	// Read the file
	data, err := os.ReadFile(s.filepath)
	if err != nil {
		return fmt.Errorf("failed to read metadata file: %w", err)
	}

	// Unmarshal data
	err = json.Unmarshal(data, &s.metadata)
	if err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	// Update calculated fields
	s.recalculateTotals()

	log.Printf("Loaded metadata with %d backup records", len(s.metadata.Backups))
	return nil
}

// Save persists the metadata to file
func (s *Store) Save() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.save()
}

// save is the internal method that performs the actual save (without locking)
func (s *Store) save() error {
	// Update last modified time
	s.metadata.LastUpdated = time.Now()

	// Recalculate totals
	s.recalculateTotals()

	// Marshal to JSON
	data, err := json.MarshalIndent(s.metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(s.filepath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for metadata: %w", err)
	}

	// Write to file
	if err := os.WriteFile(s.filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	log.Printf("Saved metadata with %d backup records to %s", len(s.metadata.Backups), s.filepath)
	return nil
}

// recalculateTotals updates the total size fields
func (s *Store) recalculateTotals() {
	var localSize, s3Size int64

	for _, backup := range s.metadata.Backups {
		// Only count active backups (not deleted or errored)
		if backup.Status == types.StatusSuccess {
			localSize += backup.Size
		}

		// Only count successful S3 uploads
		if backup.S3UploadStatus == types.StatusSuccess {
			s3Size += backup.Size
		}
	}

	s.metadata.TotalLocalSize = localSize
	s.metadata.TotalS3Size = s3Size
}

// CreateBackupMeta creates a new backup metadata entry
func (s *Store) CreateBackupMeta(serverName, serverType, database, backupType string) *types.BackupMeta {
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

	// Create new metadata entry
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

	// Add to metadata store
	s.metadata.Backups = append(s.metadata.Backups, *meta)

	// Save changes
	_ = s.save() // Ignore error, as we'll continue anyway

	return meta
}

// UpdateBackupStatus updates the status of a backup
func (s *Store) UpdateBackupStatus(id string, status types.BackupStatus, localPaths map[string]string, size int64, errorMsg string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Find the backup
	for i, backup := range s.metadata.Backups {
		if backup.ID == id {
			// Update fields
			s.metadata.Backups[i].Status = status
			s.metadata.Backups[i].LocalPaths = localPaths
			
			// For backward compatibility, set the primary LocalPath to by-server or the first path
			if len(localPaths) > 0 {
				if byServer, ok := localPaths["by-server"]; ok {
					s.metadata.Backups[i].LocalPath = byServer
				} else {
					// Just grab the first one if by-server doesn't exist
					for _, path := range localPaths {
						s.metadata.Backups[i].LocalPath = path
						break
					}
				}
			}
			
			s.metadata.Backups[i].Size = size
			s.metadata.Backups[i].ErrorMessage = errorMsg
			s.metadata.Backups[i].CompletedAt = time.Now()

			// Save changes
			return s.save()
		}
	}

	return fmt.Errorf("backup with ID %s not found", id)
}

// UpdateS3UploadStatus updates the S3 upload status of a backup
func (s *Store) UpdateS3UploadStatus(id string, status types.BackupStatus, s3Keys map[string]string, errorMsg string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Find the backup
	for i, backup := range s.metadata.Backups {
		if backup.ID == id {
			// Update fields
			s.metadata.Backups[i].S3UploadStatus = status
			s.metadata.Backups[i].S3Keys = s3Keys
			s.metadata.Backups[i].S3UploadError = errorMsg
			
			// For backward compatibility, set the primary S3Key to by-server or the first key
			if len(s3Keys) > 0 {
				if byServer, ok := s3Keys["by-server"]; ok {
					s.metadata.Backups[i].S3Key = byServer
				} else {
					// Just grab the first one if by-server doesn't exist
					for _, key := range s3Keys {
						s.metadata.Backups[i].S3Key = key
						break
					}
				}
			}
			
			if status != types.StatusPending {
				s.metadata.Backups[i].S3UploadComplete = time.Now()
			}

			// Save changes
			return s.save()
		}
	}

	return fmt.Errorf("backup with ID %s not found", id)
}

// GetBackups returns all backups
func (s *Store) GetBackups() []types.BackupMeta {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Return a copy to avoid concurrent modification issues
	result := make([]types.BackupMeta, len(s.metadata.Backups))
	copy(result, s.metadata.Backups)

	return result
}

// GetBackupsFiltered returns backups filtered by server, database and/or type
func (s *Store) GetBackupsFiltered(serverName, database, backupType string, activeOnly bool) []types.BackupMeta {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var result []types.BackupMeta

	for _, backup := range s.metadata.Backups {
		// Apply server filter if specified
		if serverName != "" && backup.ServerName != serverName {
			continue
		}

		// Apply database filter if specified
		if database != "" && backup.Database != database {
			continue
		}

		// Apply backup type filter if specified
		if backupType != "" && backup.BackupType != backupType {
			continue
		}

		// Apply active only filter if specified
		if activeOnly && backup.Status != types.StatusSuccess {
			continue
		}

		result = append(result, backup)
	}

	return result
}

// GetBackupByID returns a specific backup by ID
func (s *Store) GetBackupByID(id string) (types.BackupMeta, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for _, backup := range s.metadata.Backups {
		if backup.ID == id {
			return backup, true
		}
	}

	return types.BackupMeta{}, false
}

// MarkBackupDeleted marks a backup as deleted
func (s *Store) MarkBackupDeleted(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Find the backup
	for i, backup := range s.metadata.Backups {
		if backup.ID == id {
			// Mark as deleted
			s.metadata.Backups[i].Status = types.StatusDeleted
			
			// Save changes
			return s.save()
		}
	}

	return fmt.Errorf("backup with ID %s not found", id)
}

// GetStats returns statistics about the backups
func (s *Store) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	stats := map[string]interface{}{
		"totalCount":       len(s.metadata.Backups),
		"totalLocalSize":   s.metadata.TotalLocalSize,
		"totalS3Size":      s.metadata.TotalS3Size,
		"lastBackupTime":   nil,
		"typeDistribution": make(map[string]int),
		"serverDistribution": make(map[string]int),
		"statusCounts": map[string]int{
			"success": 0,
			"pending": 0,
			"error":   0,
			"deleted": 0,
		},
	}

	// Calculate additional stats
	var lastBackupTime time.Time
	typeCount := make(map[string]int)
	databaseCount := make(map[string]int)
	serverCount := make(map[string]int)

	for _, backup := range s.metadata.Backups {
		// Count by status
		stats["statusCounts"].(map[string]int)[string(backup.Status)]++
		
		// Count by type
		typeCount[backup.BackupType]++
		
		// Count by database
		databaseCount[backup.Database]++
		
		// Count by server
		serverCount[backup.ServerName]++
		
		// Track last backup time
		if backup.Status == types.StatusSuccess && backup.CompletedAt.After(lastBackupTime) {
			lastBackupTime = backup.CompletedAt
			stats["lastBackupTime"] = lastBackupTime
		}
	}

	stats["typeDistribution"] = typeCount
	stats["databaseDistribution"] = databaseCount
	stats["serverDistribution"] = serverCount

	return stats
}

// UpdateLogFilePath updates the log file path for a backup
func (s *Store) UpdateLogFilePath(id string, logFilePath string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Find the backup
	for i, backup := range s.metadata.Backups {
		if backup.ID == id {
			// Update log file path
			s.metadata.Backups[i].LogFilePath = logFilePath
			
			// Save changes
			return s.save()
		}
	}

	return fmt.Errorf("backup with ID %s not found", id)
}

// PurgeDeletedBackups removes backup entries that have been marked as deleted
// and are older than the specified duration
func (s *Store) PurgeDeletedBackups(olderThan time.Duration) int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	threshold := now.Add(-olderThan)
	newBackups := make([]types.BackupMeta, 0, len(s.metadata.Backups))
	removedCount := 0

	for _, backup := range s.metadata.Backups {
		// Keep anything that's not deleted
		if backup.Status != types.StatusDeleted {
			newBackups = append(newBackups, backup)
			continue
		}

		// Keep deleted backups that are newer than threshold
		if backup.CompletedAt.After(threshold) {
			newBackups = append(newBackups, backup)
			continue
		}

		// Skip (remove) deleted backups older than threshold
		removedCount++
	}

	// Only save if we removed something
	if removedCount > 0 {
		s.metadata.Backups = newBackups
		_ = s.save()
	}

	return removedCount
}
