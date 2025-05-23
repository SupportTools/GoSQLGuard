package mysql

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"
)

type Provider struct {
	db *sql.DB
}

func NewProvider(db *sql.DB) *Provider {
	return &Provider{db: db}
}

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
