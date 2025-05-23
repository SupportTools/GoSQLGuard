// Package metadata provides support for MySQL-backed metadata storage
package metadata

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global database instance
var DB *gorm.DB

// DatabaseBackup represents a database backup record in MySQL
type DatabaseBackup struct {
	ID              string     `gorm:"primaryKey;type:varchar(255)"`
	ServerName      string     `gorm:"type:varchar(255);not null;index"`
	ServerType      string     `gorm:"type:varchar(50);not null"`
	DatabaseName    string     `gorm:"column:database_name;type:varchar(255);not null;index"`
	BackupType      string     `gorm:"type:varchar(50);not null;index"`
	CreatedAt       time.Time  `gorm:"not null"`
	CompletedAt     *time.Time
	Size            int64
	Status          string     `gorm:"type:varchar(50);not null;index"`
	ErrorMessage    string     `gorm:"type:text"`
	RetentionPolicy string     `gorm:"type:varchar(255)"`
	ExpiresAt       *time.Time
	LogFilePath     string     `gorm:"type:varchar(1024)"`
	S3UploadStatus  string     `gorm:"type:varchar(50)"`
	S3UploadError   string     `gorm:"type:text"`
	S3UploadComplete *time.Time

	// Relationships
	LocalPaths []DatabaseLocalPath `gorm:"foreignKey:BackupID;constraint:OnDelete:CASCADE"`
	S3Keys     []DatabaseS3Key     `gorm:"foreignKey:BackupID;constraint:OnDelete:CASCADE"`
}

// TableName specifies the table name for the DatabaseBackup model
func (DatabaseBackup) TableName() string {
	return "backups"
}

// DatabaseLocalPath represents a local file path for a backup
type DatabaseLocalPath struct {
	BackupID     string `gorm:"primaryKey;type:varchar(255)"`
	Organization string `gorm:"primaryKey;type:varchar(50)"`
	Path         string `gorm:"type:varchar(1024);not null"`
}

// TableName specifies the table name for the DatabaseLocalPath model
func (DatabaseLocalPath) TableName() string {
	return "local_paths"
}

// DatabaseS3Key represents an S3 object key for a backup
type DatabaseS3Key struct {
	BackupID     string `gorm:"primaryKey;type:varchar(255)"`
	Organization string `gorm:"primaryKey;type:varchar(50)"`
	Key          string `gorm:"column:s3_key;type:varchar(1024);not null"`
}

// TableName specifies the table name for the DatabaseS3Key model
func (DatabaseS3Key) TableName() string {
	return "s3_keys"
}

// MetadataDBStats represents global metadata statistics
type MetadataDBStats struct {
	ID             uint      `gorm:"primaryKey;autoIncrement:false;default:1"`
	TotalLocalSize int64     `gorm:"not null;default:0"`
	TotalS3Size    int64     `gorm:"not null;default:0"`
	LastUpdated    time.Time `gorm:"not null"`
	Version        string    `gorm:"type:varchar(50);not null"`
}

// TableName specifies the table name for the MetadataDBStats model
func (MetadataDBStats) TableName() string {
	return "metadata_stats"
}

// DBStore implements metadata storage using MySQL
type DBStore struct {
	db           *gorm.DB
	mutex        sync.RWMutex
	filepath     string // For backward compatibility
	s3Key        string // For backward compatibility
	initialized  bool
}

// InitializeMetadataDatabase initializes the metadata database
func InitializeMetadataDatabase() error {
	// If database isn't enabled, use file-based storage
	if !config.CFG.MetadataDB.Enabled {
		log.Println("Metadata database is not enabled, using file-based storage")
		return Initialize()
	}

	// Connect to the database
	db, err := connect()
	if err != nil {
		log.Printf("Failed to connect to metadata database: %v", err)
		log.Println("Falling back to file-based metadata")
		return Initialize()
	}
	DB = db

	// Run auto-migrations if enabled
	if config.CFG.MetadataDB.AutoMigrate {
		log.Println("Running database migrations for metadata tables")
		if err := runMigrations(db); err != nil {
			log.Printf("Failed to run migrations: %v", err)
			log.Println("Falling back to file-based metadata")
			return Initialize()
		}
	}

	// Create the database store
	dbStore := &DBStore{
		db:          db,
		initialized: true,
	}

	// Set file paths for compatibility
	if config.CFG.Local.Enabled {
		dbStore.filepath = filepath.Join(config.CFG.Local.BackupDirectory, "metadata.json")
	}

	// Set S3 key
	if config.CFG.S3.Enabled {
		dbStore.s3Key = filepath.Join(config.CFG.S3.Prefix, "metadata.json")
	}

	// If there's an existing file-based store, migrate data
	if migrateFromFile(dbStore) != nil {
		// If migration fails, log but continue using DB
		log.Println("Warning: Failed to migrate data from existing metadata file")
	}

	// Set as the default store
	DefaultStore = dbStore

	// Add performance indexes
	if err := AddPerformanceIndexes(db); err != nil {
		log.Printf("Warning: Failed to add performance indexes: %v", err)
		// Continue anyway - indexes are for optimization only
	}

	log.Println("Using MySQL-backed metadata store")
	return nil
}

