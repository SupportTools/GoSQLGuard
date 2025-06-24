// Package mysql provides MySQL database provider implementation
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/database/common"
)

// Provider implements the database.Provider interface for MySQL
type Provider struct {
	Host             string
	Port             int
	User             string
	Password         string
	IncludeDatabases []string
	ExcludeDatabases []string

	db *sql.DB
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "mysql"
}

// Connect establishes a connection to the database server
func (p *Provider) Connect(ctx context.Context) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/",
		p.User, p.Password, p.Host, p.Port)

	var err error
	p.db, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("database backup failed: %v", err)
	}

	// Test the connection
	err = p.db.PingContext(ctx)
	if err != nil {
		p.db.Close()
		return fmt.Errorf("failed to ping MySQL server: %w", err)
	}

	return nil
}

// Close closes the database connection
func (p *Provider) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// ListDatabases returns a list of available databases
func (p *Provider) ListDatabases(ctx context.Context) ([]string, error) {
	if p.db == nil {
		return nil, errors.New("not connected to MySQL server")
	}

	rows, err := p.db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return nil, fmt.Errorf("failed to scan database name: %w", err)
		}

		// Skip system databases
		if dbName == "information_schema" || dbName == "mysql" ||
			dbName == "performance_schema" || dbName == "sys" {
			continue
		}

		databases = append(databases, dbName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating database rows: %w", err)
	}

	return databases, nil
}

// Backup performs a database backup and writes it to the provided writer
func (p *Provider) Backup(ctx context.Context, dbName string, output io.Writer, options common.BackupOptions) error {
	// Get MySQL dump options (hardcoded now, not from config)
	mysqlDumpOptions := config.MySQLDumpOptionsConfig{}

	// For debugging
	if config.CFG.Debug {
		if len(mysqlDumpOptions.CustomOptions) > 0 {
			fmt.Fprintf(os.Stderr, "DEBUG: Using custom options for backup: %v\n", mysqlDumpOptions.CustomOptions)
		}
	}

	cmd := p.createBackupCommand(dbName, options, mysqlDumpOptions)
	cmd.Stdout = output
	cmd.Stderr = os.Stderr

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start mysqldump: %w", err)
	}

	// Create a channel to signal command completion
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Wait for either context cancellation or command completion
	select {
	case <-ctx.Done():
		// Context was canceled, try to kill the process
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return ctx.Err()
	case err := <-done:
		// Command completed
		if err != nil {
			return fmt.Errorf("mysqldump failed: %w", err)
		}
		return nil
	}
}

// isOptionsEmpty checks if a MySQLDumpOptionsConfig is empty (zero value)
func isOptionsEmpty(opts config.MySQLDumpOptionsConfig) bool {
	// If CustomOptions has values, it's not empty
	if len(opts.CustomOptions) > 0 {
		return false
	}

	// If any boolean field is true, it's not empty
	if opts.SingleTransaction || opts.Quick || opts.SkipLockTables ||
		opts.SkipAddLocks || opts.SkipComments || opts.ExtendedInsert ||
		opts.SkipExtendedInsert || opts.Compress {
		return false
	}

	// All fields are default values, consider it empty
	return true
}

// BackupCommand returns the command that would be used for backup
func (p *Provider) BackupCommand(dbName string, options common.BackupOptions) string {
	// Try to find server-specific options if available
	mysqlDumpOptions := config.CFG.MySQLDumpOptions

	// Find server-specific options if possible
	for _, server := range config.CFG.DatabaseServers {
		if server.Host == p.Host && server.Type == "mysql" {
			// Found matching server, use its options
			if !isOptionsEmpty(server.MySQLDumpOptions) {
				mysqlDumpOptions = server.MySQLDumpOptions
				break
			}
		}
	}

	// Create and return the command string
	cmd := p.createBackupCommand(dbName, options, mysqlDumpOptions)
	return cmd.String()
}

