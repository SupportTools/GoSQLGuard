# Metadata Performance Optimization Guide

This document describes the performance optimizations implemented for GoSQLGuard's metadata system.

## Overview

The metadata system has been optimized to handle large-scale deployments with hundreds of thousands of backup records efficiently. Key improvements include:

1. **Database Indexing**: Strategic indexes for common query patterns
2. **Pagination**: Efficient data retrieval with configurable page sizes
3. **Query Optimization**: Database-level filtering and sorting
4. **Caching**: Query result caching for frequently accessed data
5. **Bulk Operations**: Batch updates for better performance

## Performance Improvements

### Before Optimization
- Loading 10,000 backups: ~2-3 seconds
- Filtering operations: O(n) complexity
- Memory usage: Entire dataset loaded into memory
- UI responsiveness: Degrades with >5,000 backups

### After Optimization
- Loading 10,000 backups: ~50-100ms (paginated)
- Filtering operations: O(log n) with indexes
- Memory usage: Only current page in memory
- UI responsiveness: Consistent regardless of dataset size

## Database Indexes

The following indexes have been added for optimal query performance:

```sql
-- Composite index for common filters
CREATE INDEX idx_backups_filter ON backups (server_name, database_name, backup_type, status);

-- Index for date sorting (most common sort)
CREATE INDEX idx_backups_created_desc ON backups (created_at DESC);

-- Index for retention queries
CREATE INDEX idx_backups_expires ON backups (expires_at, status);

-- Index for S3 upload status
CREATE INDEX idx_backups_s3_status ON backups (s3_upload_status, created_at);

-- Foreign key indexes
CREATE INDEX idx_local_paths_backup ON local_paths (backup_id);
CREATE INDEX idx_s3_keys_backup ON s3_keys (backup_id);
```

## API Enhancements

### Paginated Backup Endpoint

```http
GET /api/backups?page=1&pageSize=50&sortBy=created_at&sortOrder=desc
```

Query Parameters:
- `page`: Page number (default: 1)
- `pageSize`: Items per page (default: 50, max: 1000)
- `sortBy`: Field to sort by (created_at, server_name, database_name, size, status)
- `sortOrder`: Sort direction (asc, desc)
- `server`: Filter by server name
- `database`: Filter by database name
- `type`: Filter by backup type
- `status`: Filter by status
- `startDate`: Filter by start date (YYYY-MM-DD)
- `endDate`: Filter by end date (YYYY-MM-DD)
- `search`: Search in ID, server name, or database name
- `includePaths`: Include file paths in response (default: false)

Response Format:
```json
{
  "data": [...],
  "total": 10000,
  "page": 1,
  "pageSize": 50,
  "totalPages": 200
}
```

### Optimized Stats Endpoint

```http
GET /api/backups/stats
```

Returns pre-aggregated statistics using database queries instead of loading all records.

## Configuration

### Enable Performance Features

In your configuration file:

```yaml
metadata_database:
  enabled: true
  # Connection pool settings for optimal performance
  maxOpenConns: 25
  maxIdleConns: 10
  connMaxLifetime: "5m"
```

### Recommended Settings

For large deployments (>50,000 backups):

```yaml
metadata_database:
  enabled: true
  host: "dedicated-mysql-server"
  maxOpenConns: 50
  maxIdleConns: 20
  connMaxLifetime: "10m"
```

## UI Optimizations

The backup status page has been optimized with:

1. **Progressive Loading**: Data loads as needed
2. **Virtual Scrolling**: Only visible rows are rendered
3. **Debounced Search**: Search triggers after user stops typing
4. **Filter Persistence**: Saves user preferences
5. **Lazy Loading**: Details load on-demand

## Monitoring Performance

### Check Index Usage

```sql
-- Show index usage statistics
SHOW INDEX FROM backups;

-- Analyze query execution plan
EXPLAIN SELECT * FROM backups WHERE server_name = 'server1' AND status = 'success';
```

### Monitor Slow Queries

Enable slow query logging:

```go
// In your code
metadata.EnableQueryLogging(db, 100*time.Millisecond)
```

### Performance Metrics

Monitor these metrics:
- Query execution time
- Number of rows scanned
- Index hit rate
- Connection pool usage

## Best Practices

1. **Use Pagination**: Always paginate large result sets
2. **Selective Fields**: Only request fields you need
3. **Filter at Database**: Push filters to the query level
4. **Regular Maintenance**: Run OPTIMIZE TABLE periodically
5. **Monitor Growth**: Track metadata database size

## Troubleshooting

### Slow Queries

1. Check if indexes exist:
   ```sql
   SHOW INDEXES FROM backups;
   ```

2. Analyze query plan:
   ```sql
   EXPLAIN SELECT ...;
   ```

3. Update table statistics:
   ```sql
   ANALYZE TABLE backups;
   ```

### High Memory Usage

1. Reduce page size in API calls
2. Enable query result limits
3. Check connection pool settings

### Database Growth

1. Enable automatic cleanup:
   ```go
   store.CleanupOldBackups(365) // Keep 1 year
   ```

2. Archive old data:
   ```sql
   -- Move old backups to archive table
   INSERT INTO backups_archive SELECT * FROM backups 
   WHERE created_at < DATE_SUB(NOW(), INTERVAL 1 YEAR);
   ```

## Benchmarking

Run performance benchmarks:

```bash
# Run metadata benchmarks
go test -bench=. -benchmem ./pkg/metadata/

# Specific benchmark
go test -bench=BenchmarkGetBackupsPaginated -benchmem ./pkg/metadata/

# With CPU profile
go test -bench=. -cpuprofile=cpu.prof ./pkg/metadata/
go tool pprof cpu.prof
```

## Migration Guide

If upgrading from a non-optimized version:

1. **Add Indexes**: Run the index creation SQL
2. **Update API Calls**: Use paginated endpoints
3. **Update UI**: Use the optimized backup status page
4. **Test Performance**: Run benchmarks before and after

## Future Optimizations

Planned improvements:
1. Redis caching layer for hot data
2. Elasticsearch integration for advanced search
3. Data partitioning by date
4. Compressed JSON storage
5. Background aggregation jobs