// connect establishes a connection to the MySQL database
func connect() (*gorm.DB, error) {
	cfg := config.CFG.MetadataDB

	// Build DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	// Set up logger config based on debug mode
	logLevel := logger.Silent
	if config.CFG.Debug {
		logLevel = logger.Info
	}

	// Connect to database
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)

	// Parse connection max lifetime
	if cfg.ConnMaxLifetime != "" {
		duration, err := time.ParseDuration(cfg.ConnMaxLifetime)
		if err != nil {
			log.Printf("Warning: Invalid connection max lifetime '%s', using default 5m: %v", 
				cfg.ConnMaxLifetime, err)
			duration = 5 * time.Minute
		}
		sqlDB.SetConnMaxLifetime(duration)
	}

	log.Printf("Connected to metadata database at %s:%d", cfg.Host, cfg.Port)
	return db, nil
}

// runMigrations runs all necessary database migrations
func runMigrations(db *gorm.DB) error {
	// Create the tables if they don't exist
	err := db.AutoMigrate(
		&DatabaseBackup{},
		&DatabaseLocalPath{},
		&DatabaseS3Key{},
		&MetadataDBStats{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate tables: %w", err)
	}

	// Initialize stats record if it doesn't exist
	var count int64
	db.Model(&MetadataDBStats{}).Count(&count)
	if count == 0 {
		log.Println("Initializing metadata stats record")
		stats := MetadataDBStats{
			ID:          1,
			Version:     "1.0",
			LastUpdated: time.Now(),
		}
		if err := db.Create(&stats).Error; err != nil {
			return fmt.Errorf("failed to create initial stats record: %w", err)
		}
	}

	return nil
}

// migrateFromFile attempts to migrate data from an existing metadata file
func migrateFromFile(dbStore *DBStore) error {
	// Check if file exists
	if _, err := os.Stat(dbStore.filepath); os.IsNotExist(err) {
		// No file to migrate from
		return nil
	}

	// Create a temporary file store
	fileStore := &Store{
		metadata: MetadataStore{
			Backups:     make([]types.BackupMeta, 0),
			LastUpdated: time.Now(),
			Version:     "1.0",
		},
		filepath: dbStore.filepath,
	}

	// Load data from file
	if err := fileStore.Load(); err != nil {
		return fmt.Errorf("failed to load existing metadata file: %w", err)
	}

	// Get all backups
	fileBackups := fileStore.GetBackups()
	log.Printf("Migrating %d backup records from file-based store to database", len(fileBackups))

	// Start a transaction
	tx := dbStore.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// Import each backup
	migrated := 0
	for _, fb := range fileBackups {
		// Check if this backup already exists in the database
		var count int64
		if err := tx.Model(&DatabaseBackup{}).Where("id = ?", fb.ID).Count(&count).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to check existing backup: %w", err)
		}

		// Skip if already exists
		if count > 0 {
			continue
		}

		// Create backup record
		backup := DatabaseBackup{
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

		// Set times that might be zero
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

		// Create the main backup record
		if err := tx.Create(&backup).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create backup record: %w", err)
		}

		// Add local paths
		for org, path := range fb.LocalPaths {
			localPath := DatabaseLocalPath{
				BackupID:     fb.ID,
				Organization: org,
				Path:         path,
			}
			if err := tx.Create(&localPath).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create local path: %w", err)
			}
		}

		// Add S3 keys
		for org, key := range fb.S3Keys {
			s3Key := DatabaseS3Key{
				BackupID:     fb.ID,
				Organization: org,
				Key:          key,
			}
			if err := tx.Create(&s3Key).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create S3 key: %w", err)
			}
		}

		migrated++
	}

	// Calculate statistics
	var totalLocalSize, totalS3Size int64
	err := tx.Model(&DatabaseBackup{}).
		Where("status = ?", string(StatusSuccess)).
		Select("COALESCE(SUM(size), 0)").
		Scan(&totalLocalSize).Error
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to calculate local size: %w", err)
	}

	err = tx.Model(&DatabaseBackup{}).
		Where("s3_upload_status = ?", string(StatusSuccess)).
		Select("COALESCE(SUM(size), 0)").
		Scan(&totalS3Size).Error
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to calculate S3 size: %w", err)
	}

	// Update stats
	err = tx.Model(&MetadataDBStats{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"total_local_size": totalLocalSize,
		"total_s3_size":    totalS3Size,
		"last_updated":     time.Now(),
	}).Error
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update stats: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Successfully migrated %d backup records to database", migrated)
	return nil
}

