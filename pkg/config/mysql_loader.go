package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLConfigLoader loads configuration from MySQL database
type MySQLConfigLoader struct {
	DB *sql.DB // Exported for API access
}

// MySQLConfigOptions holds connection parameters for the config database
type MySQLConfigOptions struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string
}

// NewMySQLConfigLoader creates a new MySQL configuration loader
func NewMySQLConfigLoader(opts MySQLConfigOptions) (*MySQLConfigLoader, error) {
	// Build connection string
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true",
		opts.Username, opts.Password, opts.Host, opts.Port, opts.Database)

	// Open database connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping MySQL: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &MySQLConfigLoader{DB: db}, nil
}

// LoadConfiguration loads the complete configuration from MySQL
func (m *MySQLConfigLoader) LoadConfiguration() (*AppConfig, error) {
	config := &AppConfig{
		BackupTypes: make(map[string]BackupTypeConfig),
		MetadataDB: MetadataDBConfig{
			// Set sensible defaults
			MaxOpenConns:    25,
			MaxIdleConns:    25,
			ConnMaxLifetime: "5m",
			AutoMigrate:     true,
		},
	}

	// Load global configuration
	if err := m.loadGlobalConfig(config); err != nil {
		return nil, fmt.Errorf("failed to load global config: %w", err)
	}
	
	// Debug: Log metadata configuration
	log.Printf("DEBUG: MetadataDB config after loading - Enabled: %v, Host: %s, Port: %d, Database: %s, Username: %s",
		config.MetadataDB.Enabled, config.MetadataDB.Host, config.MetadataDB.Port, 
		config.MetadataDB.Database, config.MetadataDB.Username)

	// Load database servers
	servers, err := m.loadDatabaseServers()
	if err != nil {
		return nil, fmt.Errorf("failed to load database servers: %w", err)
	}

	// Set legacy configuration for backward compatibility
	if len(servers) > 0 {
		for _, server := range servers {
			if server.Type == "mysql" && config.MySQL.Host == "" {
				config.MySQL = MySQLConfig{
					Host:             server.Host,
					Port:             server.Port,
					Username:         server.Username,
					Password:         server.Password,
					IncludeDatabases: server.IncludeDatabases,
					ExcludeDatabases: server.ExcludeDatabases,
				}
			} else if server.Type == "postgresql" && config.PostgreSQL.Host == "" {
				config.PostgreSQL = PostgreSQLConfig{
					Host:      server.Host,
					Port:      server.Port,
					Username:  server.Username,
					Password:  server.Password,
					Databases: server.IncludeDatabases,
				}
			}
		}
	}
	config.DatabaseServers = servers

	// Load storage configurations
	if err := m.loadStorageConfigs(config); err != nil {
		return nil, fmt.Errorf("failed to load storage configs: %w", err)
	}

	// Load backup schedules and retention policies
	if err := m.loadBackupSchedules(config); err != nil {
		return nil, fmt.Errorf("failed to load backup schedules: %w", err)
	}

	// Load MySQL dump options
	if err := m.loadMySQLDumpOptions(config); err != nil {
		return nil, fmt.Errorf("failed to load MySQL dump options: %w", err)
	}

	return config, nil
}

// loadGlobalConfig loads global configuration settings
func (m *MySQLConfigLoader) loadGlobalConfig(config *AppConfig) error {
	query := "SELECT `key`, `value`, `type` FROM global_config"
	rows, err := m.DB.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var key, value, valueType string
		if err := rows.Scan(&key, &value, &valueType); err != nil {
			return err
		}

		switch key {
		case "debug":
			config.Debug = value == "true"
		case "metrics_port":
			config.Metrics.Port = value
		case "metrics_enabled":
			// Metrics are always enabled, just skip this field
		case "metadata_database_enabled":
			config.MetadataDB.Enabled = value == "true"
		case "metadata_database_config":
			// Parse JSON config for backward compatibility
			var metaConfig map[string]interface{}
			if err := json.Unmarshal([]byte(value), &metaConfig); err == nil {
				if host, ok := metaConfig["host"].(string); ok {
					config.MetadataDB.Host = host
				}
				if port, ok := metaConfig["port"].(string); ok {
					if p, err := strconv.Atoi(port); err == nil {
						config.MetadataDB.Port = p
					}
				}
				if database, ok := metaConfig["database"].(string); ok {
					config.MetadataDB.Database = database
				}
				if user, ok := metaConfig["user"].(string); ok {
					config.MetadataDB.Username = user
				}
				if password, ok := metaConfig["password"].(string); ok {
					config.MetadataDB.Password = password
				}
			}
		case "metadata_database_host":
			config.MetadataDB.Host = value
		case "metadata_database_port":
			if port, err := strconv.Atoi(value); err == nil {
				config.MetadataDB.Port = port
			}
		case "metadata_database_username":
			config.MetadataDB.Username = value
		case "metadata_database_password":
			config.MetadataDB.Password = value
		case "metadata_database_database":
			config.MetadataDB.Database = value
		case "metadata_database_max_open_conns":
			if conns, err := strconv.Atoi(value); err == nil {
				config.MetadataDB.MaxOpenConns = conns
			}
		case "metadata_database_max_idle_conns":
			if conns, err := strconv.Atoi(value); err == nil {
				config.MetadataDB.MaxIdleConns = conns
			}
		case "metadata_database_conn_max_lifetime":
			config.MetadataDB.ConnMaxLifetime = value
		case "metadata_database_auto_migrate":
			config.MetadataDB.AutoMigrate = value == "true"
		}
	}

	return rows.Err()
}

