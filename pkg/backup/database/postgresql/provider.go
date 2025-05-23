// Package postgresql provides PostgreSQL database provider implementation
package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/supporttools/GoSQLGuard/pkg/database/common"
)

// Provider implements the database.Provider interface for PostgreSQL
type Provider struct {
	Host     string
	Port     int
	User     string
	Password string
	Databases []string
	Schemas   []string
	
	db *sql.DB
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "postgresql"
}

// Connect establishes a connection to the database server
func (p *Provider) Connect(ctx context.Context) error {
	// Use the 'postgres' database to connect initially
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable", 
		p.Host, p.Port, p.User, p.Password)
	
	var err error
	p.db, err = sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}
	
	// Test the connection
	err = p.db.PingContext(ctx)
	if err != nil {
		p.db.Close()
		return fmt.Errorf("failed to ping PostgreSQL server: %w", err)
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
		return nil, errors.New("not connected to PostgreSQL server")
	}
	
	// Query to get all user databases
	query := `
		SELECT datname FROM pg_database 
		WHERE datistemplate = false 
		AND datname NOT IN ('postgres', 'template0', 'template1')
	`
	
	rows, err := p.db.QueryContext(ctx, query)
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
		
		databases = append(databases, dbName)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating database rows: %w", err)
	}
	
	return databases, nil
}

// Backup performs a database backup and writes it to the provided writer
func (p *Provider) Backup(ctx context.Context, dbName string, output io.Writer, options common.BackupOptions) error {
	cmd := p.createBackupCommand(dbName, options)
	cmd.Stdout = output
	cmd.Stderr = os.Stderr
	
	// Add environment variables for password authentication
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", p.Password))
	
	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start pg_dump: %w", err)
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
			return fmt.Errorf("pg_dump failed: %w", err)
		}
		return nil
	}
}

// BackupCommand returns the command that would be used for backup
func (p *Provider) BackupCommand(dbName string, options common.BackupOptions) string {
	cmd := p.createBackupCommand(dbName, options)
	return cmd.String()
}

// createBackupCommand creates the exec.Cmd for pg_dump
func (p *Provider) createBackupCommand(dbName string, options common.BackupOptions) *exec.Cmd {
	args := []string{
		"--host", p.Host,
		"--port", fmt.Sprintf("%d", p.Port),
		"--username", p.User,
		"--no-password", // Don't prompt for password; use PGPASSWORD env var
	}
	
	// Add schema-only option if requested
	if options.SchemaOnly {
		args = append(args, "--schema-only")
	}
	
	// Add specific schemas if provided
	if len(options.Schemas) > 0 {
		for _, schema := range options.Schemas {
			args = append(args, "--schema", schema)
		}
	} else if len(p.Schemas) > 0 {
		// Use configured schemas if none specified in options
		for _, schema := range p.Schemas {
			args = append(args, "--schema", schema)
		}
	}
	
	// Add specific tables if provided
	if len(options.IncludeTables) > 0 {
		for _, table := range options.IncludeTables {
			args = append(args, "--table", table)
		}
	}
	
	// Add exclude tables if provided
	if len(options.ExcludeTables) > 0 {
		for _, table := range options.ExcludeTables {
			args = append(args, "--exclude-table", table)
		}
	}
	
	// Add clean option to drop objects before recreating
	args = append(args, "--clean")
	
	// Add create option to include create database statement
	if options.IncludeSchema {
		args = append(args, "--create")
	}
	
	// Add format option (custom format is more flexible for restoration)
	args = append(args, "--format", "p") // Plain text format
	
	// Add database name
	args = append(args, dbName)
	
	return exec.Command("pg_dump", args...)
}

// Validate ensures the provider configuration is valid
func (p *Provider) Validate() error {
	if p.Host == "" {
		return errors.New("PostgreSQL host is required")
	}
	
	if p.Port <= 0 || p.Port > 65535 {
		return fmt.Errorf("invalid PostgreSQL port: %d", p.Port)
	}
	
	if p.User == "" {
		return errors.New("PostgreSQL user is required")
	}
	
	if len(p.Databases) == 0 {
		return errors.New("at least one database must be specified")
	}
	
	return nil
}

// parseSchemas parses a comma-separated list of schemas
func (p *Provider) parseSchemas(schemaStr string) []string {
	if schemaStr == "" {
		return []string{"public"} // Default to public schema
	}
	
	schemas := strings.Split(schemaStr, ",")
	var result []string
	
	for _, schema := range schemas {
		schema = strings.TrimSpace(schema)
		if schema != "" {
			result = append(result, schema)
		}
	}
	
	if len(result) == 0 {
		return []string{"public"} // Fallback to public schema
	}
	
	return result
}

// Factory creates PostgreSQL database providers
type Factory struct {
	Host     string
	Port     int
	User     string
	Password string
	Databases []string
	SchemaStr string
}

// Create returns a new Provider instance
func (f *Factory) Create() (common.Provider, error) {
	provider := &Provider{
		Host:      f.Host,
		Port:      f.Port,
		User:      f.User,
		Password:  f.Password,
		Databases: f.Databases,
	}
	
	// Parse schemas
	if provider.Schemas = provider.parseSchemas(f.SchemaStr); len(provider.Schemas) == 0 {
		provider.Schemas = []string{"public"}
	}
	
	if err := provider.Validate(); err != nil {
		return nil, err
	}
	
	return provider, nil
}

// GetDatabases returns a list of databases (implementation of common.Provider interface)
func (p *Provider) GetDatabases() []string {
	return p.Databases
}

func init() {
	// Register this provider with the database package
	common.RegisterProvider("postgresql", &Factory{})
}
