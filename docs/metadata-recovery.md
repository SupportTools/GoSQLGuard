# Metadata Recovery Procedures

This document describes how to recover GoSQLGuard metadata in case of corruption or data loss.

## Overview

GoSQLGuard stores backup metadata in two possible locations:
1. **File-based storage**: JSON file in the backup directory
2. **MySQL database**: Dedicated tables in a MySQL database

## File-Based Metadata Recovery

### Location
- Default: `{backup_directory}/metadata.json`
- Backup: `{backup_directory}/metadata.json.backup` (if implemented)

### Recovery Procedures

#### 1. Corrupted JSON File

If the metadata.json file is corrupted:

```bash
# Check if backup exists
ls -la /path/to/backups/metadata.json*

# If backup exists, restore it
cp /path/to/backups/metadata.json.backup /path/to/backups/metadata.json

# If no backup, try to repair JSON
# First, make a copy
cp /path/to/backups/metadata.json /path/to/backups/metadata.json.corrupt

# Try to pretty-print and validate
python -m json.tool /path/to/backups/metadata.json.corrupt > /path/to/backups/metadata.json.temp

# If that fails, you may need to manually edit the file
# Common issues: missing closing brackets, truncated file
```

#### 2. Missing Metadata File

If the metadata file is completely missing:

```bash
# Option 1: Restore from backup
# Check your backup systems for a copy of metadata.json

# Option 2: Rebuild from backup files (see Section: Rebuilding Metadata)
```

#### 3. Partial Data Loss

If some entries are missing but the file is valid:

```bash
# 1. Make a backup of current metadata
cp /path/to/backups/metadata.json /path/to/backups/metadata.json.partial

# 2. Use the rebuild tool to scan for missing backups
# (This would be ST-02 implementation)
```

## MySQL Metadata Recovery

### Database Structure

```sql
-- Check metadata tables
SHOW TABLES LIKE '%backup%';

-- Key tables:
-- - backups: Main backup records
-- - local_paths: Local storage paths
-- - s3_keys: S3 storage keys
-- - metadata_stats: Statistics
```

### Recovery Procedures

#### 1. Database Connection Issues

```bash
# Test connection
mysql -h <host> -u <user> -p<password> <database> -e "SELECT 1"

# If connection fails, GoSQLGuard will fall back to file-based storage
```

#### 2. Corrupted Tables

```sql
-- Check table status
CHECK TABLE backups;
ANALYZE TABLE backups;

-- Repair if needed
REPAIR TABLE backups;

-- If severe corruption, restore from MySQL backup
```

#### 3. Missing Data

```sql
-- Check for orphaned records
SELECT * FROM backups WHERE id NOT IN (SELECT backup_id FROM local_paths);

-- Find backups without metadata
-- (Would be implemented as part of ST-02)
```

## Rebuilding Metadata from Backup Files

### Manual Process

1. **List all backup files**:
```bash
find /path/to/backups -name "*.sql.gz" -type f > backup_files.txt
```

2. **Extract metadata from filenames**:
```bash
# Backup filenames follow pattern: {server}-{database}-{type}-{timestamp}.sql.gz
while read file; do
    basename=$(basename "$file")
    # Parse filename components
    echo "$file: $basename"
done < backup_files.txt
```

3. **Create new metadata structure**:
```json
{
  "backups": [],
  "lastUpdated": "2024-01-20T10:00:00Z",
  "version": "1.0"
}
```

4. **Add entries for each backup**:
```json
{
  "id": "server1-testdb-daily-20240120-100000",
  "serverName": "server1",
  "database": "testdb",
  "backupType": "daily",
  "createdAt": "2024-01-20T10:00:00Z",
  "size": 1048576,
  "localPaths": {
    "default": "/path/to/backup.sql.gz"
  },
  "status": "success"
}
```

### Automated Recovery Tool

GoSQLGuard includes a `metadata-recovery` command-line tool that automatically reconstructs metadata from existing backup files.

