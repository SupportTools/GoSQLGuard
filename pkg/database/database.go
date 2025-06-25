// Package database provides a unified interface for working with different database systems
package database

import (
	"fmt"
	"log"
	"strconv"

	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/database/common"
	"github.com/supporttools/GoSQLGuard/pkg/database/providers/mysql"
	"github.com/supporttools/GoSQLGuard/pkg/database/providers/postgresql"
)

// Provider is the interface all database providers must implement
type Provider = common.Provider

// BackupOptions contains options for the backup operation
type BackupOptions = common.BackupOptions

// Providers stores initialized database providers
var providers map[string]Provider

// Initialize sets up all configured database providers
func Initialize() error {
	providers = make(map[string]Provider)

	// Initialize MySQL provider if enabled
	if config.CFG.MySQL.Host != "" && config.CFG.MySQL.Username != "" {
		// Create MySQL provider factory
		factory, exists := common.GetProvider("mysql")
		if !exists {
			log.Println("Warning: MySQL provider is not registered")
		} else {
			// Convert port from string to int
			portInt, _ := strconv.Atoi(config.CFG.MySQL.Port)
			if portInt == 0 {
				portInt = 3306 // Default MySQL port
			}

			// Create and configure MySQL provider
			mysqlFactory, ok := factory.(common.ProviderFactory)
			if !ok {
				return fmt.Errorf("invalid MySQL provider factory type")
			}

			// Update factory fields
			// This requires type asserting to the specific factory type
			mysqlFactoryImpl, ok := mysqlFactory.(*mysql.Factory)
			if ok {
				mysqlFactoryImpl.Host = config.CFG.MySQL.Host
				mysqlFactoryImpl.Port = portInt
				mysqlFactoryImpl.User = config.CFG.MySQL.Username
				mysqlFactoryImpl.Password = config.CFG.MySQL.Password
				mysqlFactoryImpl.IncludeDatabases = config.CFG.MySQL.IncludeDatabases
				mysqlFactoryImpl.ExcludeDatabases = config.CFG.MySQL.ExcludeDatabases
			}

			// Create provider instance
			provider, err := mysqlFactory.Create()
			if err != nil {
				return fmt.Errorf("failed to create MySQL provider: %w", err)
			}

			providers["mysql"] = provider
			log.Println("MySQL provider initialized with include/exclude database filtering")
		}
	}

	// Initialize PostgreSQL provider if enabled
	if len(config.CFG.PostgreSQL.Databases) > 0 {
		// Create PostgreSQL provider factory
		factory, exists := common.GetProvider("postgresql")
		if !exists {
			log.Println("Warning: PostgreSQL provider is not registered")
		} else {
			// Convert port from string to int
			portInt, _ := strconv.Atoi(config.CFG.PostgreSQL.Port)
			if portInt == 0 {
				portInt = 5432 // Default PostgreSQL port
			}

			// Create and configure PostgreSQL provider
			postgresFactory, ok := factory.(common.ProviderFactory)
			if !ok {
				return fmt.Errorf("invalid PostgreSQL provider factory type")
			}

			// Update factory fields
			// This requires type asserting to the specific factory type
			postgresFactoryImpl, ok := postgresFactory.(*postgresql.Factory)
			if ok {
				postgresFactoryImpl.Host = config.CFG.PostgreSQL.Host
				postgresFactoryImpl.Port = portInt
				postgresFactoryImpl.User = config.CFG.PostgreSQL.Username
				postgresFactoryImpl.Password = config.CFG.PostgreSQL.Password
				postgresFactoryImpl.Databases = config.CFG.PostgreSQL.Databases
			}

			// Create provider instance
			provider, err := postgresFactory.Create()
			if err != nil {
				return fmt.Errorf("failed to create PostgreSQL provider: %w", err)
			}

			providers["postgresql"] = provider
			log.Printf("PostgreSQL provider initialized with %d databases", len(config.CFG.PostgreSQL.Databases))
		}
	}

	// Validate that we have at least one provider
	if len(providers) == 0 {
		return fmt.Errorf("no database providers were initialized")
	}

	return nil
}

// GetProvider returns a provider by name
func GetProvider(name string) (Provider, bool) {
	provider, exists := providers[name]
	return provider, exists
}

// GetDatabaseProvider returns the appropriate provider for a given database
// It delegates to the common package implementation
func GetDatabaseProvider(database string) (Provider, error) {
	return common.GetProviderForDatabase(providers, database)
}

// GetAllProviders returns all initialized providers
func GetAllProviders() map[string]Provider {
	return providers
}

// ShutdownAll closes all provider connections
func ShutdownAll() {
	for name, provider := range providers {
		if err := provider.Close(); err != nil {
			log.Printf("Error closing %s provider: %v", name, err)
		} else {
			log.Printf("Closed %s provider", name)
		}
	}
}
