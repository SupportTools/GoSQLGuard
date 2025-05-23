# Multi-Server Troubleshooting Guide

This guide helps diagnose and resolve common issues when running GoSQLGuard with multiple database servers.

## Quick Diagnostics

### 1. Check Server Connectivity

```bash
# Test all configured servers
for server in prod-primary prod-replica staging; do
  echo "Testing $server..."
  mysql -h $server.example.com -u backup_user -p -e "SELECT 1"
done

# Check network connectivity
telnet prod-db.example.com 3306

# Test with GoSQLGuard's connection
curl http://localhost:8080/api/servers/test -X POST \
  -H "Content-Type: application/json" \
  -d '{"name":"prod-primary","host":"prod-db.example.com","port":"3306","username":"backup_user","password":"xxx"}'
```

### 2. View Server Status

```bash
# Check all servers status
curl http://localhost:8080/api/servers | jq

# Check specific server backups
curl "http://localhost:8080/api/backups?server=prod-primary" | jq

# View server statistics
curl http://localhost:8080/api/stats | jq '.serverDistribution'
```

### 3. Check Logs

```bash
# View logs for specific server
grep "prod-primary" /var/log/gosqlguard.log | tail -50

# Check for connection errors
grep -i "connection\|auth\|failed" /var/log/gosqlguard.log | grep "server:"

# Monitor real-time logs
tail -f /var/log/gosqlguard.log | grep -E "(prod-primary|ERROR)"
```

## Common Issues and Solutions

### Issue 1: Server Not Backing Up

**Symptoms:**
- No backups appearing for specific server
- Server missing from backup lists
- Scheduled backups not running

**Diagnosis:**
```bash
# Check if server is in schedule
grep -A5 -B5 "server_name" config.yaml

# Verify server appears in metadata
curl http://localhost:8080/api/servers | jq '.[] | select(.name=="prod-primary")'

# Check scheduler logs
grep "scheduler.*prod-primary" /var/log/gosqlguard.log
```

**Solutions:**

1. **Server not in schedule:**
   ```yaml
   backup_schedules:
     daily:
       schedule: "0 2 * * *"
       servers:
         - "prod-primary"  # Add server here
       type: "daily"
   ```

2. **Server configuration issue:**
   ```yaml
   database_servers:
     - name: "prod-primary"
       type: "mysql"  # Ensure type is correct
       host: "correct-hostname.com"  # Verify hostname
       port: "3306"  # Check port
   ```

3. **Authentication problem:**
   ```sql
   -- On the MySQL server
   SHOW GRANTS FOR 'backup_user'@'%';
   
   -- Grant necessary permissions
   GRANT SELECT, LOCK TABLES, SHOW VIEW, RELOAD, REPLICATION CLIENT, EVENT, TRIGGER 
   ON *.* TO 'backup_user'@'backup-server-ip';
   ```

### Issue 2: Connection Timeouts

**Symptoms:**
- Backups start but timeout
- "Lost connection to MySQL server" errors
- Partial backups

**Diagnosis:**
```bash
# Test long-running query
time mysql -h prod-db.example.com -u backup_user -p \
  -e "SELECT COUNT(*) FROM large_table"

# Check network latency
ping -c 10 prod-db.example.com

# Monitor backup duration
grep "Backup completed.*prod-primary" /var/log/gosqlguard.log | \
  awk '{print $1, $2, $NF}'
```

**Solutions:**

1. **Increase timeouts:**
   ```yaml
   database_servers:
     - name: "prod-primary"
       mysql_dump_options:
         net_read_timeout: 600
         net_write_timeout: 600
   
   performance:
     network_timeout: "600s"
   ```

2. **Use compression for remote servers:**
   ```yaml
   mysql_dump_options:
     compress: true  # Reduce network traffic
   ```

3. **Optimize backup query:**
   ```yaml
   mysql_dump_options:
     single_transaction: true  # For InnoDB
     quick: true  # Don't buffer query
     skip_lock_tables: true  # Avoid locks
   ```

### Issue 3: Authentication Plugin Errors

**Symptoms:**
- "Plugin caching_sha2_password could not be loaded"
- "Authentication plugin not supported"
- MySQL 8.0 connection failures

**Diagnosis:**
```bash
# Check MySQL version
mysql -h prod-db.example.com -u backup_user -p \
  -e "SELECT VERSION()"

# Check user authentication plugin
mysql -h prod-db.example.com -u backup_user -p \
  -e "SELECT user, host, plugin FROM mysql.user WHERE user='backup_user'"
```

**Solutions:**

1. **Specify authentication plugin:**
   ```yaml
   database_servers:
     - name: "mysql8-server"
       auth_plugin: "caching_sha2_password"
   ```