// loadDatabaseServers loads database server configurations
func (m *MySQLConfigLoader) loadDatabaseServers() ([]DatabaseServerConfig, error) {
	query := `
		SELECT id, name, type, host, port, username, password, auth_plugin 
		FROM database_servers 
		WHERE enabled = TRUE
	`
	rows, err := m.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []DatabaseServerConfig
	for rows.Next() {
		var server DatabaseServerConfig
		var id int
		var authPlugin sql.NullString

		if err := rows.Scan(&id, &server.Name, &server.Type, &server.Host,
			&server.Port, &server.Username, &server.Password, &authPlugin); err != nil {
			return nil, err
		}

		if authPlugin.Valid {
			server.AuthPlugin = authPlugin.String
		}

		// Load database filters
		filters, err := m.loadDatabaseFilters(id)
		if err != nil {
			return nil, err
		}
		server.IncludeDatabases = filters["include"]
		server.ExcludeDatabases = filters["exclude"]

		servers = append(servers, server)
	}

	return servers, rows.Err()
}

// loadDatabaseFilters loads database inclusion/exclusion rules
func (m *MySQLConfigLoader) loadDatabaseFilters(serverID int) (map[string][]string, error) {
	query := `
		SELECT filter_type, database_name 
		FROM database_filters 
		WHERE server_id = ?
	`
	rows, err := m.DB.Query(query, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	filters := map[string][]string{
		"include": {},
		"exclude": {},
	}

	for rows.Next() {
		var filterType, dbName string
		if err := rows.Scan(&filterType, &dbName); err != nil {
			return nil, err
		}
		filters[filterType] = append(filters[filterType], dbName)
	}

	return filters, rows.Err()
}

// loadStorageConfigs loads storage configurations
func (m *MySQLConfigLoader) loadStorageConfigs(config *AppConfig) error {
	query := `SELECT name, type, config FROM storage_configs WHERE enabled = TRUE`
	rows, err := m.DB.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name, storageType, configJSON string
		if err := rows.Scan(&name, &storageType, &configJSON); err != nil {
			return err
		}

		switch storageType {
		case "local":
			var localCfg map[string]interface{}
			if err := json.Unmarshal([]byte(configJSON), &localCfg); err != nil {
				return err
			}
			config.Local.Enabled = true
			if dir, ok := localCfg["backupDirectory"].(string); ok {
				config.Local.BackupDirectory = dir
			}
			if strategy, ok := localCfg["organizationStrategy"].(string); ok {
				config.Local.OrganizationStrategy = strategy
			}

		case "s3":
			var s3Cfg map[string]interface{}
			if err := json.Unmarshal([]byte(configJSON), &s3Cfg); err != nil {
				return err
			}
			config.S3.Enabled = true
			if bucket, ok := s3Cfg["bucket"].(string); ok {
				config.S3.Bucket = bucket
			}
			if region, ok := s3Cfg["region"].(string); ok {
				config.S3.Region = region
			}
			if endpoint, ok := s3Cfg["endpoint"].(string); ok {
				config.S3.Endpoint = endpoint
			}
			if accessKey, ok := s3Cfg["accessKey"].(string); ok {
				config.S3.AccessKey = accessKey
			}
			if secretKey, ok := s3Cfg["secretKey"].(string); ok {
				config.S3.SecretKey = secretKey
			}
			if prefix, ok := s3Cfg["prefix"].(string); ok {
				config.S3.Prefix = prefix
			}
			if useSSL, ok := s3Cfg["useSSL"].(bool); ok {
				config.S3.UseSSL = useSSL
			}
			if strategy, ok := s3Cfg["organizationStrategy"].(string); ok {
				config.S3.OrganizationStrategy = strategy
			}
		}
	}

	return rows.Err()
}

