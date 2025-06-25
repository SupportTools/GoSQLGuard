// Package metadata provides optimized MySQL-backed metadata storage
package metadata

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
	"gorm.io/gorm"
)

// contextKey is a type for context keys
type contextKey string

const (
	// startTimeKey is the context key for query start time
	startTimeKey contextKey = "start_time"
)

// PaginatedResult represents a paginated query result
type PaginatedResult struct {
	Data       []types.BackupMeta `json:"data"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"pageSize"`
	TotalPages int                `json:"totalPages"`
}

// QueryOptions represents options for querying backups
type QueryOptions struct {
	// Filtering
	ServerName   string
	DatabaseName string
	BackupType   string
	Status       string
	StartDate    *time.Time
	EndDate      *time.Time
	SearchTerm   string
	ActiveOnly   bool

	// Pagination
	Page     int
	PageSize int

	// Sorting
	SortBy    string
	SortOrder string

	// Performance
	PreloadPaths bool
	SelectFields []string
}

// AddPerformanceIndexes adds optimized indexes for common query patterns
func AddPerformanceIndexes(db *gorm.DB) error {
	log.Println("Adding performance indexes to metadata tables...")

	// Composite indexes for common query patterns
	indexes := []struct {
		Table   string
		Name    string
		Columns []string
	}{
		// For filtering by server, database, type, and status
		{
			Table:   "backups",
			Name:    "idx_backups_filter",
			Columns: []string{"server_name", "database_name", "backup_type", "status"},
		},
		// For sorting by creation date
		{
			Table:   "idx_backups_created_desc",
			Name:    "backups",
			Columns: []string{"created_at DESC"},
		},
		// For retention queries
		{
			Table:   "backups",
			Name:    "idx_backups_expires",
			Columns: []string{"expires_at", "status"},
		},
		// For S3 upload status queries
		{
			Table:   "backups",
			Name:    "idx_backups_s3_status",
			Columns: []string{"s3_upload_status", "created_at"},
		},
		// For foreign key lookups
		{
			Table:   "local_paths",
			Name:    "idx_local_paths_backup",
			Columns: []string{"backup_id"},
		},
		{
			Table:   "s3_keys",
			Name:    "idx_s3_keys_backup",
			Columns: []string{"backup_id"},
		},
	}

	// Create indexes
	for _, idx := range indexes {
		sql := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s)",
			idx.Name, idx.Table, joinColumns(idx.Columns))

		if err := db.Exec(sql).Error; err != nil {
			log.Printf("Warning: Failed to create index %s: %v", idx.Name, err)
			// Continue with other indexes even if one fails
		}
	}

	// Analyze tables to update statistics
	tables := []string{"backups", "local_paths", "s3_keys", "metadata_stats"}
	for _, table := range tables {
		if err := db.Exec(fmt.Sprintf("ANALYZE TABLE %s", table)).Error; err != nil {
			log.Printf("Warning: Failed to analyze table %s: %v", table, err)
		}
	}

	log.Println("Performance indexes added successfully")
	return nil
}

// GetBackupsPaginated returns paginated backup results with optimized queries
func (s *DBStore) GetBackupsPaginated(opts QueryOptions) (*PaginatedResult, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Set defaults
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 {
		opts.PageSize = 50
	}
	if opts.PageSize > 1000 {
		opts.PageSize = 1000 // Max page size
	}
	if opts.SortBy == "" {
		opts.SortBy = "created_at"
	}
	if opts.SortOrder == "" {
		opts.SortOrder = "desc"
	}

	// Build base query
	query := s.db.Model(&DatabaseBackup{})

	// Apply filters
	query = applyFilters(query, opts)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count backups: %w", err)
	}

	// Apply sorting
	sortClause := fmt.Sprintf("%s %s", opts.SortBy, opts.SortOrder)
	query = query.Order(sortClause)

	// Apply pagination
	offset := (opts.Page - 1) * opts.PageSize
	query = query.Offset(offset).Limit(opts.PageSize)

	// Select specific fields if requested
	if len(opts.SelectFields) > 0 {
		query = query.Select(opts.SelectFields)
	}

	// Conditionally preload relationships
	if opts.PreloadPaths {
		query = query.Preload("LocalPaths").Preload("S3Keys")
	}

	// Execute query
	var dbBackups []DatabaseBackup
	if err := query.Find(&dbBackups).Error; err != nil {
		return nil, fmt.Errorf("failed to query backups: %w", err)
	}

	// Convert to BackupMeta format
	backups := convertToBackupMetas(dbBackups)

	// Calculate total pages
	totalPages := int(total) / opts.PageSize
	if int(total)%opts.PageSize > 0 {
		totalPages++
	}

	return &PaginatedResult{
		Data:       backups,
		Total:      total,
		Page:       opts.Page,
		PageSize:   opts.PageSize,
		TotalPages: totalPages,
	}, nil
}