2. **Use mysql_native_password (less secure):**
   ```sql
   ALTER USER 'backup_user'@'%' 
   IDENTIFIED WITH mysql_native_password BY 'password';
   ```

3. **Update GoSQLGuard Docker image:**
   ```dockerfile
   # Ensure using Ubuntu-based image with MySQL 8.0 client
   FROM supporttools/gosqlguard:latest
   ```

### Issue 4: Overlapping Backups

**Symptoms:**
- High server load during backups
- Backups taking longer than scheduled interval
- "Previous backup still running" errors

**Diagnosis:**
```bash
# Check concurrent backups
ps aux | grep mysqldump | grep -v grep

# Monitor backup overlap
grep "Backup started\|completed" /var/log/gosqlguard.log | \
  grep "prod-primary" | tail -20

# Check system resources during backup
top -b -n 1 | head -20
```

**Solutions:**

1. **Limit concurrent backups:**
   ```yaml
   performance:
     max_concurrent_backups: 2
     server_parallelism:
       "production": 1  # Only 1 backup at a time
   ```

2. **Stagger backup schedules:**
   ```yaml
   backup_schedules:
     prod_primary_daily:
       schedule: "0 2 * * *"  # 2 AM
       servers: ["prod-primary"]
       
     prod_replica_daily:
       schedule: "0 3 * * *"  # 3 AM (1 hour later)
       servers: ["prod-replica"]
   ```

3. **Use different backup types:**
   ```yaml
   # Light hourly backups
   hourly:
     mysql_dump_options:
       quick: true
       skip_extended_insert: true
   
   # Full daily backups
   daily:
     mysql_dump_options:
       extended_insert: true
       routines: true
   ```

### Issue 5: Storage Organization Issues

**Symptoms:**
- Backups in wrong directories
- Cannot find backups for specific server
- Confusing file organization

**Diagnosis:**
```bash
# Check current organization
find /backups -name "*.sql.gz" -type f | head -20

# Verify configuration
grep "backup_organization" config.yaml

# List backup structure
tree -d -L 3 /backups
```

**Solutions:**

1. **Choose appropriate organization:**
   ```yaml
   # For multi-server: organize by server
   backup_organization: "by-server"
   
   # For single server: organize by type
   backup_organization: "by-type"
   
   # For simple setups: flat structure
   backup_organization: "combined"
   ```

2. **Fix existing backups:**
   ```bash
   # Reorganize existing backups (example)
   cd /backups
   for file in *.sql.gz; do
     server=$(echo $file | cut -d'-' -f1)
     type=$(echo $file | cut -d'-' -f3)
     mkdir -p "$server/$type"
     mv "$file" "$server/$type/"
   done
   ```

### Issue 6: Database Filtering Not Working

**Symptoms:**
- Backing up unwanted databases
- Missing expected databases
- Include/exclude rules not applying

**Diagnosis:**
```bash
# List databases on server
mysql -h prod-db.example.com -u backup_user -p \
  -e "SHOW DATABASES"

# Check filter configuration
grep -A5 "include_databases\|exclude_databases" config.yaml

# Verify what's being backed up
ls -la /backups/prod-primary/daily/ | grep "database_name"
```

**Solutions:**

1. **Fix filter syntax:**
   ```yaml
   database_servers:
     - name: "prod-primary"
       include_databases:
         - "users"
         - "orders"
         - "products"  # Exact names, not patterns
       exclude_databases:
         - "test_*"    # Patterns work for exclude
   ```

2. **Check filter precedence:**
   ```yaml
   # Include is processed first, then exclude
   include_databases:
     - "analytics_*"  # Include all analytics DBs
   exclude_databases:
     - "analytics_temp"  # But exclude this one
   ```

3. **Verify permissions:**
   ```sql
   -- User needs SELECT permission on databases
   GRANT SELECT ON users.* TO 'backup_user'@'%';
   GRANT SELECT ON orders.* TO 'backup_user'@'%';
   ```

## Performance Optimization

### Slow Backup Performance

1. **Use read replica:**
   ```yaml
   backup_schedules:
     daily:
       servers:
         - "prod-replica"  # Use replica instead of primary
   ```

2. **Optimize mysqldump options:**
   ```yaml
   mysql_dump_options:
     single_transaction: true  # Consistent backup without locks
     quick: true              # Don't buffer query in memory
     compress: true           # Compress on the wire
     extended_insert: true    # Smaller output files
   ```

3. **Parallel backups for different servers:**
   ```yaml
   performance:
     max_concurrent_backups: 4  # Backup 4 servers at once
   ```

### Resource Utilization

