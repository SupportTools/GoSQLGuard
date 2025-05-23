# GoSQLGuard Testing Guide

This document provides instructions for setting up and running tests for the GoSQLGuard project.

## Test Environment

The test environment consists of:

- MySQL database container
- PostgreSQL database container 
- MinIO S3-compatible storage container
- GoSQLGuard application container

All services are orchestrated using Docker Compose, making it easy to start and stop the entire test environment.

## Setting Up the Test Environment

### Prerequisites

- Docker and Docker Compose installed
- Go development environment set up
- cURL for API testing

### Quick Start

Run the provided test setup script:

```bash
./run-tests.sh
```

This script will:
1. Start all required Docker containers
2. Create and initialize test databases
3. Set up test data
4. Provide instructions for running the tests

## Test Files

The following test files and scripts are available:

- `mysql_test.go` - Integration tests for MySQL functionality
- `pkg/test/integration/postgresql/postgresql_test.go` - Integration tests for PostgreSQL functionality
- `setup-test-db.sh` - Script to set up MySQL test databases
- `setup-postgres-db.sh` - Script to set up PostgreSQL test databases
- `test_backup.sh` - Script to test backup functionality
- `run-tests.sh` - Main script to start the test environment

## Running Tests

### MySQL Tests

```bash
go test -v ./mysql_test.go
```

### PostgreSQL Tests

```bash
TEST_DB_TYPE=postgres go test -v ./pkg/test/integration/postgresql/...
```

### Backup Verification

```bash
./test_backup.sh
```

### Running All Tests

```bash
go test -v ./...
```

## Test Environment Details

### Database Connections

#### MySQL
- Host: localhost (or mysql-service within Docker network)
- Port: 3306
- User: backup-user
- Password: test-password
- Databases: db1, db2, db3

#### PostgreSQL
- Host: localhost (or postgres-service within Docker network)
- Port: 5432
- User: backup-user
- Password: test-password
- Databases: db1, db2, db3

### MinIO (S3-compatible storage)
- API Endpoint: http://localhost:9000
- Console: http://localhost:9001
- Access Key: minioadmin
- Secret Key: minioadmin
- Bucket: gosqlguard-backups

### GoSQLGuard Admin Interface
- URL: http://localhost:8888

## Test Data

The test environment is set up with the following:

- Three MySQL databases (db1, db2, db3), each with a test table and sample data
- Three PostgreSQL databases (db1, db2, db3), each with a test table and sample data
- MinIO bucket for storing backups

## Debugging

### Viewing Container Logs

```bash
# All container logs
docker-compose logs

# Specific container logs
docker-compose logs mysql-service
docker-compose logs postgres-service
docker-compose logs gosqlguard
```

### Accessing Databases Directly

```bash
# MySQL CLI
docker exec -it gosqlguard-mysql mysql -u backup-user -ptest-password

# PostgreSQL CLI
docker exec -it gosqlguard-postgres psql -U backup-user
```

### Checking Backup Files

```bash
# Local backup files
ls -la ./backups/hourly/

# MinIO backup files
docker exec gosqlguard-minio mc ls myminio/gosqlguard-backups
```

## Common Issues and Solutions

### Container Fails to Start

If a container fails to start, check the logs:

```bash
docker-compose logs [service-name]
```

### Connection Issues

If you're having trouble connecting to the databases:

1. Verify the containers are running: `docker-compose ps`
2. Check the healthcheck status: `docker inspect --format "{{.State.Health.Status}}" gosqlguard-mysql`
3. Test connectivity from within Docker: `docker exec gosqlguard-controller ping mysql-service`

### Test Failures

If tests are failing:

1. Make sure all containers are running and healthy
2. Check the logs for errors
3. Verify the test databases exist and contain data
4. Ensure the GoSQLGuard API is accessible

## Cleaning Up

Use the provided cleanup script to remove test containers, volumes, and backup files:

```bash
# Remove containers only (keep volumes and backup files)
./cleanup-tests.sh

# Remove containers and volumes
./cleanup-tests.sh -v

# Remove containers, volumes, and all backup files
./cleanup-tests.sh -v -b

# Remove everything without confirmation prompt
./cleanup-tests.sh -f -v -b
```

For help and additional options:

```bash
./cleanup-tests.sh -h
```

## Continuous Integration

When running in a CI environment, you can skip Docker-based tests by setting environment variables:

```bash
CI=true SKIP_DOCKER_TESTS=true go test ./...
