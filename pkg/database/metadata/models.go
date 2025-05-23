// Package metadata provides database models and operations for backup metadata
package metadata

import (
	"time"
)

// ServerConfig represents a database server configuration
type ServerConfig struct {
	ID              string    `gorm:"primaryKey;type:varchar(255)"`
	Name            string    `gorm:"type:varchar(255);not null;uniqueIndex"`
	Type            string    `gorm:"type:varchar(50);not null"` // mysql or postgresql
	Host            string    `gorm:"type:varchar(255);not null"`
	Port            string    `gorm:"type:varchar(10);not null"`
	Username        string    `gorm:"type:varchar(255);not null"`
	Password        string    `gorm:"type:varchar(255);not null"`
	AuthPlugin      string    `gorm:"type:varchar(100)"`
	CreatedAt       time.Time `gorm:"not null"`
	UpdatedAt       time.Time `gorm:"not null"`

	// Relationships
	DatabaseFilters []ServerDatabaseFilter `gorm:"foreignKey:ServerID;constraint:OnDelete:CASCADE"`
	MySQLOptions    []ServerMySQLOption    `gorm:"foreignKey:ServerID;constraint:OnDelete:CASCADE"`
}

// TableName specifies the table name for the ServerConfig model
func (ServerConfig) TableName() string {
	return "server_configs"
}

// ServerDatabaseFilter represents include/exclude database filters for a server
type ServerDatabaseFilter struct {
	ID           uint      `gorm:"primaryKey;autoIncrement"`
	ServerID     string    `gorm:"type:varchar(255);not null;index"`
	FilterType   string    `gorm:"type:varchar(10);not null"` // include or exclude
	DatabaseName string    `gorm:"type:varchar(255);not null"`
	CreatedAt    time.Time `gorm:"not null"`
}

// TableName specifies the table name for the ServerDatabaseFilter model
func (ServerDatabaseFilter) TableName() string {
	return "server_database_filters"
}

// ServerMySQLOption represents MySQL-specific options for a server
type ServerMySQLOption struct {
	ID         uint      `gorm:"primaryKey;autoIncrement"`
	ServerID   string    `gorm:"type:varchar(255);not null;index"`
	OptionName string    `gorm:"type:varchar(100);not null"`
	OptionValue string   `gorm:"type:varchar(255)"`
	CreatedAt  time.Time `gorm:"not null"`
}

// TableName specifies the table name for the ServerMySQLOption model
func (ServerMySQLOption) TableName() string {
	return "server_mysql_options"
}

// BackupSchedule represents a backup schedule configuration
type BackupSchedule struct {
	ID             string    `gorm:"primaryKey;type:varchar(255)"`
	Name           string    `gorm:"type:varchar(100);not null;uniqueIndex"`
	BackupType     string    `gorm:"type:varchar(50);not null"`
	CronExpression string    `gorm:"type:varchar(100);not null"`
	Enabled        bool      `gorm:"not null;default:true"`
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`

	// Relationships
	RetentionPolicies []ScheduleRetentionPolicy `gorm:"foreignKey:ScheduleID;constraint:OnDelete:CASCADE"`
}

// TableName specifies the table name for the BackupSchedule model
func (BackupSchedule) TableName() string {
	return "backup_schedules"
}

// ScheduleRetentionPolicy represents retention settings for a backup schedule
type ScheduleRetentionPolicy struct {
	ID          uint      `gorm:"primaryKey;autoIncrement"`
	ScheduleID  string    `gorm:"type:varchar(255);not null;index"`
	StorageType string    `gorm:"type:varchar(10);not null"` // local or s3
	Duration    string    `gorm:"type:varchar(50)"`
	KeepForever bool      `gorm:"not null;default:false"`
	CreatedAt   time.Time `gorm:"not null"`
}

// TableName specifies the table name for the ScheduleRetentionPolicy model
func (ScheduleRetentionPolicy) TableName() string {
	return "schedule_retention_policies"
}

// Backup represents a database backup record
type Backup struct {
	ID              string    `gorm:"primaryKey;type:varchar(255)"`
	ServerName      string    `gorm:"type:varchar(255);not null;index"`
	ServerType      string    `gorm:"type:varchar(50);not null"`
	DatabaseName    string    `gorm:"column:database_name;type:varchar(255);not null;index"`
	BackupType      string    `gorm:"type:varchar(50);not null;index"`
	CreatedAt       time.Time `gorm:"not null"`
	CompletedAt     *time.Time
	Size            int64
	Status          string    `gorm:"type:varchar(50);not null;index"`
	ErrorMessage    string    `gorm:"type:text"`
	RetentionPolicy string    `gorm:"type:varchar(255)"`
	ExpiresAt       *time.Time
	LogFilePath     string    `gorm:"type:varchar(1024)"`
	S3UploadStatus  string    `gorm:"type:varchar(50)"`
	S3UploadError   string    `gorm:"type:text"`
	S3UploadComplete *time.Time

	// Relationships
	LocalPaths []LocalPath `gorm:"foreignKey:BackupID;constraint:OnDelete:CASCADE"`
	S3Keys     []S3Key     `gorm:"foreignKey:BackupID;constraint:OnDelete:CASCADE"`
}

// TableName specifies the table name for the Backup model
func (Backup) TableName() string {
	return "backups"
}

// LocalPath represents a local file path for a backup
type LocalPath struct {
	BackupID     string `gorm:"primaryKey;type:varchar(255)"`
	Organization string `gorm:"primaryKey;type:varchar(50)"`
	Path         string `gorm:"type:varchar(1024);not null"`
}

// TableName specifies the table name for the LocalPath model
func (LocalPath) TableName() string {
	return "local_paths"
}

// S3Key represents an S3 object key for a backup
type S3Key struct {
	BackupID     string `gorm:"primaryKey;type:varchar(255)"`
	Organization string `gorm:"primaryKey;type:varchar(50)"`
	Key          string `gorm:"column:s3_key;type:varchar(1024);not null"`
}

// TableName specifies the table name for the S3Key model
func (S3Key) TableName() string {
	return "s3_keys"
}

// MetadataStats represents global metadata statistics
type MetadataStats struct {
	ID             uint      `gorm:"primaryKey;autoIncrement:false;default:1"`
	TotalLocalSize int64     `gorm:"not null;default:0"`
	TotalS3Size    int64     `gorm:"not null;default:0"`
	LastUpdated    time.Time `gorm:"not null"`
	Version        string    `gorm:"type:varchar(50);not null"`
}

// TableName specifies the table name for the MetadataStats model
func (MetadataStats) TableName() string {
	return "metadata_stats"
}
