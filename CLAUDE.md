# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GoSQLGuard is a Go-based database backup management tool designed for Kubernetes environments. It supports MySQL and PostgreSQL databases with flexible scheduling, retention policies, and multi-destination storage (local filesystem and S3).

## Architecture

The codebase follows a modular architecture with clear separation of concerns:

- **Entry Point**: `main.go` initializes configuration, starts the scheduler, and launches the admin server
- **Core Components**:
  - `pkg/backup/`: Database backup logic with provider-based architecture for MySQL/PostgreSQL
  - `pkg/scheduler/`: Cron-based scheduling system for automated backups
  - `pkg/storage/`: Storage providers (local filesystem and S3)
  - `pkg/metadata/`: Metadata tracking (file-based or MySQL-backed)
  - `pkg/adminserver/`: Web UI and API endpoints
  - `pkg/config/`: Configuration parsing with environment variable substitution

## Development Commands

### Build and Release
```bash
make build          # Build Docker image with auto-incrementing RC version
make push           # Push to registry
make release        # Build and push (full release)
make increment-rc   # Just increment RC number
make help           # View current version info
```

### Local Development
```bash
# Start development environment
docker-compose up -d

# Run all tests
./scripts/run-tests.sh

# Run specific database tests
./scripts/run-mysql-tests.sh
./scripts/run-postgres-tests.sh

# Access services
# Admin UI: http://localhost:8888
# MinIO Console: http://localhost:9001 (minioadmin/minioadmin)
```

### Testing
The project uses Go's built-in testing framework. Integration tests are located in `pkg/test/integration/` and require a running database instance.

## Configuration Patterns

The application uses YAML configuration with environment variable substitution (`${VAR}` syntax). Key configuration sections:

- `mysql`/`postgresql`: Database connection settings
- `local`: Local storage configuration
- `s3`: S3-compatible storage configuration
- `backupTypes`: Defines backup schedules and retention policies
- `metadata_database`: Optional MySQL-based metadata storage

## API Design

The admin server exposes:
- Web UI at `/` with server-side rendered HTML templates
- API endpoints under `/api/` for backup operations
- Prometheus metrics at `/metrics`
- Health check at `/health`

## Key Development Patterns

1. **Provider Pattern**: Database operations use a provider interface allowing easy extension for new database types
2. **Configuration Validation**: All configuration is validated at startup with clear error messages
3. **Error Handling**: Consistent error wrapping with context for debugging
4. **Metrics**: All operations emit Prometheus metrics for monitoring
5. **Logging**: Structured logging with configurable debug mode

## Important Files

- `pkg/config/config.go`: Central configuration structure and validation
- `pkg/backup/backup.go`: Core backup orchestration logic
- `pkg/scheduler/scheduler.go`: Cron job management
- `pkg/pages/*.go`: HTML template rendering for admin UI
- `example-configs/`: Reference configurations for common scenarios