// applyFilters applies query filters to the database query
func applyFilters(query *gorm.DB, opts QueryOptions) *gorm.DB {
	// Server filter
	if opts.ServerName != "" {
		query = query.Where("server_name = ?", opts.ServerName)
	}

	// Database filter
	if opts.DatabaseName != "" {
		query = query.Where("database_name = ?", opts.DatabaseName)
	}

	// Backup type filter
	if opts.BackupType != "" {
		query = query.Where("backup_type = ?", opts.BackupType)
	}

	// Status filter
	if opts.Status != "" {
		query = query.Where("status = ?", opts.Status)
	} else if opts.ActiveOnly {
		query = query.Where("status = ?", string(StatusSuccess))
	}

	// Date range filter
	if opts.StartDate != nil {
		query = query.Where("created_at >= ?", *opts.StartDate)
	}
	if opts.EndDate != nil {
		// Add 1 day to include the entire end date
		endDate := opts.EndDate.AddDate(0, 0, 1)
		query = query.Where("created_at < ?", endDate)
	}

	// Search filter
	if opts.SearchTerm != "" {
		searchPattern := "%" + opts.SearchTerm + "%"
		query = query.Where(
			"id LIKE ? OR server_name LIKE ? OR database_name LIKE ?",
			searchPattern, searchPattern, searchPattern,
		)
	}

	return query
}

// GetStatsOptimized returns statistics using database aggregations
func (s *DBStore) GetStatsOptimized() (map[string]interface{}, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	result := make(map[string]interface{})

	// Use goroutines for parallel queries
	type queryResult struct {
		name  string
		value interface{}
		err   error
	}

	queries := []func() queryResult{
		// Total count and sizes
		func() queryResult {
			var stats struct {
				TotalCount     int64
				TotalLocalSize int64
				TotalS3Size    int64
			}
			err := s.db.Model(&DatabaseBackup{}).
				Select(`
					COUNT(*) as total_count,
					COALESCE(SUM(CASE WHEN status = ? THEN size ELSE 0 END), 0) as total_local_size,
					COALESCE(SUM(CASE WHEN s3_upload_status = ? THEN size ELSE 0 END), 0) as total_s3_size
				`, string(StatusSuccess), string(StatusSuccess)).
				Scan(&stats).Error

			return queryResult{
				name: "totals",
				value: map[string]interface{}{
					"totalCount":     stats.TotalCount,
					"totalLocalSize": stats.TotalLocalSize,
					"totalS3Size":    stats.TotalS3Size,
				},
				err: err,
			}
		},

		// Status distribution
		func() queryResult {
			var counts []struct {
				Status string
				Count  int64
			}
			err := s.db.Model(&DatabaseBackup{}).
				Select("status, COUNT(*) as count").
				Group("status").
				Find(&counts).Error

			statusMap := make(map[string]int64)
			for _, c := range counts {
				statusMap[c.Status] = c.Count
			}

			return queryResult{name: "statusCounts", value: statusMap, err: err}
		},

		// Type distribution
		func() queryResult {
			var counts []struct {
				BackupType string `gorm:"column:backup_type"`
				Count      int64
			}
			err := s.db.Model(&DatabaseBackup{}).
				Select("backup_type, COUNT(*) as count").
				Group("backup_type").
				Find(&counts).Error

			typeMap := make(map[string]int64)
			for _, c := range counts {
				typeMap[c.BackupType] = c.Count
			}

			return queryResult{name: "typeDistribution", value: typeMap, err: err}
		},

		// Server distribution
		func() queryResult {
			var counts []struct {
				ServerName string `gorm:"column:server_name"`
				Count      int64
			}
			err := s.db.Model(&DatabaseBackup{}).
				Select("server_name, COUNT(*) as count").
				Group("server_name").
				Find(&counts).Error

			serverMap := make(map[string]int64)
			for _, c := range counts {
				serverMap[c.ServerName] = c.Count
			}

			return queryResult{name: "serverDistribution", value: serverMap, err: err}
		},

		// Recent activity
		func() queryResult {
			var activity struct {
				Last24Hours int64
				Last7Days   int64
				Last30Days  int64
			}
			now := time.Now()
			err := s.db.Model(&DatabaseBackup{}).
				Select(`
					COUNT(CASE WHEN created_at >= ? THEN 1 END) as last24_hours,
					COUNT(CASE WHEN created_at >= ? THEN 1 END) as last7_days,
					COUNT(CASE WHEN created_at >= ? THEN 1 END) as last30_days
				`,
					now.Add(-24*time.Hour),
					now.Add(-7*24*time.Hour),
					now.Add(-30*24*time.Hour)).
				Scan(&activity).Error

			return queryResult{
				name: "recentActivity",
				value: map[string]int64{
					"last24Hours": activity.Last24Hours,
					"last7Days":   activity.Last7Days,
					"last30Days":  activity.Last30Days,
				},
				err: err,
			}
		},
	}

	// Execute queries in parallel
	results := make(chan queryResult, len(queries))
	for _, queryFunc := range queries {
		go func(qf func() queryResult) {
			results <- qf()
		}(queryFunc)
	}

	// Collect results
	for i := 0; i < len(queries); i++ {
		res := <-results
		if res.err != nil {
			log.Printf("Error in stats query %s: %v", res.name, res.err)
			continue
		}
		result[res.name] = res.value
	}
	close(results)

	// Get last backup time (single query)
	var lastBackup struct {
		CompletedAt time.Time
	}
	err := s.db.Model(&DatabaseBackup{}).
		Where("status = ? AND completed_at IS NOT NULL", string(StatusSuccess)).
		Select("completed_at").
		Order("completed_at DESC").
		Limit(1).
		Scan(&lastBackup).Error

	if err == nil && !lastBackup.CompletedAt.IsZero() {
		result["lastBackupTime"] = lastBackup.CompletedAt
	}

	return result, nil
}

