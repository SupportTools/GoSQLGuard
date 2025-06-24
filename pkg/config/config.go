// Package config provides configuration loading and management for GoSQLGuard
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// MySQLConfig defines MySQL connection settings
type MySQLConfig struct {
	Host             string   `yaml:"host"`
	Port             string   `yaml:"port"`
	Username         string   `yaml:"username"`
	Password         string   `yaml:"password"`
	IncludeDatabases []string `yaml:"includeDatabases"`
	ExcludeDatabases []string `yaml:"excludeDatabases"`
}

// PostgreSQLConfig defines PostgreSQL connection settings
type PostgreSQLConfig struct {
	Host      string   `yaml:"host"`
	Port      string   `yaml:"port"`
	Username  string   `yaml:"username"`
	Password  string   `yaml:"password"`
	Databases []string `yaml:"databases"`
}

// MySQLDumpOptionsConfig is a placeholder to maintain compatibility
// All actual mysqldump options are now hardcoded in the MySQL provider
type MySQLDumpOptionsConfig struct {
	SingleTransaction  bool     `yaml:"-"`
	Quick              bool     `yaml:"-"`
	SkipLockTables     bool     `yaml:"-"`
	SkipAddLocks       bool     `yaml:"-"`
	SkipComments       bool     `yaml:"-"`
	ExtendedInsert     bool     `yaml:"-"`
	SkipExtendedInsert bool     `yaml:"-"`
	Compress           bool     `yaml:"-"`
	CustomOptions      []string `yaml:"-"`
}

// PostgreSQLDumpOptionsConfig defines configuration for pg_dump options
type PostgreSQLDumpOptionsConfig struct {
	Format              string   `yaml:"format"`
	Verbose             bool     `yaml:"verbose"`
	NoComments          bool     `yaml:"noComments"`
	SchemaOnly          bool     `yaml:"schemaOnly"`
	DataOnly            bool     `yaml:"dataOnly"`
	Blobs               bool     `yaml:"blobs"`
	NoBlobs             bool     `yaml:"noBlobs"`
	Clean               bool     `yaml:"clean"`
	Create              bool     `yaml:"create"`
	IfExists            bool     `yaml:"ifExists"`
	NoOwner             bool     `yaml:"noOwner"`
	NoPrivileges        bool     `yaml:"noPrivileges"`
	NoTablespaces       bool     `yaml:"noTablespaces"`
	NoPassword          bool     `yaml:"noPassword"`
	InsertColumns       bool     `yaml:"insertColumns"`
	OnConflictDoNothing bool     `yaml:"onConflictDoNothing"`
	Jobs                int      `yaml:"jobs"`
	Compress            int      `yaml:"compress"`
	CustomOptions       []string `yaml:"customOptions"`
}

// DatabaseServerConfig defines configuration for a single database server
type DatabaseServerConfig struct {
	Name             string                 `yaml:"name"`
	Type             string                 `yaml:"type"` // mysql or postgresql
	Host             string                 `yaml:"host"`
	Port             string                 `yaml:"port"`
	Username         string                 `yaml:"username"`
	Password         string                 `yaml:"password"`
	AuthPlugin       string                 `yaml:"authPlugin,omitempty"` // MySQL authentication plugin (mysql_native_password, caching_sha2_password)
	IncludeDatabases []string               `yaml:"includeDatabases"`
	ExcludeDatabases []string               `yaml:"excludeDatabases"`
	MySQLDumpOptions MySQLDumpOptionsConfig `yaml:"mysqlDumpOptions,omitempty"`
	PostgreSQLDumpOptions PostgreSQLDumpOptionsConfig `yaml:"postgresqlDumpOptions,omitempty"`
}

// LocalConfig defines local backup settings
type LocalConfig struct {
	Enabled              bool   `yaml:"enabled"`
	BackupDirectory      string `yaml:"backupDirectory"`
	OrganizationStrategy string `yaml:"organizationStrategy"` // server-only, type-only, combined
}

