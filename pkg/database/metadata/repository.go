package metadata

import (
	"log"
	"sync"
	"time"

	"gorm.io/gorm"
)

// Repository handles database operations for backup metadata
type Repository struct {
	db    *gorm.DB
	mutex sync.RWMutex
}

// NewRepository creates a new metadata repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db: db,
	}
}

// CreateBackup creates a new backup record
func (r *Repository) CreateBackup(backup *Backup) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.db.Create(backup).Error
}

// UpdateBackupStatus updates the status of a backup
func (r *Repository) UpdateBackupStatus(id, status, errorMsg string, size int64, completedAt time.Time) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.db.Model(&Backup{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        status,
		"error_message": errorMsg,
		"size":          size,
		"completed_at":  completedAt,
	}).Error
}

// UpdateBackupLocalPaths updates the local paths for a backup
func (r *Repository) UpdateBackupLocalPaths(id string, paths map[string]string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Start a transaction
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete existing paths
		if err := tx.Where("backup_id = ?", id).Delete(&LocalPath{}).Error; err != nil {
			return err
		}

		// Create new paths if any
		for org, path := range paths {
			localPath := LocalPath{
				BackupID:     id,
				Organization: org,
				Path:         path,
			}
			if err := tx.Create(&localPath).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// UpdateBackupS3Keys updates the S3 keys for a backup
func (r *Repository) UpdateBackupS3Keys(id string, keys map[string]string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Start a transaction
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete existing keys
		if err := tx.Where("backup_id = ?", id).Delete(&S3Key{}).Error; err != nil {
			return err
		}

		// Create new keys if any
		for org, key := range keys {
			s3Key := S3Key{
				BackupID:     id,
				Organization: org,
				Key:          key,
			}
			if err := tx.Create(&s3Key).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// UpdateS3UploadStatus updates the S3 upload status of a backup
func (r *Repository) UpdateS3UploadStatus(id, status, errorMsg string, uploadTime time.Time) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.db.Model(&Backup{}).Where("id = ?", id).Updates(map[string]interface{}{
		"s3_upload_status":   status,
		"s3_upload_error":    errorMsg,
		"s3_upload_complete": uploadTime,
	}).Error
}

// GetBackupByID retrieves a backup by ID with all related data
func (r *Repository) GetBackupByID(id string) (*Backup, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var backup Backup
	err := r.db.Preload("LocalPaths").Preload("S3Keys").Where("id = ?", id).First(&backup).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}

	return &backup, nil
}

// GetAllBackups retrieves all backups with optional preloading
func (r *Repository) GetAllBackups(preload bool) ([]Backup, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var backups []Backup
	query := r.db
	
	if preload {
		query = query.Preload("LocalPaths").Preload("S3Keys")
	}
	
	err := query.Find(&backups).Error
	if err != nil {
		return nil, err
	}

	return backups, nil
}

// GetBackupsFiltered retrieves backups with filters
func (r *Repository) GetBackupsFiltered(serverName, database, backupType, status string) ([]Backup, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	query := r.db.Preload("LocalPaths").Preload("S3Keys")
	
	if serverName != "" {
		query = query.Where("server_name = ?", serverName)
	}
	
	if database != "" {
		query = query.Where("database_name = ?", database)
	}
	
	if backupType != "" {
		query = query.Where("backup_type = ?", backupType)
	}
	
	if status != "" {
		query = query.Where("status = ?", status)
	}
	
	var backups []Backup
	err := query.Find(&backups).Error
	if err != nil {
		return nil, err
	}

	return backups, nil
}

// MarkBackupDeleted marks a backup as deleted
func (r *Repository) MarkBackupDeleted(id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.db.Model(&Backup{}).Where("id = ?", id).Update("status", "deleted").Error
}

// UpdateLogFilePath updates the log file path for a backup
func (r *Repository) UpdateLogFilePath(id, logFilePath string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.db.Model(&Backup{}).Where("id = ?", id).Update("log_file_path", logFilePath).Error
}

// GetBackupStats calculates statistics about backups
func (r *Repository) GetBackupStats() (*MetadataStats, map[string]interface{}, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Get the global stats record
	var stats MetadataStats
	if err := r.db.First(&stats, 1).Error; err != nil {
		return nil, nil, err
	}

	// Count by status
	type StatusCount struct {
		Status string
		Count  int
	}
	var statusCounts []StatusCount
	if err := r.db.Model(&Backup{}).Select("status, count(*) as count").Group("status").Find(&statusCounts).Error; err != nil {
		return nil, nil, err
	}

	// Count by type
	type TypeCount struct {
		BackupType string `gorm:"column:backup_type"`
		Count      int
	}
	var typeCounts []TypeCount
	if err := r.db.Model(&Backup{}).Select("backup_type, count(*) as count").Group("backup_type").Find(&typeCounts).Error; err != nil {
		return nil, nil, err
	}

	// Count by server
	type ServerCount struct {
		ServerName string `gorm:"column:server_name"`
		Count      int
	}
	var serverCounts []ServerCount
	if err := r.db.Model(&Backup{}).Select("server_name, count(*) as count").Group("server_name").Find(&serverCounts).Error; err != nil {
		return nil, nil, err
	}

	// Count by database
	type DatabaseCount struct {
		DatabaseName string `gorm:"column:database_name"`
		Count        int
	}
	var databaseCounts []DatabaseCount
	if err := r.db.Model(&Backup{}).Select("database_name, count(*) as count").Group("database_name").Find(&databaseCounts).Error; err != nil {
		return nil, nil, err
	}

	// Last successful backup time
	var lastBackup Backup
	lastBackupTime := r.db.Model(&Backup{}).Where("status = ?", "success").Order("completed_at DESC").Limit(1).Find(&lastBackup)
	if lastBackupTime.Error != nil && lastBackupTime.Error != gorm.ErrRecordNotFound {
		log.Printf("Error finding last backup time: %v", lastBackupTime.Error)
	}

	// Build the detailed stats map
	detailedStats := map[string]interface{}{
		"totalCount":     len(statusCounts),
		"totalLocalSize": stats.TotalLocalSize,
		"totalS3Size":    stats.TotalS3Size,
		"statusCounts":   make(map[string]int),
		"typeDistribution": make(map[string]int),
		"serverDistribution": make(map[string]int),
		"databaseDistribution": make(map[string]int),
	}

	// Map status counts
	for _, sc := range statusCounts {
		detailedStats["statusCounts"].(map[string]int)[sc.Status] = sc.Count
	}

	// Map type counts
	for _, tc := range typeCounts {
		detailedStats["typeDistribution"].(map[string]int)[tc.BackupType] = tc.Count
	}

	// Map server counts
	for _, sc := range serverCounts {
		detailedStats["serverDistribution"].(map[string]int)[sc.ServerName] = sc.Count
	}

	// Map database counts
	for _, dc := range databaseCounts {
		detailedStats["databaseDistribution"].(map[string]int)[dc.DatabaseName] = dc.Count
	}

	// Add last backup time if available
	if lastBackupTime.RowsAffected > 0 && !lastBackup.CompletedAt.IsZero() {
		detailedStats["lastBackupTime"] = lastBackup.CompletedAt
	}

	return &stats, detailedStats, nil
}

// UpdateMetadataStats updates the metadata stats record
func (r *Repository) UpdateMetadataStats(totalLocalSize, totalS3Size int64) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.db.Model(&MetadataStats{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"total_local_size": totalLocalSize,
		"total_s3_size":    totalS3Size,
		"last_updated":     time.Now(),
	}).Error
}

// RecalculateMetadataStats recalculates and updates the metadata stats
func (r *Repository) RecalculateMetadataStats() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.db.Transaction(func(tx *gorm.DB) error {
		// Calculate total local size
		var totalLocalSize int64
		err := tx.Model(&Backup{}).
			Where("status = ?", "success").
			Select("COALESCE(SUM(size), 0)").
			Scan(&totalLocalSize).Error
		if err != nil {
			return err
		}

		// Calculate total S3 size
		var totalS3Size int64
		err = tx.Model(&Backup{}).
			Where("s3_upload_status = ?", "success").
			Select("COALESCE(SUM(size), 0)").
			Scan(&totalS3Size).Error
		if err != nil {
			return err
		}

		// Update stats
		return tx.Model(&MetadataStats{}).Where("id = ?", 1).Updates(map[string]interface{}{
			"total_local_size": totalLocalSize,
			"total_s3_size":    totalS3Size,
			"last_updated":     time.Now(),
		}).Error
	})
}