// CreateBackupMeta creates a new backup metadata entry
func (s *DBStore) CreateBackupMeta(serverName, serverType, database, backupType string) *types.BackupMeta {
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
	dbBackup := DatabaseBackup{
		ID:              id,
		ServerName:      serverName,
		ServerType:      serverType,
		DatabaseName:    database,
		BackupType:      backupType,
		CreatedAt:       time.Now(),
		Status:          string(StatusPending),
		S3UploadStatus:  string(StatusPending),
		RetentionPolicy: retentionText,
	}

	if !expiresAt.IsZero() {
		expiresCopy := expiresAt
		dbBackup.ExpiresAt = &expiresCopy
	}

	err := s.db.Create(&dbBackup).Error
	if err != nil {
		log.Printf("Error creating backup record in database: %v", err)
	}

	// Convert to types.BackupMeta for compatibility
	meta := &types.BackupMeta{
		ID:              id,
		ServerName:      serverName,
		ServerType:      serverType,
		Database:        database,
		BackupType:      backupType,
		CreatedAt:       time.Now(),
		Status:          StatusPending,
		S3UploadStatus:  StatusPending,
		RetentionPolicy: retentionText,
		ExpiresAt:       expiresAt,
		LocalPaths:      make(map[string]string),
		S3Keys:          make(map[string]string),
	}

	return meta
}

