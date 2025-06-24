# MySQL Configuration Storage

GoSQLGuard now supports storing configuration in a MySQL database sidecar instead of YAML files. This provides dynamic configuration management and better integration with Kubernetes environments.

## Overview

When using MySQL configuration storage:
- All configuration is stored in a MySQL database running as a sidecar container
- Configuration can be updated dynamically via API endpoints
- Changes are automatically detected and applied without restarting GoSQLGuard
- Configuration is versioned for audit trail

## Architecture

```
┌─────────────────┐     ┌──────────────────┐
│                 │     │                  │
│   GoSQLGuard    │────▶│  MySQL Sidecar   │
│                 │     │  (Config DB)     │
└─────────────────┘     └──────────────────┘
        │                        │
        │                        │
        ▼                        ▼
┌─────────────────┐     ┌──────────────────┐
│  Target DBs     │     │  Configuration   │
│  (MySQL/PG)     │     │  Admin UI        │
└─────────────────┘     └──────────────────┘
```

## Deployment

### Kubernetes

Deploy GoSQLGuard with MySQL sidecar:

```bash
kubectl apply -f deployments/mysql-sidecar/deployment.yaml
```

This creates:
- GoSQLGuard pod with MySQL sidecar container
- ConfigMap with database initialization scripts
- Secret for MySQL passwords
- PersistentVolumeClaim for MySQL data
- Service for accessing GoSQLGuard

### Docker Compose (Development)

For local development with MySQL configuration:

```bash
docker-compose -f docker-compose.mysql-config.yml up -d
```

This starts:
- GoSQLGuard with MySQL configuration enabled
- MySQL configuration database
- Target MySQL and PostgreSQL databases
- MinIO for S3 storage
- phpMyAdmin for configuration management

Access:
- GoSQLGuard UI: http://localhost:8889
- phpMyAdmin: http://localhost:8890 (user: gosqlguard, password: config_password)

## Configuration Schema

### Tables

1. **global_config** - Key-value configuration settings
2. **database_servers** - Database server definitions
3. **database_filters** - Include/exclude database rules
4. **storage_configs** - Storage backend configurations
5. **backup_schedules** - Backup schedule definitions
6. **retention_policies** - Retention rules per schedule/storage
7. **mysql_dump_options** - MySQL dump command options
8. **config_history** - Audit trail of changes
9. **config_versions** - Configuration versioning

### Example: Adding a Database Server

Using the stored procedure:
```sql
CALL add_database_server(
    'production-mysql',
    'mysql',
    'mysql.prod.svc.cluster.local',
    3306,
    'backup_user',
    'secure_password',
    JSON_ARRAY('app_db', 'users_db')
);
```

Using the API:
```bash
curl -X POST http://localhost:8889/api/config/servers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "production-mysql",
    "type": "mysql",
    "host": "mysql.prod.svc.cluster.local",
    "port": 3306,
    "username": "backup_user",
    "password": "secure_password",
    "include_databases": ["app_db", "users_db"]
  }'
```

## API Endpoints

### Configuration Management

- `GET /api/config` - Get complete configuration
- `GET /api/config/servers` - List all database servers
- `POST /api/config/servers` - Add a new database server
- `PUT /api/config/servers?id=X` - Update a database server
- `DELETE /api/config/servers?id=X` - Delete a database server
- `GET /api/config/storage` - List storage configurations
- `PUT /api/config/storage?id=X` - Update storage configuration
- `GET /api/config/schedules` - List backup schedules
- `PUT /api/config/schedules?id=X` - Update schedule
- `POST /api/config/reload` - Trigger configuration reload

## Environment Variables

To enable MySQL configuration storage:

```bash
CONFIG_SOURCE=mysql
CONFIG_MYSQL_HOST=localhost
CONFIG_MYSQL_PORT=3306
CONFIG_MYSQL_DATABASE=gosqlguard_config
CONFIG_MYSQL_USER=gosqlguard
CONFIG_MYSQL_PASSWORD=your_password
```

## Migration from YAML

To migrate existing YAML configuration to MySQL:

1. Start MySQL configuration database
2. Load the schema from `deployments/mysql-sidecar/init/01-config-schema.sql`
3. Convert YAML settings to SQL inserts
4. Set `CONFIG_SOURCE=mysql` and restart GoSQLGuard

## Benefits

1. **Dynamic Updates** - Change configuration without restarts
2. **Multi-tenancy** - Easy to manage multiple GoSQLGuard instances
3. **Audit Trail** - All changes are tracked in config_history
4. **API Access** - Programmatic configuration management
5. **Version Control** - Configuration versioning built-in
6. **Validation** - Database constraints ensure valid configuration

## Security Considerations

1. **Network Isolation** - Keep MySQL sidecar on localhost only
2. **Authentication** - Use strong passwords for MySQL access
3. **Encryption** - Enable TLS for MySQL connections in production
4. **Secrets** - Store sensitive data (passwords) in Kubernetes secrets
5. **RBAC** - Limit access to configuration API endpoints

## Troubleshooting

### GoSQLGuard can't connect to MySQL config

Check:
1. MySQL sidecar is running: `kubectl logs <pod> -c config-mysql`
2. Environment variables are set correctly
3. MySQL user has proper permissions
4. Network connectivity between containers

### Configuration changes not applying

1. Check config version: `SELECT * FROM config_versions WHERE active = TRUE`
2. Verify GoSQLGuard is watching for changes (check logs)
3. Manually trigger reload: `POST /api/config/reload`

### Performance issues

1. Ensure MySQL has adequate resources
2. Check indexes are created (they should be from schema)
3. Monitor slow query log
4. Consider connection pooling settings