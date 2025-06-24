# API Testing Guide

This guide covers testing for all GoSQLGuard API endpoints.

## Running Tests

### Run All API Tests
```bash
./scripts/test-api-endpoints.sh
```

### Run Tests with Coverage
```bash
./scripts/test-api-endpoints.sh --coverage
```

### Run Specific Test Packages
```bash
# Test S3 configuration API
go test -v ./pkg/api -run TestS3

# Test MySQL options API
go test -v ./pkg/api -run TestMySQLOptions

# Test PostgreSQL options API
go test -v ./pkg/api -run TestPostgreSQLOptions

# Test server connection API
go test -v ./pkg/api -run TestServerHandler

# Test schedule API
go test -v ./pkg/api -run TestScheduleHandler

# Test backup triggering
go test -v ./pkg/adminserver -run TestServer_RunBackup

# Run integration tests
go test -tags=integration -v ./pkg/api
```

## Test Coverage

The test suite covers:

### S3 Configuration API (`pkg/api/s3_test.go`)
- ✅ GET /api/s3 - Retrieve current S3 configuration
- ✅ PUT /api/s3 - Update S3 configuration
- ✅ POST /api/s3/test - Test S3 connection
- ✅ Invalid method handling
- ✅ Invalid JSON handling

### MySQL Options API (`pkg/api/mysql_options_test.go`)
- ✅ GET /api/mysql-options - Get global and per-server options
- ✅ PUT /api/mysql-options - Update global options
- ✅ GET /api/mysql-options/server - Get server-specific options
- ✅ PUT /api/mysql-options/server - Update server-specific options
- ✅ DELETE /api/mysql-options/server - Remove server-specific options
- ✅ Server not found errors
- ✅ Non-MySQL server errors
- ✅ Missing server name validation

### PostgreSQL Options API (`pkg/api/postgresql_options_test.go`)
- ✅ GET /api/postgresql-options - Get global and per-server options
- ✅ PUT /api/postgresql-options - Update global options
- ✅ GET /api/postgresql-options/server - Get server-specific options
- ✅ PUT /api/postgresql-options/server - Update server-specific options
- ✅ DELETE /api/postgresql-options/server - Remove server-specific options
- ✅ Format validation (plain, custom, directory, tar)
- ✅ Compression level validation (0-9)
- ✅ Non-PostgreSQL server errors

### Server Connection API (`pkg/api/server_test.go`)
- ✅ POST /api/servers/test - Test MySQL connections
- ✅ POST /api/servers/test - Test PostgreSQL connections
- ✅ Default port handling (3306 for MySQL, 5432 for PostgreSQL)
- ✅ Invalid database type errors
- ✅ Invalid method errors
- ✅ Invalid JSON handling

### Schedule API (`pkg/api/schedule_test.go`)
- ✅ GET /api/schedules - List all schedules
- ✅ GET /api/schedules?id=X - Get specific schedule
- ✅ POST /api/schedules - Create new schedule
- ✅ POST /api/schedules - Update existing schedule
- ✅ POST /api/schedules/delete - Delete schedule
- ✅ Validation errors (missing fields)
- ✅ Retention policy configuration

### Backup & Retention API (`pkg/adminserver/adminserver_test.go`)
- ✅ POST /api/backups/run - Trigger manual backup
- ✅ Backup type validation
- ✅ Server validation
- ✅ Database filtering
- ✅ Concurrent backup prevention
- ✅ POST /api/retention/run - Trigger retention policy
- ✅ GET /healthz - Health check endpoint

## Mock Objects

The tests use mock implementations for:
- `MockServerRepository` - Simulates database operations for servers
- `MockScheduleRepository` - Simulates database operations for schedules
- `MockScheduler` - Simulates the backup scheduler

## Integration Tests

The `integration_test.go` file demonstrates complete workflows:
1. Configuring S3 storage
2. Setting global MySQL options
3. Setting global PostgreSQL options
4. Testing S3 connection
5. Configuring per-server options

## Test Data Examples

### S3 Configuration
```json
{
  "enabled": true,
  "region": "us-east-1",
  "bucket": "backup-bucket",
  "prefix": "backups",
  "endpoint": "https://minio.local",
  "access_key_id": "access-key",
  "secret_access_key": "secret-key",
  "use_ssl": true,
  "insecure_ssl": false
}
```

### MySQL Options
```json
{
  "global": {
    "single_transaction": true,
    "quick": true,
    "skip_lock_tables": true,
    "extended_insert": true,
    "compress": true,
    "triggers": true,
    "routines": true,
    "events": true
  }
}
```

### PostgreSQL Options
```json
{
  "global": {
    "format": "custom",
    "verbose": false,
    "no_owner": true,
    "no_privileges": true,
    "jobs": 4,
    "compress": 6
  }
}
```

### Schedule Creation
```json
{
  "name": "Daily Backup",
  "backupType": "daily",
  "cronExpression": "0 2 * * *",
  "enabled": true,
  "localStorage": {
    "enabled": true,
    "duration": "168h",
    "keepForever": false
  },
  "s3Storage": {
    "enabled": true,
    "duration": "720h",
    "keepForever": false
  }
}
```

## Error Handling Tests

All endpoints are tested for:
- Invalid HTTP methods
- Missing required parameters
- Invalid JSON payloads
- Resource not found errors
- Type mismatches (e.g., PostgreSQL options on MySQL server)

## Performance Considerations

- Tests use in-memory mocks to avoid database dependencies
- Each test is isolated with its own configuration
- Concurrent backup tests verify mutex behavior

## Extending Tests

To add new test cases:
1. Add test functions to the appropriate `*_test.go` file
2. Use the existing mock objects or create new ones
3. Follow the naming convention `TestHandlerName_ScenarioDescription`
4. Include both positive and negative test cases
5. Test all error conditions