// S3Config defines S3 storage settings
type S3Config struct {
	Enabled              bool   `yaml:"enabled"`
	Bucket               string `yaml:"bucket"`
	Region               string `yaml:"region"`
	Endpoint             string `yaml:"endpoint"`
	AccessKey            string `yaml:"accessKey"`
	SecretKey            string `yaml:"secretKey"`
	Prefix               string `yaml:"prefix"`
	PathStyle            bool   `yaml:"pathStyle"` // Use path-style access for S3
	UseSSL               bool   `yaml:"useSSL"`
	CustomCAPath         string `yaml:"customCAPath"`         // Path to custom CA certificate
	SkipCertValidation   bool   `yaml:"skipCertValidation"`   // Skip certificate validation
	OrganizationStrategy string `yaml:"organizationStrategy"` // server-only, type-only, combined
}

// MetadataDBConfig defines MySQL connection settings for metadata database
type MetadataDBConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	Database        string `yaml:"database"`
	MaxOpenConns    int    `yaml:"maxOpenConns"`
	MaxIdleConns    int    `yaml:"maxIdleConns"`
	ConnMaxLifetime string `yaml:"connMaxLifetime"`
	AutoMigrate     bool   `yaml:"autoMigrate"`
}

// MetricsConfig defines metrics server settings
type MetricsConfig struct {
	Port string `yaml:"port"`
}

// RetentionRule defines retention policy rules
type RetentionRule struct {
	Duration string `yaml:"duration"`
	Forever  bool   `yaml:"forever"`
}

// LocalBackupConfig defines local storage settings for a backup type
type LocalBackupConfig struct {
	Enabled   bool          `yaml:"enabled"`
	Retention RetentionRule `yaml:"retention"`
}

// S3BackupConfig defines S3 storage settings for a backup type
type S3BackupConfig struct {
	Enabled   bool          `yaml:"enabled"`
	Retention RetentionRule `yaml:"retention"`
}

// BackupTypeConfig defines configuration for a specific backup type
type BackupTypeConfig struct {
	Schedule         string                 `yaml:"schedule"` // Cron schedule format
	Local            LocalBackupConfig      `yaml:"local"`
	S3               S3BackupConfig         `yaml:"s3"`
	MySQLDumpOptions MySQLDumpOptionsConfig `yaml:"mysqlDumpOptions,omitempty"`
}

// AppConfig contains the complete application configuration
type AppConfig struct {
	// Legacy single-server configuration (for backward compatibility)
	MySQL      MySQLConfig      `yaml:"mysql"`
	PostgreSQL PostgreSQLConfig `yaml:"postgresql"`

	// Multi-server configuration
	DatabaseServers []DatabaseServerConfig `yaml:"database_servers"`

	Local            LocalConfig                 `yaml:"local"`
	S3               S3Config                    `yaml:"s3"`
	Metrics          MetricsConfig               `yaml:"metrics"`
	MetadataDB       MetadataDBConfig            `yaml:"metadata_database"`
	BackupTypes      map[string]BackupTypeConfig `yaml:"backupTypes"`
	MySQLDumpOptions MySQLDumpOptionsConfig      `yaml:"mysqlDumpOptions,omitempty"` // Default MySQL dump options
	PostgreSQLDumpOptions PostgreSQLDumpOptionsConfig `yaml:"postgresqlDumpOptions,omitempty"` // Default PostgreSQL dump options
	Debug            bool                        `yaml:"debug"`
	ConfigFile       string                      `json:"configFile,omitempty"`
}

// CFG is the global configuration object
var CFG AppConfig

// LoadConfiguration loads configuration from environment variables only
func LoadConfiguration() {
	log.Println("Loading configuration from environment variables...")
	loadFromEnvironment()
}