// UpdateBackupStatus updates the status of a backup
func (s *DBStore) UpdateBackupStatus(id string, status types.BackupStatus, localPaths map[string]string, size int64, errorMsg string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Start a transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// Update the backup status
	now := time.Now()
	updates := map[string]interface{}{
		"status":        string(status),
		"size":          size,
		"error_message": errorMsg,
		"completed_at":  now,
	}

	if err := tx.Model(&DatabaseBackup{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Delete existing local paths if any
	if err := tx.Where("backup_id = ?", id).Delete(&DatabaseLocalPath{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Add new local paths
	for org, path := range localPaths {
		localPath := DatabaseLocalPath{
			BackupID:     id,
			Organization: org,
			Path:         path,
		}
		if err := tx.Create(&localPath).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// Recalculate stats
	if err := s.updateStats(tx); err != nil {
		tx.Rollback()
		return err
	}

	// Commit transaction
	return tx.Commit().Error
}

// UpdateS3UploadStatus updates the S3 upload status of a backup
func (s *DBStore) UpdateS3UploadStatus(id string, status types.BackupStatus, s3Keys map[string]string, errorMsg string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Start a transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// Update the S3 upload status
	now := time.Now()
	updates := map[string]interface{}{
		"s3_upload_status":   string(status),
		"s3_upload_error":    errorMsg,
		"s3_upload_complete": now,
	}

	if err := tx.Model(&DatabaseBackup{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Delete existing S3 keys if any
	if err := tx.Where("backup_id = ?", id).Delete(&DatabaseS3Key{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Add new S3 keys
	for org, key := range s3Keys {
		s3Key := DatabaseS3Key{
			BackupID:     id,
			Organization: org,
			Key:          key,
		}
		if err := tx.Create(&s3Key).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// Recalculate stats
	if err := s.updateStats(tx); err != nil {
		tx.Rollback()
		return err
	}

	// Commit transaction
	return tx.Commit().Error
}

// updateStats recalculates and updates the metadata stats
func (s *DBStore) updateStats(tx *gorm.DB) error {
	// Calculate total local size
	var totalLocalSize int64
	err := tx.Model(&DatabaseBackup{}).
		Where("status = ?", string(StatusSuccess)).
		Select("COALESCE(SUM(size), 0)").
		Scan(&totalLocalSize).Error
	if err != nil {
		return fmt.Errorf("failed to calculate local size: %w", err)
	}

	// Calculate total S3 size
	var totalS3Size int64
	err = tx.Model(&DatabaseBackup{}).
		Where("s3_upload_status = ?", string(StatusSuccess)).
		Select("COALESCE(SUM(size), 0)").
		Scan(&totalS3Size).Error
	if err != nil {
		return fmt.Errorf("failed to calculate S3 size: %w", err)
	}

	// Update stats
	return tx.Model(&MetadataDBStats{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"total_local_size": totalLocalSize,
		"total_s3_size":    totalS3Size,
		"last_updated":     time.Now(),
	}).Error
}

// UpdateLogFilePath updates the log file path for a backup
func (s *DBStore) UpdateLogFilePath(id string, logFilePath string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.db.Model(&DatabaseBackup{}).Where("id = ?", id).Update("log_file_path", logFilePath).Error
}

// GetBackups returns all backups
func (s *DBStore) GetBackups() []types.BackupMeta {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var dbBackups []DatabaseBackup
	if err := s.db.Preload("LocalPaths").Preload("S3Keys").Find(&dbBackups).Error; err != nil {
		log.Printf("Error retrieving backups from database: %v", err)
		return []types.BackupMeta{}
	}

	return convertToBackupMetas(dbBackups)
}

// GetBackupsFiltered returns backups filtered by server, database and/or type
func (s *DBStore) GetBackupsFiltered(serverName, database, backupType string, activeOnly bool) []types.BackupMeta {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Build the query
	query := s.db.Model(&DatabaseBackup{}).Preload("LocalPaths").Preload("S3Keys")

	if serverName != "" {
		query = query.Where("server_name = ?", serverName)
	}

	if database != "" {
		query = query.Where("database_name = ?", database)
	}

	if backupType != "" {
		query = query.Where("backup_type = ?", backupType)
	}

	if activeOnly {
		query = query.Where("status = ?", string(StatusSuccess))
	}

	// Execute the query
	var dbBackups []DatabaseBackup
	if err := query.Find(&dbBackups).Error; err != nil {
		log.Printf("Error retrieving filtered backups from database: %v", err)
		return []types.BackupMeta{}
	}

	return convertToBackupMetas(dbBackups)
}

// GetBackupByID returns a specific backup by ID
func (s *DBStore) GetBackupByID(id string) (types.BackupMeta, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var dbBackup DatabaseBackup
	if err := s.db.Preload("LocalPaths").Preload("S3Keys").Where("id = ?", id).First(&dbBackup).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return types.BackupMeta{}, false
		}
		log.Printf("Error retrieving backup by ID from database: %v", err)
		return types.BackupMeta{}, false
	}

	backups := convertToBackupMetas([]DatabaseBackup{dbBackup})
	if len(backups) == 0 {
		return types.BackupMeta{}, false
	}

	return backups[0], true
}

// MarkBackupDeleted marks a backup as deleted
func (s *DBStore) MarkBackupDeleted(id string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.db.Model(&DatabaseBackup{}).Where("id = ?", id).Update("status", string(StatusDeleted)).Error
}

// GetStats returns statistics about the backups
func (s *DBStore) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Get the stats record
	var stats MetadataDBStats
	if err := s.db.First(&stats, 1).Error; err != nil {
		log.Printf("Error retrieving metadata stats from database: %v", err)
		return createEmptyStats()
	}

	// Initialize stats map
	result := map[string]interface{}{
		"totalLocalSize": stats.TotalLocalSize,
		"totalS3Size":    stats.TotalS3Size,
		"statusCounts": map[string]int{
			"success": 0,
			"pending": 0,
			"error":   0,
			"deleted": 0,
		},
		"typeDistribution":     make(map[string]int),
		"serverDistribution":   make(map[string]int),
		"databaseDistribution": make(map[string]int),
	}

	// Count by status
	var statusCounts []struct {
		Status string
		Count  int
	}
	if err := s.db.Model(&DatabaseBackup{}).
		Select("status, count(*) as count").
		Group("status").
		Find(&statusCounts).Error; err != nil {
		log.Printf("Error getting status counts: %v", err)
	} else {
		for _, sc := range statusCounts {
			result["statusCounts"].(map[string]int)[sc.Status] = sc.Count
		}
	}

	// Count by type
	var typeCounts []struct {
		BackupType string `gorm:"column:backup_type"`
		Count      int
	}
	if err := s.db.Model(&DatabaseBackup{}).
		Select("backup_type, count(*) as count").
		Group("backup_type").
		Find(&typeCounts).Error; err != nil {
		log.Printf("Error getting type counts: %v", err)
	} else {
		for _, tc := range typeCounts {
			result["typeDistribution"].(map[string]int)[tc.BackupType] = tc.Count
		}
	}

	// Count by server
	var serverCounts []struct {
		ServerName string `gorm:"column:server_name"`
		Count      int
	}
	if err := s.db.Model(&DatabaseBackup{}).
		Select("server_name, count(*) as count").
		Group("server_name").
		Find(&serverCounts).Error; err != nil {
		log.Printf("Error getting server counts: %v", err)
	} else {
		for _, sc := range serverCounts {
			result["serverDistribution"].(map[string]int)[sc.ServerName] = sc.Count
		}
	}

	// Count by database
	var databaseCounts []struct {
		DatabaseName string `gorm:"column:database_name"`
		Count        int
	}
	if err := s.db.Model(&DatabaseBackup{}).
		Select("database_name, count(*) as count").
		Group("database_name").
		Find(&databaseCounts).Error; err != nil {
		log.Printf("Error getting database counts: %v", err)
	} else {
		for _, dc := range databaseCounts {
			result["databaseDistribution"].(map[string]int)[dc.DatabaseName] = dc.Count
		}
	}

	// Get last backup time
	var lastBackup DatabaseBackup
	if err := s.db.Model(&DatabaseBackup{}).
		Where("status = ?", string(StatusSuccess)).
		Order("completed_at DESC").
		Limit(1).
		First(&lastBackup).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			log.Printf("Error getting last backup time: %v", err)
		}
	} else if lastBackup.CompletedAt != nil {
		result["lastBackupTime"] = *lastBackup.CompletedAt
	}

	// Get total count
	var count int64
	if err := s.db.Model(&DatabaseBackup{}).Count(&count).Error; err != nil {
		log.Printf("Error getting total count: %v", err)
	} else {
		result["totalCount"] = count
	}

	return result
}

// createEmptyStats creates an empty stats map for error cases
func createEmptyStats() map[string]interface{} {
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
		"typeDistribution":     make(map[string]int),
		"serverDistribution":   make(map[string]int),
		"databaseDistribution": make(map[string]int),
	}
}

// PurgeDeletedBackups removes backup records that have been marked as deleted
// and are older than the specified duration
func (s *DBStore) PurgeDeletedBackups(olderThan time.Duration) int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	threshold := time.Now().Add(-olderThan)

	// Start a transaction
	tx := s.db.Begin()
	if tx.Error != nil {
		log.Printf("Failed to start transaction: %v", tx.Error)
		return 0
	}

	// Count records to delete
	var count int64
	if err := tx.Model(&DatabaseBackup{}).
		Where("status = ? AND completed_at < ?", string(StatusDeleted), threshold).
		Count(&count).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to count records to delete: %v", err)
		return 0
	}

	if count == 0 {
		tx.Rollback()
		return 0
	}

	// Delete the records
	if err := tx.Unscoped().
		Where("status = ? AND completed_at < ?", string(StatusDeleted), threshold).
		Delete(&DatabaseBackup{}).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to delete records: %v", err)
		return 0
	}

	// Recalculate stats after deletion
	if err := s.updateStats(tx); err != nil {
		tx.Rollback()
		log.Printf("Failed to update stats after deletion: %v", err)
		return 0
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		return 0
	}

	return int(count)
}

// Load loads the metadata (not needed for DB implementation, but kept for compatibility)
func (s *DBStore) Load() error {
	// No-op for database implementation
	return nil
}

// Save persists the metadata (not needed for DB implementation, but kept for compatibility)
func (s *DBStore) Save() error {
	// No-op for database implementation
	return nil
}

// convertToBackupMetas converts database model backups to the original format
func convertToBackupMetas(dbBackups []DatabaseBackup) []types.BackupMeta {
	result := make([]types.BackupMeta, 0, len(dbBackups))
	
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
		
		result = append(result, backup)
	}
	
	return result
}