#### Usage

```bash
# Basic usage - scan both local and S3 storage
metadata-recovery -config /path/to/config.yaml

# Dry run - preview what would be recovered without saving
metadata-recovery -config /path/to/config.yaml -dry-run

# Force rebuild even if metadata exists
metadata-recovery -config /path/to/config.yaml -force

# Merge with existing metadata instead of replacing
metadata-recovery -config /path/to/config.yaml -merge

# Scan only local storage
metadata-recovery -config /path/to/config.yaml -s3=false

# Verbose output
metadata-recovery -config /path/to/config.yaml -verbose
```

#### Options

- `-config`: Path to GoSQLGuard configuration file (default: "config.yaml")
- `-output`: Output file for recovered metadata (default: auto-detect from config)
- `-dry-run`: Perform a dry run without writing metadata
- `-verbose`: Enable verbose logging
- `-local`: Scan local storage for backups (default: true)
- `-s3`: Scan S3 storage for backups (default: true)
- `-force`: Force rebuild even if metadata exists
- `-merge`: Merge with existing metadata instead of replacing

#### What it does

1. Scans local and S3 storage for backup files matching the pattern: `{server}-{database}-{type}-{timestamp}.sql.gz`
2. Parses filenames to extract metadata (server, database, backup type, timestamp)
3. Gets file sizes and modification times
4. Rebuilds the metadata structure
5. Handles conflicts and duplicates (prefers files that exist in both local and S3)
6. Validates the rebuilt metadata
7. Saves the metadata (unless -dry-run is specified)

#### Example Output

```
Starting metadata recovery process...
Found 48 backups in local storage
Found 72 backups in S3 storage
Recovered backup: server1-testdb-hourly-20250523-100000
Recovered backup: server1-testdb-hourly-20250523-110000
...

Recovery Summary:
- Total backups recovered: 96
- Successful backups: 96
- Failed backups: 0
- Total local size: 1.2 GB
- Total S3 size: 2.4 GB
Metadata saved successfully!
```

## Prevention Best Practices

### 1. Regular Backups

```bash
# Add to cron
0 * * * * cp /path/to/backups/metadata.json /path/to/backups/metadata.json.backup
```

### 2. Use MySQL Metadata Storage

For production environments, use MySQL storage for better reliability:

```yaml
metadata_database:
  enabled: true
  host: "localhost"
  database: "gosqlguard_metadata"
  autoMigrate: true
```

### 3. Monitor Metadata Health

```bash
# Check metadata file size (shouldn't be 0)
ls -la /path/to/backups/metadata.json

# Validate JSON structure
python -m json.tool /path/to/backups/metadata.json > /dev/null && echo "Valid" || echo "Invalid"

# Check last modified time
stat /path/to/backups/metadata.json
```

### 4. Implement Atomic Writes

The current implementation should use atomic writes:
1. Write to temporary file
2. Validate the temporary file
3. Rename (atomic operation) to replace original

## Testing Recovery Procedures

Run the metadata persistence tests to verify recovery works:

```bash
# Run all metadata tests
./scripts/run-metadata-tests.sh

# Test specific recovery scenario
go test -v ./pkg/metadata -run TestFileStoreCorruptedFile
go test -v ./pkg/metadata -run TestMetadataCorruptionRecovery -tags=integration
```

## Emergency Recovery Checklist

- [ ] Stop GoSQLGuard to prevent further writes
- [ ] Make backup copies of all metadata files
- [ ] Check system logs for errors
- [ ] Attempt recovery using procedures above
- [ ] Validate recovered metadata
- [ ] Test with read-only operations first
- [ ] Resume normal operations
- [ ] Implement prevention measures

## Support

If metadata recovery fails:
1. Check GoSQLGuard logs for specific errors
2. Ensure backup files are intact
3. Contact support with:
   - Error messages
   - Metadata file samples
   - System configuration