// loadBackupSchedules loads backup schedules and retention policies
func (m *MySQLConfigLoader) loadBackupSchedules(config *AppConfig) error {
	query := `
		SELECT id, name, backup_type, cron_expression 
		FROM backup_schedules 
		WHERE enabled = TRUE
	`
	rows, err := m.DB.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name, backupType, cronExpr string
		if err := rows.Scan(&id, &name, &backupType, &cronExpr); err != nil {
			return err
		}

		backupConfig := BackupTypeConfig{
			Schedule: cronExpr,
		}

		// Load retention policies for this schedule
		if err := m.loadRetentionPolicies(id, &backupConfig); err != nil {
			return err
		}

		config.BackupTypes[backupType] = backupConfig
	}

	return rows.Err()
}

// loadRetentionPolicies loads retention policies for a backup schedule
func (m *MySQLConfigLoader) loadRetentionPolicies(scheduleID int, config *BackupTypeConfig) error {
	query := `
		SELECT sc.type, rp.retention_duration, rp.keep_forever
		FROM retention_policies rp
		JOIN storage_configs sc ON rp.storage_id = sc.id
		WHERE rp.schedule_id = ? AND rp.enabled = TRUE
	`
	rows, err := m.DB.Query(query, scheduleID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var storageType, duration string
		var keepForever bool
		if err := rows.Scan(&storageType, &duration, &keepForever); err != nil {
			return err
		}

		retention := RetentionRule{
			Duration: duration,
			Forever:  keepForever,
		}

		switch storageType {
		case "local":
			config.Local.Enabled = true
			config.Local.Retention = retention
		case "s3":
			config.S3.Enabled = true
			config.S3.Retention = retention
		}
	}

	return rows.Err()
}

// loadMySQLDumpOptions loads MySQL dump options
func (m *MySQLConfigLoader) loadMySQLDumpOptions(config *AppConfig) error {
	query := `
		SELECT option_name, option_value 
		FROM mysql_dump_options 
		WHERE server_id IS NULL AND enabled = TRUE
	`
	rows, err := m.DB.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var options []string
	for rows.Next() {
		var name, value sql.NullString
		if err := rows.Scan(&name, &value); err != nil {
			return err
		}

		if name.Valid {
			option := name.String
			if value.Valid && value.String != "" {
				option += "=" + value.String
			}
			options = append(options, option)
		}
	}

	config.MySQLDumpOptions.CustomOptions = options
	return rows.Err()
}

// WatchForChanges monitors the configuration database for changes
func (m *MySQLConfigLoader) WatchForChanges(interval time.Duration, onChange func(*AppConfig)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastVersion string
	for range ticker.C {
		var currentVersion string
		err := m.DB.QueryRow(`
			SELECT version FROM config_versions WHERE active = TRUE LIMIT 1
		`).Scan(&currentVersion)

		if err == nil && currentVersion != lastVersion {
			config, err := m.LoadConfiguration()
			if err != nil {
				log.Printf("Error reloading configuration: %v", err)
				continue
			}
			onChange(config)
			lastVersion = currentVersion
		}
	}
}

// Close closes the database connection
func (m *MySQLConfigLoader) Close() error {
	return m.DB.Close()
}

// LoadConfigFromMySQL loads configuration from MySQL if CONFIG_SOURCE=mysql
func LoadConfigFromMySQL() (*AppConfig, error) {
	opts := MySQLConfigOptions{
		Host:     os.Getenv("CONFIG_MYSQL_HOST"),
		Port:     os.Getenv("CONFIG_MYSQL_PORT"),
		Database: os.Getenv("CONFIG_MYSQL_DATABASE"),
		Username: os.Getenv("CONFIG_MYSQL_USER"),
		Password: os.Getenv("CONFIG_MYSQL_PASSWORD"),
	}

	// Set defaults
	if opts.Host == "" {
		opts.Host = "config-mysql"  // Default to sidecar container name
	}
	if opts.Port == "" {
		opts.Port = "3306"
	}
	if opts.Database == "" {
		opts.Database = "gosqlguard_config"
	}

	loader, err := NewMySQLConfigLoader(opts)
	if err != nil {
		return nil, err
	}
	defer loader.Close()

	return loader.LoadConfiguration()
}