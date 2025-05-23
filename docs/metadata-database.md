# MySQL Metadata Database

GoSQLGuard now supports storing backup metadata in a MySQL database instead of a local JSON file. This provides better reliability, concurrent access, and the ability to query historical backup data.

## Configuration

To enable the MySQL metadata database, add the following to your configuration file:

```yaml
metadata_database:
  enabled: true
  host: "localhost"
  port: 3306
  username: "gosqlguard"
  password: "your_password"
  database: "gosqlguard_metadata"
  maxOpenConns: 10
  maxIdleConns: 5
  connMaxLifetime: "5m"
  autoMigrate: true  # Automatically create/update tables
```

Alternatively, you can use environment variables:

```
METADATA_DB_ENABLED=true
METADATA_DB_HOST=localhost
METADATA_DB_PORT=3306
METADATA_DB_USERNAME=gosqlguard
METADATA_DB_PASSWORD=your_password
METADATA_DB_DATABASE=gosqlguard_metadata
METADATA_DB_MAX_OPEN_CONNS=10
METADATA_DB_MAX_IDLE_CONNS=5
METADATA_DB_CONN_MAX_LIFETIME=5m
METADATA_DB_AUTO_MIGRATE=true
```

## Database Schema

When `autoMigrate` is enabled, GoSQLGuard will automatically create the following tables:

1. `backups` - Main table for backup records
2. `local_paths` - Storage paths for local backups
3. `s3_keys` - Storage keys for S3 backups
4. `metadata_stats` - Global statistics and settings

The schema is designed to efficiently store and retrieve backup information while maintaining relationships between backups and their storage locations.

## Migrating from File-Based Storage

When you first enable the database, GoSQLGuard will attempt to migrate your existing metadata from the JSON file to the database. This migration happens automatically and preserves all your historical backup records.

## Fallback Mechanism

If the database becomes unavailable, GoSQLGuard will log an error and fall back to the file-based storage mechanism to ensure your backups continue to work.

## Example Configuration

See the complete example configuration at `example-configs/mysql-metadata-config.yaml`.

## MySQL Sidecar Container for Kubernetes

For Kubernetes deployments, you can use a MySQL sidecar container to store metadata without requiring an external database. This approach provides:

1. **Self-contained deployment**: No external database dependency
2. **Simplified setup**: Automatic database initialization
3. **Co-located storage**: Reduced network latency for metadata operations

A complete Kubernetes deployment manifest with a MySQL sidecar is available at `k8s/deployment-with-mysql-sidecar.yaml`.

### Sidecar Container Configuration

The MySQL sidecar uses:
- Official MySQL 8.0 image
- Persistent volume for data storage
- Init script for database and user setup
- Health checks to ensure database availability

### Required Kubernetes Resources

1. **PersistentVolumeClaim** for MySQL data
2. **ConfigMap** for initialization scripts
3. **Secret** for database credentials

### Secret Setup

```bash
kubectl create secret generic gosqlguard-credentials \
  --from-literal=MYSQL_PASSWORD=your-mysql-password \
  --from-literal=S3_ACCESS_KEY=your-s3-access-key \
  --from-literal=S3_SECRET_KEY=your-s3-secret-key \
  --from-literal=MYSQL_ROOT_PASSWORD=your-mysql-root-password \
  --from-literal=METADATA_DB_PASSWORD=your-metadata-db-password
```

## Database Maintenance

The metadata database is designed to be low-maintenance. It includes functionality to automatically purge deleted backup records based on your retention policies. However, standard database maintenance practices are recommended:

1. Regular backups of the metadata database
2. Monitoring of database size and performance
3. Occasional optimization of database tables

## Security Considerations

- Create a dedicated MySQL user with access only to the gosqlguard_metadata database
- Use strong passwords and consider using MySQL's encryption features
- If running in a production environment, configure TLS for the database connection