// loadFromEnvironment loads configuration from environment variables
func loadFromEnvironment() {
	// Debug setting
	CFG.Debug = parseEnvBool("DEBUG", false)

	// Local backup settings
	CFG.Local.Enabled = parseEnvBool("LOCAL_BACKUP_ENABLED", true)
	CFG.Local.BackupDirectory = getEnvOrDefault("LOCAL_BACKUP_DIRECTORY", "/backups")

	// S3 settings
	CFG.S3.Enabled = parseEnvBool("S3_BACKUP_ENABLED", false)
	CFG.S3.Bucket = getEnvOrDefault("S3_BUCKET", "")
	CFG.S3.Region = getEnvOrDefault("S3_REGION", "us-east-1")
	CFG.S3.Endpoint = getEnvOrDefault("S3_ENDPOINT", "")
	CFG.S3.AccessKey = getEnvOrDefault("S3_ACCESS_KEY", "")
	CFG.S3.SecretKey = getEnvOrDefault("S3_SECRET_KEY", "")
	CFG.S3.Prefix = getEnvOrDefault("S3_PREFIX", "mysql-backups")
	CFG.S3.PathStyle = parseEnvBool("S3_PATH_STYLE", false)
	CFG.S3.UseSSL = parseEnvBool("S3_USE_SSL", true)
	CFG.S3.CustomCAPath = getEnvOrDefault("S3_CUSTOM_CA_PATH", "")
	CFG.S3.SkipCertValidation = parseEnvBool("S3_SKIP_CERT_VALIDATION", false)

	// Metadata DB settings
	CFG.MetadataDB.Enabled = parseEnvBool("METADATA_DB_ENABLED", false)
	CFG.MetadataDB.Host = getEnvOrDefault("METADATA_DB_HOST", "localhost")
	if port, err := strconv.Atoi(getEnvOrDefault("METADATA_DB_PORT", "3306")); err == nil {
		CFG.MetadataDB.Port = port
	} else {
		CFG.MetadataDB.Port = 3306
	}
	CFG.MetadataDB.Username = getEnvOrDefault("METADATA_DB_USERNAME", "gosqlguard")
	CFG.MetadataDB.Password = getEnvOrDefault("METADATA_DB_PASSWORD", "")
	CFG.MetadataDB.Database = getEnvOrDefault("METADATA_DB_DATABASE", "gosqlguard_metadata")
	
	if maxOpen, err := strconv.Atoi(getEnvOrDefault("METADATA_DB_MAX_OPEN_CONNS", "10")); err == nil {
		CFG.MetadataDB.MaxOpenConns = maxOpen
	} else {
		CFG.MetadataDB.MaxOpenConns = 10
	}
	
	if maxIdle, err := strconv.Atoi(getEnvOrDefault("METADATA_DB_MAX_IDLE_CONNS", "5")); err == nil {
		CFG.MetadataDB.MaxIdleConns = maxIdle
	} else {
		CFG.MetadataDB.MaxIdleConns = 5
	}
	
	CFG.MetadataDB.ConnMaxLifetime = getEnvOrDefault("METADATA_DB_CONN_MAX_LIFETIME", "5m")
	CFG.MetadataDB.AutoMigrate = parseEnvBool("METADATA_DB_AUTO_MIGRATE", true)

	// Metrics settings
	CFG.Metrics.Port = getEnvOrDefault("METRICS_PORT", "8080")

	// Set organization strategies (optional)
	if orgStrategy := getEnvOrDefault("LOCAL_ORGANIZATION_STRATEGY", ""); orgStrategy != "" {
		CFG.Local.OrganizationStrategy = orgStrategy
	}

	if orgStrategy := getEnvOrDefault("S3_ORGANIZATION_STRATEGY", ""); orgStrategy != "" {
		CFG.S3.OrganizationStrategy = orgStrategy
	}

	// Note: Database servers must be configured via config file
	log.Println("Database configuration can only be loaded from config file, not environment variables")

	setDefaults()

	if CFG.Debug {
		log.Printf("Configuration Loaded from environment: %+v\n", CFG)
	}
}