// createBackupCommand creates the exec.Cmd for mysqldump with hardcoded options
func (p *Provider) createBackupCommand(dbName string, options common.BackupOptions, _ config.MySQLDumpOptionsConfig) *exec.Cmd {
	// Basic required args that we still need to set dynamically
	args := []string{
		"-h", p.Host,
		"-P", fmt.Sprintf("%d", p.Port),
		"-u", p.User,
	}

	// Add password if provided
	if p.Password != "" {
		args = append(args, fmt.Sprintf("-p%s", p.Password))
	}

	// Hardcoded options as per requirements
	args = append(args,
		"--single-transaction",
		"--quick",
		"--triggers",
		"--routines",
		"--events",
		"--set-gtid-purged=OFF",
	)

	// Add schema-only option if requested
	if options.SchemaOnly {
		args = append(args, "--no-data")
	}

	// Exclude specific tables if requested
	for _, table := range options.ExcludeTables {
		args = append(args, fmt.Sprintf("--ignore-table=%s.%s", dbName, table))
	}

	// Add database name and tables if specified
	args = append(args, dbName)
	if len(options.IncludeTables) > 0 {
		args = append(args, options.IncludeTables...)
	}

	return exec.Command("mysqldump", args...)
}

// Validate ensures the provider configuration is valid
func (p *Provider) Validate() error {
	if p.Host == "" {
		return errors.New("MySQL host is required")
	}

	if p.Port <= 0 || p.Port > 65535 {
		return fmt.Errorf("invalid MySQL port: %d", p.Port)
	}

	if p.User == "" {
		return errors.New("MySQL user is required")
	}

	// No need to validate databases list - we'll use all databases
	// if neither include nor exclude lists are specified

	return nil
}

// GetDatabases returns a list of databases to backup
// It applies include/exclude filters based on configuration
func (p *Provider) GetDatabases() []string {
	// We'll need to connect and get the actual list of databases
	ctx := context.Background()
	if p.db == nil {
		if err := p.Connect(ctx); err != nil {
			// If we can't connect, return an empty list
			return []string{}
		}
		defer p.Close()
	}

	// Get all available databases
	allDBs, err := p.ListDatabases(ctx)
	if err != nil {
		// If we can't list databases, return an empty list
		return []string{}
	}

	// Apply filtering logic

	// If include list is not empty, use only those databases
	if len(p.IncludeDatabases) > 0 {
		var filtered []string
		for _, db := range allDBs {
			// Check if this database is in the include list
			for _, includeDB := range p.IncludeDatabases {
				if db == includeDB {
					filtered = append(filtered, db)
					break
				}
			}
		}
		return filtered
	}

	// If only exclude list is specified, use all databases except those
	if len(p.ExcludeDatabases) > 0 {
		var filtered []string
		for _, db := range allDBs {
			excluded := false
			// Check if this database is in the exclude list
			for _, excludeDB := range p.ExcludeDatabases {
				if db == excludeDB {
					excluded = true
					break
				}
			}
			if !excluded {
				filtered = append(filtered, db)
			}
		}
		return filtered
	}

	// Both lists are empty, return all databases
	return allDBs
}

// Factory creates MySQL database providers
type Factory struct {
	Host             string
	Port             int
	User             string
	Password         string
	IncludeDatabases []string
	ExcludeDatabases []string
}

// Create returns a new Provider instance
func (f *Factory) Create() (common.Provider, error) {
	provider := &Provider{
		Host:             f.Host,
		Port:             f.Port,
		User:             f.User,
		Password:         f.Password,
		IncludeDatabases: f.IncludeDatabases,
		ExcludeDatabases: f.ExcludeDatabases,
	}

	if err := provider.Validate(); err != nil {
		return nil, err
	}

	return provider, nil
}

func init() {
	// Register this provider with the database package
	common.RegisterProvider("mysql", &Factory{})
}
