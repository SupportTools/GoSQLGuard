# GoSQLGuard Example Configurations

This directory contains example configurations demonstrating various ways to configure GoSQLGuard. These examples highlight how to selectively enable specific backup schedules while omitting others.

## Key Configuration Behavior

**Important:** GoSQLGuard will only run backup schedules that are explicitly defined in the configuration. If a schedule type (hourly, daily, weekly, etc.) is not included in the `backupTypes` section, it will not run.

From `pkg/backup/backup.go`:
```go
// Check if this backup type is configured
typeConfig, exists := m.cfg.BackupTypes[backupType]
if !exists {
    return fmt.Errorf("no configuration found for backup type: %s", backupType)
}
```

## Available Examples

1. **basic-mysql-config.yaml**
   - Simple single MySQL server configuration
   - Only defines daily and weekly backups (hourly omitted)
   - Local backups only (S3 disabled)

2. **custom-schedules-config.yaml**
   - Uses custom schedule names (nightly, monthly, quarterly)
   - No standard hourly/weekly schedules
   - Demonstrates custom cron schedule patterns

3. **enterprise-s3-config.yaml**
   - Multiple database servers (MySQL and PostgreSQL)
   - S3 storage with advanced options (MinIO compatibility)
   - Only runs daily and weekly backups (hourly omitted)

4. **selective-schedules-config.yaml**
   - Explicitly demonstrates omitting hourly and daily backups
   - Only enables weekly, monthly, and custom backups
   - Custom "dev-backup" schedule for development environments

## Usage

To use any of these examples:

1. Copy the desired configuration file to your GoSQLGuard installation directory
2. Rename it to `config.yaml` (or your preferred config name)
3. Update values like hostnames, credentials, etc. to match your environment
4. Start GoSQLGuard with the configuration file:

```bash
./GoSQLGuard --config=config.yaml
```

You can also set the configuration path using the `CONFIG_PATH` environment variable:

```bash
CONFIG_PATH=/path/to/config.yaml ./GoSQLGuard