// setDefaults ensures all config fields have reasonable default values
func setDefaults() {
	if CFG.Metrics.Port == "" {
		CFG.Metrics.Port = "8080"
	}

	// Set default organization strategy
	if CFG.Local.OrganizationStrategy == "" {
		CFG.Local.OrganizationStrategy = "combined" // Default to combined organization
	}

	if CFG.S3.OrganizationStrategy == "" {
		CFG.S3.OrganizationStrategy = "combined" // Default to combined organization
	}
	
	// Set defaults for metadata database if enabled
	if CFG.MetadataDB.Enabled {
		if CFG.MetadataDB.Host == "" {
			CFG.MetadataDB.Host = "localhost"
		}
		if CFG.MetadataDB.Port == 0 {
			CFG.MetadataDB.Port = 3306
		}
		if CFG.MetadataDB.Database == "" {
			CFG.MetadataDB.Database = "gosqlguard_metadata"
		}
		if CFG.MetadataDB.MaxOpenConns == 0 {
			CFG.MetadataDB.MaxOpenConns = 10
		}
		if CFG.MetadataDB.MaxIdleConns == 0 {
			CFG.MetadataDB.MaxIdleConns = 5
		}
		if CFG.MetadataDB.ConnMaxLifetime == "" {
			CFG.MetadataDB.ConnMaxLifetime = "5m"
		}
	}

	// Create database servers from legacy config if no database servers are specified
	if len(CFG.DatabaseServers) == 0 && CFG.MySQL.Host != "" {
		// Create a virtual server from legacy MySQL config
		CFG.DatabaseServers = append(CFG.DatabaseServers, DatabaseServerConfig{
			Name:             "default",
			Type:             "mysql",
			Host:             CFG.MySQL.Host,
			Port:             CFG.MySQL.Port,
			Username:         CFG.MySQL.Username,
			Password:         CFG.MySQL.Password,
			IncludeDatabases: CFG.MySQL.IncludeDatabases,
			ExcludeDatabases: CFG.MySQL.ExcludeDatabases,
			MySQLDumpOptions: CFG.MySQLDumpOptions,
		})
	}

	// Set up default backup types if none are configured
	if len(CFG.BackupTypes) == 0 {
		CFG.BackupTypes = map[string]BackupTypeConfig{
			"manual": {
				Schedule: "", // No schedule - manual only
				Local: LocalBackupConfig{
					Enabled: true,
					Retention: RetentionRule{
						Duration: "90d", // Keep manual backups longer
						Forever:  false,
					},
				},
				S3: S3BackupConfig{
					Enabled: true,
					Retention: RetentionRule{
						Duration: "365d", // Keep manual backups for a year in S3
						Forever:  false,
					},
				},
			},
			"hourly": {
				Schedule: "0 * * * *", // Every hour
				Local: LocalBackupConfig{
					Enabled: false,
					Retention: RetentionRule{
						Duration: "24h",
						Forever:  false,
					},
				},
				S3: S3BackupConfig{
					Enabled: false,
				},
			},
			"daily": {
				Schedule: "0 0 * * *", // Every day at midnight
				Local: LocalBackupConfig{
					Enabled: true,
					Retention: RetentionRule{
						Duration: "7d",
						Forever:  false,
					},
				},
				S3: S3BackupConfig{
					Enabled: true,
					Retention: RetentionRule{
						Duration: "30d",
						Forever:  false,
					},
				},
			},
		}
	}
}

// Helper functions for environment variables

func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	if defaultValue != "" && os.Getenv("DEBUG") == "true" {
		log.Printf("Environment variable %s not set. Using default: %s", key, defaultValue)
	}
	return defaultValue
}

func parseEnvBool(key string, defaultValue bool) bool {
	value, exists := os.LookupEnv(key)
	if !exists {
		if os.Getenv("DEBUG") == "true" {
			log.Printf("Environment variable %s not set. Using default: %t", key, defaultValue)
		}
		return defaultValue
	}
	value = strings.ToLower(value)

	// Handle additional truthy and falsy values
	switch value {
	case "1", "t", "true", "yes", "on", "enabled":
		return true
	case "0", "f", "false", "no", "off", "disabled":
		return false
	default:
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			log.Printf("Error parsing %s as bool: %v. Using default value: %t", key, err, defaultValue)
			return defaultValue
		}
		return boolValue
	}
}

