# GoSQLGuard Project Brief

## Project Overview
GoSQLGuard is a MySQL backup management system written in Go that automates and monitors the backup process for MySQL databases. It provides a reliable way to schedule, execute, store, and monitor database backups with support for multiple storage options including local filesystem and S3-compatible storage.

## Core Requirements

### Backup Functionality
- Schedule and execute database backups (MySQL and PostgreSQL) on configurable intervals
- Support for different backup types (hourly, daily, weekly) with different retention policies
- Backup multiple databases with a single configuration
- Compress backups to minimize storage requirements
- Ensure transactional consistency of backups with proper database flags

### Storage Options
- Local filesystem storage for backups
- S3-compatible storage support (AWS S3, MinIO, etc.)
- Configurable retention policies for each storage type and backup type
- Automatic cleanup of expired backups according to retention policies

### Monitoring & Reporting
- Track backup success/failure status
- Measure backup size and duration
- Expose Prometheus metrics for monitoring
- Maintain metadata about backups for reporting

### Administration
- Web-based admin interface for monitoring backup status
- API endpoints for triggering backups and viewing status
- Configuration via YAML files
- Graceful shutdown and recovery

### Metadata System
- Store metadata about backups for tracking and reporting
- Track backup status, size, location, and other relevant details
- Support searching and filtering of backup records
- Maintain metadata persistence across restarts

## Technical Constraints
- Written in Go for performance and cross-platform compatibility
- Minimal external dependencies to ensure reliability
- Container-friendly design for easy deployment
- Configuration via environment variables and/or config files
- Support for running in Kubernetes environments

## Security Considerations
- Secure storage of database credentials
- Support for encrypted backups
- Proper error handling to prevent information leakage
- Secure API endpoints with authentication when needed

## Target Users
- Database administrators responsible for MySQL/PostgreSQL backup management
- DevOps engineers setting up database backup infrastructure
- System administrators of applications using popular open-source databases
