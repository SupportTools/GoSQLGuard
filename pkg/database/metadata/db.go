package metadata

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/supporttools/GoSQLGuard/pkg/config"
)

// DB is the global database instance
var DB *gorm.DB

// Initialize sets up the database connection and runs migrations if enabled
func Initialize() error {
	if !config.CFG.MetadataDB.Enabled {
		log.Println("Metadata database is not enabled, skipping initialization")
		return nil
	}

	// Connect to the database
	db, err := Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to metadata database: %w", err)
	}
	DB = db

	// Run auto-migrations if enabled
	if config.CFG.MetadataDB.AutoMigrate {
		log.Println("Running database migrations for metadata tables")
		if err := RunMigrations(db); err != nil {
			return fmt.Errorf("failed to run database migrations: %w", err)
		}
	}

	return nil
}

// Connect establishes a connection to the database
func Connect() (*gorm.DB, error) {
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

// RunMigrations runs all necessary database migrations
func RunMigrations(db *gorm.DB) error {
	// Create the tables if they don't exist
	err := db.AutoMigrate(
		&Backup{},
		&LocalPath{},
		&S3Key{},
		&Stats{},
		&ServerConfig{},
		&ServerDatabaseFilter{},
		&ServerMySQLOption{},
		&BackupSchedule{},
		&ScheduleRetentionPolicy{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate tables: %w", err)
	}

	// Initialize stats record if it doesn't exist
	var count int64
	db.Model(&Stats{}).Count(&count)
	if count == 0 {
		log.Println("Initializing metadata stats record")
		stats := Stats{
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

// Close closes the database connection
func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	return sqlDB.Close()
}

// GetDB returns the global database instance
func GetDB() *gorm.DB {
	return DB
}