// DisplayConfiguration outputs the current configuration in a readable format
// while masking sensitive information
func DisplayConfiguration() {
	log.Println("========== GoSQLGuard Configuration ==========")

	// General settings
	log.Printf("Debug Mode: %t", CFG.Debug)
	log.Printf("Config File: %s", CFG.ConfigFile)

	// MySQL options removed - now hardcoded in provider
	log.Println("\n----- MySQL Dump Options -----")
	log.Println("Using hardcoded options in provider - see provider.go for details")

	// Database Servers
	log.Println("\n----- Database Servers Configuration -----")
	if len(CFG.DatabaseServers) > 0 {
		for _, server := range CFG.DatabaseServers {
			log.Printf("\nServer Name: %s", server.Name)
			log.Printf("Server Type: %s", server.Type)
			log.Printf("Host: %s", server.Host)
			log.Printf("Port: %s", server.Port)
			log.Printf("Username: %s", server.Username)
			log.Printf("Password: %s", maskSensitiveInfo(server.Password))

			log.Println("Include Databases:")
			if len(server.IncludeDatabases) > 0 {
				for _, db := range server.IncludeDatabases {
					log.Printf("  - %s", db)
				}
			} else {
				log.Println("  [Empty - will use all available databases or apply exclude filters]")
			}

			log.Println("Exclude Databases:")
			if len(server.ExcludeDatabases) > 0 {
				for _, db := range server.ExcludeDatabases {
					log.Printf("  - %s", db)
				}
			} else {
				log.Println("  [Empty - no databases excluded]")
			}
		}
	} else {
		log.Println("No database servers configured.")
	}

	// Legacy MySQL settings (if configured)
	if CFG.MySQL.Host != "" {
		log.Println("\n----- Legacy MySQL Configuration (Deprecated) -----")
		log.Printf("Host: %s", CFG.MySQL.Host)
		log.Printf("Port: %s", CFG.MySQL.Port)
		log.Printf("Username: %s", CFG.MySQL.Username)
		log.Printf("Password: %s", maskSensitiveInfo(CFG.MySQL.Password))

		log.Println("Include Databases:")
		if len(CFG.MySQL.IncludeDatabases) > 0 {
			for _, db := range CFG.MySQL.IncludeDatabases {
				log.Printf("  - %s", db)
			}
		} else {
			log.Println("  [Empty - will use all available databases or apply exclude filters]")
		}

		log.Println("Exclude Databases:")
		if len(CFG.MySQL.ExcludeDatabases) > 0 {
			for _, db := range CFG.MySQL.ExcludeDatabases {
				log.Printf("  - %s", db)
			}
		} else {
			log.Println("  [Empty - no databases excluded]")
		}
	}

	// Legacy PostgreSQL settings (if configured)
	if len(CFG.PostgreSQL.Databases) > 0 {
		log.Println("\n----- Legacy PostgreSQL Configuration (Deprecated) -----")
		log.Printf("Host: %s", CFG.PostgreSQL.Host)
		log.Printf("Port: %s", CFG.PostgreSQL.Port)
		log.Printf("Username: %s", CFG.PostgreSQL.Username)
		log.Printf("Password: %s", maskSensitiveInfo(CFG.PostgreSQL.Password))

		log.Println("Databases:")
		for _, db := range CFG.PostgreSQL.Databases {
			log.Printf("  - %s", db)
		}
	}

	// Local backup settings
	log.Println("\n----- Local Backup Configuration -----")
	log.Printf("Enabled: %t", CFG.Local.Enabled)
	log.Printf("Backup Directory: %s", CFG.Local.BackupDirectory)
	log.Printf("Organization Strategy: %s", CFG.Local.OrganizationStrategy)

	// S3 settings
	log.Println("\n----- S3 Backup Configuration -----")
	log.Printf("Enabled: %t", CFG.S3.Enabled)
	if CFG.S3.Enabled {
		log.Printf("Bucket: %s", CFG.S3.Bucket)
		log.Printf("Region: %s", CFG.S3.Region)
		log.Printf("Endpoint: %s", CFG.S3.Endpoint)
		log.Printf("Access Key: %s", maskSensitiveInfo(CFG.S3.AccessKey))
		log.Printf("Secret Key: %s", maskSensitiveInfo(CFG.S3.SecretKey))
		log.Printf("Prefix: %s", CFG.S3.Prefix)
		log.Printf("Organization Strategy: %s", CFG.S3.OrganizationStrategy)
		log.Printf("Use SSL: %t", CFG.S3.UseSSL)
		log.Printf("Custom CA Path: %s", CFG.S3.CustomCAPath)
		log.Printf("Skip Cert Validation: %t", CFG.S3.SkipCertValidation)
	}

	// Metrics settings
	log.Println("\n----- Metrics Configuration -----")
	log.Printf("Port: %s", CFG.Metrics.Port)

	// Backup types
	log.Println("\n----- Backup Types Configuration -----")
	for typeName, typeConfig := range CFG.BackupTypes {
		log.Printf("\nBackup Type: %s", typeName)
		log.Printf("  Schedule: %s", typeConfig.Schedule)

		log.Println("  Local Storage:")
		log.Printf("    Enabled: %t", typeConfig.Local.Enabled)
		if typeConfig.Local.Enabled {
			log.Printf("    Retention Duration: %s", typeConfig.Local.Retention.Duration)
			log.Printf("    Keep Forever: %t", typeConfig.Local.Retention.Forever)
		}

		log.Println("  S3 Storage:")
		log.Printf("    Enabled: %t", typeConfig.S3.Enabled)
		if typeConfig.S3.Enabled {
			log.Printf("    Retention Duration: %s", typeConfig.S3.Retention.Duration)
			log.Printf("    Keep Forever: %t", typeConfig.S3.Retention.Forever)
		}
	}

	log.Println("============================================")
	
	// Metadata DB settings if enabled
	if CFG.MetadataDB.Enabled {
		log.Println("\n----- Metadata Database Configuration -----")
		log.Printf("Host: %s", CFG.MetadataDB.Host)
		log.Printf("Port: %d", CFG.MetadataDB.Port)
		log.Printf("Username: %s", CFG.MetadataDB.Username)
		log.Printf("Password: %s", maskSensitiveInfo(CFG.MetadataDB.Password))
		log.Printf("Database: %s", CFG.MetadataDB.Database)
		log.Printf("Max Open Connections: %d", CFG.MetadataDB.MaxOpenConns)
		log.Printf("Max Idle Connections: %d", CFG.MetadataDB.MaxIdleConns)
		log.Printf("Connection Max Lifetime: %s", CFG.MetadataDB.ConnMaxLifetime)
		log.Printf("Auto Migrate: %t", CFG.MetadataDB.AutoMigrate)
	}
}

