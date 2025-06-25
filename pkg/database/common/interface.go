// Package common provides shared types and interfaces for database operations
package common

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Provider represents a database provider interface
type Provider interface {
	// Name returns the provider name (e.g., "mysql", "postgresql")
	Name() string

	// Connect establishes a connection to the database server
	Connect(ctx context.Context) error

	// Close closes the database connection
	Close() error

	// ListDatabases returns a list of available databases
	ListDatabases(ctx context.Context) ([]string, error)

	// Backup performs a database backup and writes it to the provided writer
	// The database parameter specifies which database to backup
	// Optional parameters can be provided through the options parameter
	Backup(ctx context.Context, database string, output io.Writer, options BackupOptions) error

	// BackupCommand returns the command that would be used for backup
	// This is useful for logging and debugging purposes
	BackupCommand(database string, options BackupOptions) string

	// Validate ensures the provider configuration is valid
	Validate() error

	// GetDatabases returns the configured databases for this provider
	GetDatabases() []string
}

// BackupOptions contains options for the backup operation
type BackupOptions struct {
	// Compression indicates whether the backup should be compressed
	Compression bool

	// IncludeSchema indicates whether to include schema creation statements
	IncludeSchema bool

	// SchemaOnly indicates whether to include only schema without data
	SchemaOnly bool

	// ExcludeTables is a list of tables to exclude from the backup
	ExcludeTables []string

	// IncludeTables is a list of tables to include in the backup (empty means all)
	IncludeTables []string

	// Schemas is a list of schemas to include (for PostgreSQL, empty means all public)
	Schemas []string

	// TransactionMode indicates whether to use transaction-consistent backup
	TransactionMode bool

	// Timestamp is a timestamp to include in the backup filename
	Timestamp time.Time
}

// ProviderFactory creates a database provider from configuration
type ProviderFactory interface {
	// Create returns a new Provider instance
	Create() (Provider, error)
}

// providerFactories stores the registered provider factories
var providerFactories = make(map[string]ProviderFactory)

// RegisterProvider registers a provider factory with the given name
func RegisterProvider(name string, factory ProviderFactory) {
	providerFactories[name] = factory
}

// GetProvider returns a provider for the given name
func GetProvider(name string) (ProviderFactory, bool) {
	factory, exists := providerFactories[name]
	return factory, exists
}

// GetProviderForDatabase returns the appropriate provider for a given database
func GetProviderForDatabase(providers map[string]Provider, database string) (Provider, error) {
	// Check each provider's database list
	for _, provider := range providers {
		for _, db := range provider.GetDatabases() {
			if db == database {
				return provider, nil
			}
		}
	}
	return nil, fmt.Errorf("no provider found for database: %s", database)
}
