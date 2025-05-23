// Package configtypes defines types used for database configuration
package configtypes

// ServerConfig represents a database server configuration
type ServerConfig struct {
	ID              string
	Name            string
	Type            string
	Host            string
	Port            string
	Username        string
	Password        string
	AuthPlugin      string
	IncludeDatabases []string
	ExcludeDatabases []string
	// MySQL options would be added here
}

// ScheduleConfig represents a backup schedule configuration
type ScheduleConfig struct {
	ID             string
	Name           string
	BackupType     string
	CronExpression string
	Enabled        bool
	LocalStorage   StorageConfig
	S3Storage      StorageConfig
}

// StorageConfig represents storage configuration for a backup schedule
type StorageConfig struct {
	Enabled     bool
	Duration    string
	KeepForever bool
}
