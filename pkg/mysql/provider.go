package mysql

import (
    "context"
    "database/sql"
    "io"
    "github.com/pkg/errors"
    "github.com/supporttools/GoSQLGuard/pkg/database/common"
)

type Provider struct {
    db *sql.DB
    databases []string
}

func NewProvider(db *sql.DB) *Provider {
    return &Provider{db: db}
}

// GetDatabases returns the configured databases (implementation of common.Provider interface)
func (p *Provider) GetDatabases() []string {
    return p.databases
}

// Name returns the provider name
func (p *Provider) Name() string {
    return "mysql"
}

// Connect establishes a connection to the database server
func (p *Provider) Connect(ctx context.Context) error {
    // Connection is handled by the caller
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
    rows, err := p.db.QueryContext(ctx, "SHOW DATABASES")
    if err != nil {
        return nil, errors.Wrap(err, "failed to fetch databases")
    }
    defer rows.Close()

    var databases []string
    for rows.Next() {
        var db string
        if err := rows.Scan(&db); err != nil {
            return nil, errors.Wrap(err, "failed to scan database name")
        }
        databases = append(databases, db)
    }

    return databases, nil
}

// Backup performs a database backup (placeholder implementation)
func (p *Provider) Backup(ctx context.Context, dbName string, output io.Writer, options common.BackupOptions) error {
    return errors.New("backup not implemented")
}

// BackupCommand returns the command that would be used for backup (placeholder)
func (p *Provider) BackupCommand(dbName string, options common.BackupOptions) string {
    return "mysqldump (placeholder)"
}

// Validate ensures the provider configuration is valid
func (p *Provider) Validate() error {
    return nil
}