// CleanupOldBackups removes backups older than the retention period
func (s *DBStore) CleanupOldBackups(retentionDays int) (int64, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	// Use soft delete first
	result := s.db.Model(&DatabaseBackup{}).
		Where("created_at < ? AND status != ?", cutoffDate, string(StatusDeleted)).
		Update("status", string(StatusDeleted))

	if result.Error != nil {
		return 0, fmt.Errorf("failed to mark old backups as deleted: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// VacuumDatabase optimizes the database by running maintenance commands
func (s *DBStore) VacuumDatabase() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Optimize tables
	tables := []string{"backups", "local_paths", "s3_keys", "metadata_stats"}
	for _, table := range tables {
		if err := s.db.Exec(fmt.Sprintf("OPTIMIZE TABLE %s", table)).Error; err != nil {
			log.Printf("Warning: Failed to optimize table %s: %v", table, err)
			// Continue with other tables
		}
	}

	return nil
}

// GetBackupsByDateRange returns backups within a specific date range with minimal data
func (s *DBStore) GetBackupsByDateRange(start, end time.Time, fields []string) ([]types.BackupMeta, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	query := s.db.Model(&DatabaseBackup{}).
		Where("created_at BETWEEN ? AND ?", start, end).
		Order("created_at DESC")

	// Select only requested fields
	if len(fields) > 0 {
		query = query.Select(fields)
	}

	var dbBackups []DatabaseBackup
	if err := query.Find(&dbBackups).Error; err != nil {
		return nil, fmt.Errorf("failed to query backups by date range: %w", err)
	}

	return convertToBackupMetas(dbBackups), nil
}

// BulkUpdateStatus updates multiple backup statuses in a single transaction
func (s *DBStore) BulkUpdateStatus(updates map[string]types.BackupStatus) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.db.Transaction(func(tx *gorm.DB) error {
		for id, status := range updates {
			if err := tx.Model(&DatabaseBackup{}).
				Where("id = ?", id).
				Update("status", string(status)).Error; err != nil {
				return fmt.Errorf("failed to update backup %s: %w", id, err)
			}
		}
		return nil
	})
}

// joinColumns joins column names for SQL
func joinColumns(columns []string) string {
	result := ""
	for i, col := range columns {
		if i > 0 {
			result += ", "
		}
		result += col
	}
	return result
}

// EnableQueryLogging enables slow query logging for performance monitoring
func EnableQueryLogging(db *gorm.DB, threshold time.Duration) {
	db.Callback().Query().After("gorm:query").Register("log_slow_queries", func(tx *gorm.DB) {
		elapsed := time.Since(tx.Statement.Context.Value(startTimeKey).(time.Time))
		if elapsed > threshold {
			log.Printf("Slow query detected (%.3fs): %s", elapsed.Seconds(), tx.Statement.SQL.String())
		}
	})

	db.Callback().Query().Before("gorm:query").Register("set_query_start_time", func(tx *gorm.DB) {
		tx.Statement.Context = context.WithValue(tx.Statement.Context, startTimeKey, time.Now())
	})
}
