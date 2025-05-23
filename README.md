# GoSQLGuard

A robust Go-based MySQL backup management tool designed for Kubernetes deployments with flexible scheduling, retention policies, and multi-destination storage support.

![GoSQLGuard Logo](https://example.com/gosqlguard-logo.png)

## Overview

GoSQLGuard is a lightweight yet powerful solution to automate MySQL database backups in Kubernetes environments. It combines flexible scheduling capabilities with sophisticated retention policies, allowing you to maintain database backups across multiple storage destinations with different lifecycle rules.

## Features

- **Flexible Backup Scheduling**: Configure custom schedules for different backup types using standard cron syntax
- **Dual Storage Support**: Store backups both locally (PVC) and in S3-compatible storage
- **Independent Retention Policies**: Configure different retention rules for each backup type and storage destination
- **MySQL Metadata Database**: Store backup metadata in MySQL for improved reliability and queryability
- **Kubernetes Native**: Designed to run as a Kubernetes pod with standard resource management
- **Prometheus Metrics**: Comprehensive metrics for monitoring backup operations
- **Configurable via YAML**: Simple YAML-based configuration
- **Multi-Database Support**: Back up multiple MySQL databases in a single deployment

## Installation

### Prerequisites

- Kubernetes cluster 1.19+
- kubectl configured to communicate with your cluster
- A MySQL server accessible from your Kubernetes cluster
- Storage class for PVC (if using local storage)
- S3 bucket or S3-compatible storage (if using S3 storage)

### Quick Start

1. Clone the repository:

```bash
git clone https://github.com/yourusername/gosqlguard.git
cd gosqlguard
```

2. Build the Docker image:

```bash
docker build -t your-registry/gosqlguard:latest .
docker push your-registry/gosqlguard:latest
```

3. Create your configuration:

```bash
kubectl create configmap gosqlguard-config --from-file=config.yaml=./examples/config.yaml
```

4. Create a secret for credentials:

```bash
kubectl create secret generic gosqlguard-credentials \
  --from-literal=MYSQL_PASSWORD=your-mysql-password \
  --from-literal=S3_ACCESS_KEY=your-s3-access-key \
  --from-literal=S3_SECRET_KEY=your-s3-secret-key
```

5. Deploy GoSQLGuard:

```bash
kubectl apply -f k8s/deployment.yaml
```

## Configuration

GoSQLGuard is configured using a YAML file. Here's an example configuration:

```yaml
# MySQL connection settings
mysql:
  host: "mysql-service"
  port: "3306"
  username: "backup-user"
  password: "${MYSQL_PASSWORD}"  # Will be replaced by environment variable
  databases:
    - "db1"
    - "db2"
    - "db3"

# Local backup settings
local:
  enabled: true
  backupDirectory: "/backups"

# S3 storage settings
s3:
  enabled: true
  bucket: "my-database-backups"
  region: "us-east-1"
  endpoint: ""  # Leave empty for AWS S3, set for S3-compatible storage
  accessKey: "${S3_ACCESS_KEY}"  # Will be replaced by environment variable
  secretKey: "${S3_SECRET_KEY}"  # Will be replaced by environment variable
  prefix: "mysql/prod"
  useSSL: true

# Metrics configuration
metrics:
  port: "8080"

# Backup type configuration
backupTypes:
  hourly:
    schedule: "0 * * * *"  # Every hour at minute 0
    local:
      enabled: true
      retention:
        duration: "72h"
        forever: false
    s3:
      enabled: false  # Hourly backups only stored locally
      retention:
        duration: "0h" 
        forever: false
  daily:
    schedule: "0 0 * * *"  # Every day at midnight
    local:
      enabled: true
      retention:
        duration: "720h"  # 30 days
        forever: false
    s3:
      enabled: true
      retention:
        duration: "2160h"  # 90 days
        forever: false
  weekly:
    schedule: "0 0 * * 0"  # Every Sunday at midnight
    local:
      enabled: true
      retention:
        duration: "2016h"  # 12 weeks
        forever: false
    s3:
      enabled: true
      retention:
        duration: "4320h"  # 6 months
        forever: false
  monthly:
    schedule: "0 0 1 * *"  # First day of month at midnight
    local:
      enabled: true
      retention:
        duration: "8760h"  # 1 year
        forever: false
    s3:
      enabled: true
      retention:
        duration: "17520h"  # 2 years
        forever: false
  yearly:
    schedule: "0 0 1 1 *"  # January 1st at midnight
    local:
      enabled: true
      retention:
        duration: "0h"
        forever: true  # Keep forever locally
    s3:
      enabled: true
      retention:
        duration: "0h"
        forever: true  # Keep forever in S3
```

### Configuration Options

#### MySQL Settings
- `host`: MySQL server hostname
- `port`: MySQL server port
- `username`: MySQL username with backup privileges
- `password`: MySQL password (can use environment variable syntax)
- `databases`: List of databases to back up

#### Local Storage Settings
- `enabled`: Enable/disable local storage
- `backupDirectory`: Directory path for backups (mounted from PVC)

#### S3 Storage Settings
- `enabled`: Enable/disable S3 storage
- `bucket`: S3 bucket name
- `region`: AWS region or S3-compatible region
- `endpoint`: Custom endpoint for S3-compatible storage (leave empty for AWS S3)
- `accessKey`: S3 access key (can use environment variable syntax)
- `secretKey`: S3 secret key (can use environment variable syntax)
- `prefix`: Prefix for S3 objects (useful for organizing backups)
- `useSSL`: Whether to use SSL for S3 connections

#### Metadata Database Settings
- `enabled`: Enable/disable MySQL metadata database storage
- `host`: MySQL server hostname
- `port`: MySQL server port
- `username`: MySQL username with appropriate privileges
- `password`: MySQL password (can use environment variable syntax)
- `database`: Database name to use for metadata storage
- `maxOpenConns`: Maximum number of open connections to the database
- `maxIdleConns`: Maximum number of idle connections in the connection pool
- `connMaxLifetime`: Maximum amount of time a connection may be reused
- `autoMigrate`: Whether to automatically create/update database tables

#### Backup Types
For each backup type (hourly, daily, weekly, etc.):
- `schedule`: Cron expression for the backup schedule
- `local.enabled`: Enable/disable local storage for this backup type
- `local.retention.duration`: How long to keep backups (Go duration format)
- `local.retention.forever`: Whether to keep backups forever
- `s3.enabled`: Enable/disable S3 storage for this backup type
- `s3.retention.duration`: How long to keep backups in S3
- `s3.retention.forever`: Whether to keep S3 backups forever

## Kubernetes Deployment

Here's an example of a Kubernetes deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gosqlguard
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gosqlguard
  template:
    metadata:
      labels:
        app: gosqlguard
    spec:
      containers:
      - name: gosqlguard
        image: your-registry/gosqlguard:latest
        env:
        - name: CONFIG_PATH
          value: "/app/config/config.yaml"
        - name: MYSQL_PASSWORD
          valueFrom:
            secretKeyRef:
              name: gosqlguard-credentials
              key: MYSQL_PASSWORD
        - name: S3_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: gosqlguard-credentials
              key: S3_ACCESS_KEY
        - name: S3_SECRET_KEY
          valueFrom:
            secretKeyRef:
              name: gosqlguard-credentials
              key: S3_SECRET_KEY
        volumeMounts:
        - name: backup-storage
          mountPath: /backups
        - name: config-volume
          mountPath: /app/config
        ports:
        - containerPort: 8080
          name: metrics
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: metrics
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: metrics
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: backup-storage
        persistentVolumeClaim:
          claimName: gosqlguard-storage
      - name: config-volume
        configMap:
          name: gosqlguard-config
```

And the corresponding PVC:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: gosqlguard-storage
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 100Gi
```

## Monitoring

GoSQLGuard exposes Prometheus metrics on the specified port (default: 8080). These metrics include:

- `mysql_backup_total`: Counter of total backups performed (with status)
- `mysql_backup_duration_seconds`: Histogram of backup durations
- `mysql_backup_size_bytes`: Gauge of backup sizes
- `mysql_backup_deletions_total`: Counter of backups deleted by retention policy
- `mysql_backup_last_timestamp`: Timestamp of the last successful backup
- `mysql_backup_s3_upload_total`: Counter of S3 uploads
- `mysql_backup_s3_upload_duration_seconds`: Histogram of S3 upload durations

You can use these metrics to set up Grafana dashboards and Prometheus alerts.

Example Prometheus scrape config:

```yaml
scrape_configs:
  - job_name: 'gosqlguard'
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: gosqlguard
      - source_labels: [__address__]
        action: replace
        target_label: __address__
        regex: (.+):(\d+)
        replacement: ${1}:8080
```

## Common Usage Scenarios

### Development Environment
```yaml
backupTypes:
  hourly:
    schedule: "0 */4 * * *"  # Every 4 hours
    local:
      enabled: true
      retention:
        duration: "24h"
        forever: false
    s3:
      enabled: false
```

### Production Environment
```yaml
backupTypes:
  hourly:
    schedule: "0 * * * *"  # Every hour
    local:
      enabled: true
      retention:
        duration: "72h"
        forever: false
    s3:
      enabled: false
  daily:
    schedule: "0 0 * * *"  # Every day at midnight
    local:
      enabled: true
      retention:
        duration: "720h"  # 30 days
        forever: false
    s3:
      enabled: true
      retention:
        duration: "8760h"  # 1 year
        forever: false
```

### Compliance Environment
```yaml
backupTypes:
  hourly:
    schedule: "0 * * * *"  # Every hour
    local:
      enabled: true
      retention:
        duration: "168h"  # 7 days
        forever: false
    s3:
      enabled: true
      retention:
        duration: "8760h"  # 1 year
        forever: false
  daily:
    schedule: "0 0 * * *"  # Daily at midnight
    local:
      enabled: true
      retention:
        duration: "720h"  # 30 days
        forever: false
    s3:
      enabled: true
      retention:
        duration: "87600h"  # ~10 years
        forever: false
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch: `git checkout -b feature/my-new-feature`
3. Commit your changes: `git commit -am 'Add some feature'`
4. Push to the branch: `git push origin feature/my-new-feature`
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- The Go community for providing excellent libraries
- The Kubernetes community for setting standards for cloud-native applications

## Support

If you encounter any issues or have questions, please open an issue on GitHub.
