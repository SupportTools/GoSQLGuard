package mysql

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"
)

// Provider handles MySQL database operations
type Provider struct {
	db *sql.DB
}

// NewProvider creates a new MySQL provider instance
func NewProvider(db *sql.DB) *Provider {
	return &Provider{db: db}
}

// GetDatabases retrieves a list of all databases from the MySQL server
func (p *Provider) GetDatabases(ctx context.Context) ([]string, error) {
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