// PurgeDeletedBackups removes backup records that have been marked as deleted and are older than the specified duration
func (r *Repository) PurgeDeletedBackups(olderThan time.Duration) (int64, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	threshold := time.Now().Add(-olderThan)
	
	// Count records that will be deleted
	var count int64
	if err := r.db.Model(&Backup{}).
		Where("status = ? AND completed_at < ?", "deleted", threshold).
		Count(&count).Error; err != nil {
		return 0, err
	}
	
	// Delete the records if any exist
	if count > 0 {
		// Using unscoped delete to remove the related records via cascading deletes
		result := r.db.Unscoped().
			Where("status = ? AND completed_at < ?", "deleted", threshold).
			Delete(&Backup{})
		
		if result.Error != nil {
			return 0, result.Error
		}
	}

	return count, nil
}

// ImportBackups imports a batch of backups into the database
func (r *Repository) ImportBackups(backups []Backup) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.db.Transaction(func(tx *gorm.DB) error {
		for i := range backups {
			// Check if backup already exists
			var count int64
			if err := tx.Model(&Backup{}).Where("id = ?", backups[i].ID).Count(&count).Error; err != nil {
				return err
			}

			// Skip if already exists
			if count > 0 {
				continue
			}

			// Extract related records
			localPaths := backups[i].LocalPaths
			s3Keys := backups[i].S3Keys

			// Clear related records for creation
			backups[i].LocalPaths = nil
			backups[i].S3Keys = nil

			// Create the backup record
			if err := tx.Create(&backups[i]).Error; err != nil {
				return err
			}

			// Create local paths
			for j := range localPaths {
				localPaths[j].BackupID = backups[i].ID
				if err := tx.Create(&localPaths[j]).Error; err != nil {
					return err
				}
			}

			// Create S3 keys
			for j := range s3Keys {
				s3Keys[j].BackupID = backups[i].ID
				if err := tx.Create(&s3Keys[j]).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}
