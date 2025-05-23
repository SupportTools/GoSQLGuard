# Multi-Server Configuration Guide

This guide explains how to configure and manage multiple database servers with GoSQLGuard.

## Table of Contents

1. [Overview](#overview)
2. [Configuration](#configuration)
3. [Server Organization Strategies](#server-organization-strategies)
4. [Database Filtering](#database-filtering)
5. [MySQL Dump Options](#mysql-dump-options)
6. [Authentication](#authentication)
7. [Backup Scheduling](#backup-scheduling)
8. [Storage Organization](#storage-organization)
9. [UI Management](#ui-management)
10. [Best Practices](#best-practices)
11. [Common Scenarios](#common-scenarios)
12. [Troubleshooting](#troubleshooting)

## Overview

GoSQLGuard supports backing up multiple database servers from a single instance. This allows you to:

- Centralize backup management for all your database servers
- Apply different backup policies to different servers
- Organize backups by server, type, or custom strategies
- Monitor all backups from a single dashboard

## Configuration

### Basic Multi-Server Setup

```yaml
# config.yaml
database_servers:
  - name: "production-primary"
    type: "mysql"
    host: "prod-db-1.example.com"
    port: "3306"
    username: "backup_user"
    password: "secure_password"
    
  - name: "production-replica"
    type: "mysql"
    host: "prod-db-2.example.com"
    port: "3306"
    username: "backup_user"
    password: "secure_password"
    
  - name: "development"
    type: "mysql"
    host: "dev-db.example.com"
    port: "3306"
    username: "backup_user"
    password: "secure_password"
```

### Environment Variables

You can use environment variables for sensitive information:

```yaml
database_servers:
  - name: "production"
    type: "mysql"
    host: "${PROD_DB_HOST}"
    port: "${PROD_DB_PORT}"
    username: "${PROD_DB_USER}"
    password: "${PROD_DB_PASSWORD}"
```

## Server Organization Strategies

GoSQLGuard offers three strategies for organizing backup files:

### 1. By Server (Default)

```yaml
backup_organization: "by-server"
```

Directory structure:
```
/backups/
├── production-primary/
│   ├── hourly/
│   ├── daily/
│   └── weekly/
├── production-replica/
│   ├── hourly/
│   ├── daily/
│   └── weekly/
└── development/
    ├── hourly/
    └── daily/
```

### 2. By Type

```yaml
backup_organization: "by-type"
```

Directory structure:
```
/backups/
├── hourly/
│   ├── production-primary-*.sql.gz
│   ├── production-replica-*.sql.gz
│   └── development-*.sql.gz
├── daily/
│   ├── production-primary-*.sql.gz
│   ├── production-replica-*.sql.gz
│   └── development-*.sql.gz
└── weekly/
    └── production-primary-*.sql.gz
```

### 3. Combined (Flat)

```yaml
backup_organization: "combined"
```

All backups in a single directory:
```
/backups/
├── production-primary-hourly-*.sql.gz
├── production-primary-daily-*.sql.gz
├── production-replica-hourly-*.sql.gz
└── development-daily-*.sql.gz
```

## Database Filtering

Control which databases to backup on each server:

### Include Specific Databases

```yaml
database_servers:
  - name: "production"
    # ... connection details ...
    include_databases:
      - "users"
      - "orders"
      - "products"
```

### Exclude Specific Databases

```yaml
database_servers:
  - name: "development"
    # ... connection details ...
    exclude_databases:
      - "test_*"
      - "tmp_*"
      - "backup_*"
```

### Combining Include and Exclude

When both are specified, includes are processed first, then excludes:

```yaml
database_servers:
  - name: "reporting"
    # ... connection details ...
    include_databases:
      - "analytics_*"
      - "reports_*"
    exclude_databases:
      - "*_temp"
      - "*_old"
```

## MySQL Dump Options

Configure server-specific MySQL dump options:

### Global Options

```yaml
mysql_dump_options:
  single_transaction: true
  lock_tables: false
  quick: true
  compress: true
```

### Server-Specific Options

```yaml
database_servers:
  - name: "production"
    # ... connection details ...
    mysql_dump_options:
      single_transaction: true
      master_data: 2
      routines: true
      triggers: true
      
  - name: "development"
    # ... connection details ...
    mysql_dump_options:
      lock_tables: true  # OK for dev environment
      skip_comments: true
```

### Backup Type-Specific Options

```yaml
backup_types:
  hourly:
    mysql_dump_options:
      skip_extended_insert: false
      quick: true
      
  daily:
    mysql_dump_options:
      extended_insert: true
      routines: true
      events: true
```

## Authentication

### MySQL 8.0 Authentication Plugins

For MySQL 8.0+ servers using caching_sha2_password:

```yaml
database_servers:
  - name: "mysql8-server"
    type: "mysql"
    host: "mysql8.example.com"
    port: "3306"
    username: "backup_user"
    password: "secure_password"
    auth_plugin: "caching_sha2_password"
```

### Using MySQL Configuration Files

```yaml
database_servers:
  - name: "production"
    type: "mysql"
    host: "prod-db.example.com"
    defaults_file: "/etc/mysql/backup.cnf"
```

Example `/etc/mysql/backup.cnf`:
```ini
[client]
user=backup_user
password=secure_password
ssl-ca=/etc/mysql/ca.pem
ssl-cert=/etc/mysql/client-cert.pem
ssl-key=/etc/mysql/client-key.pem
```

## Backup Scheduling

### Different Schedules per Server

```yaml
backup_schedules:
  hourly:
    schedule: "0 * * * *"
    servers:
      - "production-primary"
      - "production-replica"
    
  daily:
    schedule: "0 2 * * *"
    servers:
      - "production-primary"
      - "development"
      
  weekly:
    schedule: "0 3 * * 0"
    servers:
      - "production-primary"
```

### Database-Specific Schedules

```yaml
backup_schedules:
  hourly_critical:
    schedule: "0 * * * *"
    servers:
      - name: "production"
        databases:
          - "users"
          - "orders"
          
  daily_all:
    schedule: "0 2 * * *"
    servers:
      - "production"
      - "development"
```

## Storage Organization

### S3 Storage with Server Prefixes

```yaml
s3:
  enabled: true
  bucket: "company-backups"
  prefix: "gosqlguard"
  organization_strategy: "by-server"  # Creates server subdirectories in S3
```

S3 structure:
```
s3://company-backups/gosqlguard/
├── production-primary/
│   ├── hourly/
│   └── daily/
├── production-replica/
│   ├── hourly/
│   └── daily/
└── development/
    └── daily/
```

## UI Management

### Viewing Server Status

The GoSQLGuard UI provides several ways to manage multi-server setups:

1. **Dashboard**: Overview of all servers with backup counts and status
2. **Servers Page** (`/servers`): Detailed server management and statistics
3. **Backup Status**: Filter backups by server
4. **Manual Backups**: Select specific servers for on-demand backups

### Server Statistics

The dashboard shows:
- Total backups per server
- Success rate percentage
- Last backup time
- Storage usage per server

## Best Practices

### 1. Server Naming Conventions

Use descriptive, consistent names:
```yaml
# Good naming
database_servers:
  - name: "prod-mysql-primary-us-east"
  - name: "prod-mysql-replica-us-east"
  - name: "staging-mysql-us-west"
  - name: "dev-mysql-local"

# Avoid unclear names
database_servers:
  - name: "db1"
  - name: "backup-server"
  - name: "mysql"
```

### 2. Security Best Practices

1. **Use Dedicated Backup Users**:
   ```sql
   CREATE USER 'backup'@'%' IDENTIFIED BY 'strong_password';
   GRANT SELECT, LOCK TABLES, SHOW VIEW, RELOAD, REPLICATION CLIENT, EVENT, TRIGGER ON *.* TO 'backup'@'%';
   ```

2. **Use SSL/TLS Connections**:
   ```yaml
   database_servers:
     - name: "production"
       # ... other config ...
       ssl_mode: "REQUIRED"
       ssl_ca: "/path/to/ca.pem"
   ```

3. **Rotate Credentials Regularly**:
   - Use environment variables
   - Implement credential rotation
   - Monitor access logs

### 3. Resource Management

1. **Stagger Backup Times**:
   ```yaml
   backup_schedules:
     prod_hourly:
       schedule: "0 * * * *"
       servers: ["prod-primary"]
       
     prod_replica_hourly:
       schedule: "30 * * * *"  # 30 minutes offset
       servers: ["prod-replica"]
   ```

2. **Limit Concurrent Backups**:
   ```yaml
   performance:
     max_concurrent_backups: 2
     server_parallelism:
       "production": 1  # Only 1 backup at a time for production
       "development": 3  # Allow 3 concurrent backups for dev
   ```

### 4. Monitoring and Alerting

1. **Monitor Backup Success Rates**:
   - Set up alerts for failed backups
   - Track backup duration trends
   - Monitor storage usage

2. **Regular Testing**:
   - Perform restore tests monthly
   - Verify backup integrity
   - Document restore procedures

## Common Scenarios

### Scenario 1: Production and Development Separation

```yaml
# Separate production and development with different policies
database_servers:
  - name: "production"
    host: "prod-db.example.com"
    # ... connection details ...
    
  - name: "development"
    host: "dev-db.example.com"
    # ... connection details ...

backup_types:
  hourly:
    local:
      enabled: true
      retention:
        count: 24  # Keep 24 hours
    servers:
      - "production"  # Only production gets hourly
      
  daily:
    local:
      enabled: true
      retention:
        count: 7
    servers:
      - "production"
      - "development"  # Both get daily
```

### Scenario 2: Geographic Distribution

```yaml
# Servers in different regions
database_servers:
  - name: "us-east-primary"
    host: "db1.us-east.example.com"
    # ... connection details ...
    
  - name: "eu-west-primary"
    host: "db1.eu-west.example.com"
    # ... connection details ...
    
  - name: "ap-south-primary"
    host: "db1.ap-south.example.com"
    # ... connection details ...

# Region-specific S3 buckets
s3:
  enabled: true
  buckets:
    us-east: "backups-us-east"
    eu-west: "backups-eu-west"
    ap-south: "backups-ap-south"
```

### Scenario 3: Database Sharding

```yaml
# Multiple shards of the same application
database_servers:
  - name: "users-shard-1"
    host: "users-db-1.example.com"
    include_databases: ["users_1"]
    
  - name: "users-shard-2"
    host: "users-db-2.example.com"
    include_databases: ["users_2"]
    
  - name: "users-shard-3"
    host: "users-db-3.example.com"
    include_databases: ["users_3"]

# Coordinated backup schedule
backup_schedules:
  shard_sync:
    schedule: "0 2 * * *"
    servers:
      - "users-shard-1"
      - "users-shard-2"
      - "users-shard-3"
```

## Troubleshooting

### Common Issues

#### 1. Connection Failures

**Problem**: Cannot connect to one or more servers

**Solutions**:
- Verify network connectivity: `telnet <host> <port>`
- Check firewall rules
- Verify credentials
- Check MySQL user permissions
- Review authentication plugin compatibility

**Debug Commands**:
```bash
# Test connection
mysql -h <host> -P <port> -u <user> -p

# Check grants
SHOW GRANTS FOR 'backup_user'@'%';
```

#### 2. Performance Issues

**Problem**: Backups taking too long

**Solutions**:
- Use `--single-transaction` for InnoDB
- Enable compression
- Limit concurrent backups
- Use dedicated replica for backups
- Optimize network bandwidth

**Configuration**:
```yaml
database_servers:
  - name: "production-replica"
    # Use replica for backups
    mysql_dump_options:
      single_transaction: true
      compress: true
      quick: true
```

#### 3. Storage Issues

**Problem**: Running out of storage space

**Solutions**:
- Implement aggressive retention policies
- Use S3 lifecycle policies
- Monitor storage usage
- Compress backups more aggressively

**Retention Configuration**:
```yaml
backup_types:
  hourly:
    local:
      retention:
        count: 6  # Only keep 6 hours locally
    s3:
      retention:
        duration: "7d"  # Keep 7 days in S3
```

#### 4. Scheduling Conflicts

**Problem**: Backups overlapping or not running

**Solutions**:
- Check cron expressions
- Verify server time zones
- Monitor scheduler logs
- Use mutex locks for critical backups

**Debug Logs**:
```bash
# Check scheduler logs
grep "scheduler" /var/log/gosqlguard.log

# Verify cron syntax
crontab -l
```

### Monitoring Commands

```bash
# Check backup status for specific server
curl http://localhost:8080/api/backups?server=production

# Get server statistics
curl http://localhost:8080/api/stats

# Monitor real-time logs
tail -f /var/log/gosqlguard.log | grep "production"
```

### Performance Tuning

1. **Database-Side Optimization**:
   ```sql
   -- Create dedicated backup user with minimal permissions
   CREATE USER 'backup'@'10.0.0.%' IDENTIFIED BY 'password';
   GRANT SELECT, LOCK TABLES, SHOW VIEW, RELOAD, REPLICATION CLIENT ON *.* TO 'backup'@'10.0.0.%';
   
   -- For MySQL 8.0+
   ALTER USER 'backup'@'10.0.0.%' WITH MAX_USER_CONNECTIONS 5;
   ```

2. **Network Optimization**:
   ```yaml
   # Use compression for remote servers
   database_servers:
     - name: "remote-production"
       mysql_dump_options:
         compress: true
         protocol: "tcp"
         ssl_mode: "PREFERRED"
   ```

3. **Resource Limits**:
   ```yaml
   # Limit resource usage
   performance:
     max_concurrent_backups: 3
     network_bandwidth_limit: "100MB"  # Per backup
     cpu_priority: "low"
   ```

## Conclusion

Multi-server support in GoSQLGuard provides flexible, centralized backup management for complex database infrastructures. By following the practices outlined in this guide, you can implement a robust backup strategy that scales with your needs while maintaining security and performance.