1. **Monitor during backups:**
   ```bash
   # CPU and memory usage
   vmstat 5 10
   
   # Disk I/O
   iostat -x 5 10
   
   # Network usage
   iftop -i eth0
   ```

2. **Limit resource usage:**
   ```yaml
   performance:
     compression_level: 6  # Balance between size and CPU
     nice_priority: 10     # Lower CPU priority
     ionice_class: 3       # Idle I/O priority
   ```

## Monitoring and Alerts

### Set Up Monitoring

1. **Prometheus metrics:**
   ```yaml
   # prometheus.yml
   scrape_configs:
     - job_name: 'gosqlguard'
       static_configs:
         - targets: ['localhost:9090']
       metric_relabel_configs:
         - source_labels: [server_name]
           target_label: database_server
   ```

2. **Alert rules:**
   ```yaml
   # alerts.yml
   groups:
     - name: backup_alerts
       rules:
         - alert: BackupFailed
           expr: backup_status{status="error"} > 0
           for: 5m
           labels:
             severity: critical
           annotations:
             summary: "Backup failed for {{ $labels.server_name }}"
             
         - alert: BackupDelayed
           expr: time() - backup_last_success > 86400
           for: 1h
           labels:
             severity: warning
           annotations:
             summary: "No successful backup for {{ $labels.server_name }} in 24h"
   ```

### Health Checks

```bash
# Create health check script
cat > /usr/local/bin/check_gosqlguard.sh << 'EOF'
#!/bin/bash

# Check if service is running
if ! systemctl is-active --quiet gosqlguard; then
  echo "CRITICAL: GoSQLGuard service not running"
  exit 2
fi

# Check last backup time for each server
for server in prod-primary prod-replica staging; do
  last_backup=$(curl -s "http://localhost:8080/api/backups?server=$server&limit=1" | \
    jq -r '.data[0].created_at // empty')
  
  if [ -z "$last_backup" ]; then
    echo "WARNING: No backups found for $server"
    exit 1
  fi
  
  # Check if backup is recent (within 25 hours for daily)
  backup_age=$(( $(date +%s) - $(date -d "$last_backup" +%s) ))
  if [ $backup_age -gt 90000 ]; then  # 25 hours
    echo "WARNING: Last backup for $server is older than 25 hours"
    exit 1
  fi
done

echo "OK: All servers have recent backups"
exit 0
EOF

chmod +x /usr/local/bin/check_gosqlguard.sh
```

## Emergency Procedures

### Backup Failure Recovery

1. **Immediate actions:**
   ```bash
   # Check what failed
   tail -100 /var/log/gosqlguard.log | grep ERROR
   
   # Run manual backup
   curl -X POST http://localhost:8080/api/backups/run \
     -H "Content-Type: application/json" \
     -d '{"server":"prod-primary","type":"manual","database":"critical_db"}'
   ```

2. **Fallback to manual mysqldump:**
   ```bash
   # Direct mysqldump if GoSQLGuard fails
   mysqldump -h prod-db.example.com \
     -u backup_user -p \
     --single-transaction \
     --routines --triggers --events \
     critical_db | gzip > /emergency/critical_db-$(date +%Y%m%d-%H%M%S).sql.gz
   ```

### Disaster Recovery

1. **Switch to replica:**
   ```yaml
   # Temporarily update config to use replica
   database_servers:
     - name: "prod-primary"
       host: "prod-replica.example.com"  # Point to replica
   ```

2. **Cross-region failover:**
   ```bash
   # Copy backups to DR region
   aws s3 sync s3://backups-us-east/ s3://backups-us-west/ \
     --source-region us-east-1 \
     --region us-west-2
   ```

## Getting Help

1. **Enable debug logging:**
   ```yaml
   logging:
     level: "debug"
   ```

2. **Collect diagnostic info:**
   ```bash
   # Create diagnostic bundle
   mkdir gosqlguard-diag
   cd gosqlguard-diag
   
   # Collect configs (remove passwords!)
   cp /etc/gosqlguard/config.yaml .
   sed -i 's/password:.*/password: REDACTED/g' config.yaml
   
   # Recent logs
   tail -1000 /var/log/gosqlguard.log > recent.log
   
   # Server status
   curl http://localhost:8080/api/servers > servers.json
   curl http://localhost:8080/api/stats > stats.json
   
   # System info
   uname -a > system.txt
   free -m >> system.txt
   df -h >> system.txt
   
   # Create archive
   tar czf gosqlguard-diag.tar.gz .
   ```

3. **Report issues:**
   - Include diagnostic bundle
   - Describe the issue and when it started
   - List any recent changes
   - Provide example server configurations