// maskSensitiveInfo masks sensitive information for logging
func maskSensitiveInfo(info string) string {
	if info == "" {
		return "[not set]"
	}

	if len(info) <= 4 {
		return "****"
	}

	// Show first and last character, mask the rest
	return info[:2] + "****" + info[len(info)-2:]
}

// ValidateConfig validates the configuration
func ValidateConfig() error {
	// Check if database servers are configured
	hasServers := len(CFG.DatabaseServers) > 0

	// Check if either MySQL or PostgreSQL is configured (legacy method)
	mysqlConfigured := CFG.MySQL.Host != "" && CFG.MySQL.Username != ""
	postgresConfigured := len(CFG.PostgreSQL.Databases) > 0

	if !hasServers && !mysqlConfigured && !postgresConfigured {
		return fmt.Errorf("at least one database system (MySQL or PostgreSQL) must be configured")
	}

	// Validate MySQL configuration
	if mysqlConfigured {
		if CFG.MySQL.Host == "" {
			return fmt.Errorf("MySQL host is required")
		}

		if CFG.MySQL.Username == "" {
			return fmt.Errorf("MySQL username is required")
		}
	}

	// Validate PostgreSQL configuration if databases are specified
	if postgresConfigured {
		if CFG.PostgreSQL.Host == "" {
			return fmt.Errorf("PostgreSQL host is required when PostgreSQL databases are configured")
		}

		if CFG.PostgreSQL.Username == "" {
			return fmt.Errorf("PostgreSQL username is required when PostgreSQL databases are configured")
		}
	}

	// Validate storage configuration
	if !CFG.Local.Enabled && !CFG.S3.Enabled {
		return fmt.Errorf("at least one storage destination (local or S3) must be enabled")
	}

	if CFG.Local.Enabled && CFG.Local.BackupDirectory == "" {
		return fmt.Errorf("local backup directory must be specified when local backups are enabled")
	}

	if CFG.S3.Enabled {
		if CFG.S3.Bucket == "" {
			return fmt.Errorf("S3 bucket must be specified when S3 backups are enabled")
		}
		if CFG.S3.AccessKey == "" || CFG.S3.SecretKey == "" {
			return fmt.Errorf("S3 access key and secret key must be specified when S3 backups are enabled")
		}

		// Validate custom CA path if provided
		if CFG.S3.CustomCAPath != "" {
			if _, err := os.Stat(CFG.S3.CustomCAPath); err != nil {
				return fmt.Errorf("custom CA path %s is not accessible: %w", CFG.S3.CustomCAPath, err)
			}
		}

		// Validate that both custom CA and skip validation are not set
		if CFG.S3.CustomCAPath != "" && CFG.S3.SkipCertValidation {
			log.Printf("Warning: Both custom CA path and skip certificate validation are set. Custom CA will be ignored.")
		}
	}

	// Validate metadata database configuration if enabled
	if CFG.MetadataDB.Enabled {
		if CFG.MetadataDB.Host == "" {
			return fmt.Errorf("metadata database host is required when enabled")
		}
		if CFG.MetadataDB.Username == "" {
			return fmt.Errorf("metadata database username is required when enabled")
		}
		if CFG.MetadataDB.Database == "" {
			return fmt.Errorf("metadata database name is required when enabled")
		}
		
		// Validate connection max lifetime is a valid duration
		if CFG.MetadataDB.ConnMaxLifetime != "" {
			if _, err := time.ParseDuration(CFG.MetadataDB.ConnMaxLifetime); err != nil {
				return fmt.Errorf("invalid metadata database connection max lifetime: %v", err)
			}
		}
	}

	// Validate backup types and schedules
	if len(CFG.BackupTypes) == 0 {
		return fmt.Errorf("at least one backup type must be configured")
	}

	for name, backupType := range CFG.BackupTypes {
		if backupType.Schedule == "" && name != "manual" {
			return fmt.Errorf("backup type %s requires a schedule", name)
		}

		// Validate local backup settings
		if backupType.Local.Enabled && !CFG.Local.Enabled {
			return fmt.Errorf("local backup is enabled for type %s but global local backup is disabled", name)
		}

		// Validate S3 backup settings
		if backupType.S3.Enabled && !CFG.S3.Enabled {
			return fmt.Errorf("S3 backup is enabled for type %s but global S3 backup is disabled", name)
		}

		// Validate retention durations
		if !backupType.Local.Retention.Forever {
			if _, err := time.ParseDuration(backupType.Local.Retention.Duration); err != nil {
				return fmt.Errorf("invalid local retention duration for backup type %s: %v", name, err)
			}
		}

		if !backupType.S3.Retention.Forever {
			if _, err := time.ParseDuration(backupType.S3.Retention.Duration); err != nil {
				return fmt.Errorf("invalid S3 retention duration for backup type %s: %v", name, err)
			}
		}
	}

	return nil
}
