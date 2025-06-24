# Active Context

## Planned React Frontend Integration

The GoSQLGuard application is planned to be enhanced with a modern React frontend while maintaining the Go backend as an API service. This significant architectural change will:

- Replace the current server-side rendered HTML interface with a React/Next.js frontend
- Transform the Go backend to provide RESTful API endpoints
- Migrate from file-based metadata to a MariaDB database
- Implement Auth0 for authentication
- Add Stripe for subscription-based premium features
- Deploy using Kubernetes, Helm, and ArgoCD

The implementation will begin with a Proof of Concept (POC) phase to validate the approach before proceeding with full implementation. Detailed tasks have been organized in the `frontend-tasks.md` file, with the initial focus on:

- Setting up a Next.js project with TypeScript
- Converting key GoSQLGuard endpoints to JSON APIs
- Implementing Auth0 authentication
- Creating a basic dashboard UI for backup status visualization

## Recent Changes

### Database-Driven Configuration System

As of version 0.1.0-rc10, GoSQLGuard has transitioned to a fully database-driven configuration system:

- **No Config Files**: The application no longer uses config.yaml files
- **ConfigMap Removed**: Kubernetes deployments no longer need ConfigMaps
- **Environment Variables**: Basic settings come from environment variables
- **MySQL Metadata Database**: All dynamic configuration is stored in the database:
  - Server definitions with credentials and options
  - Backup schedules with cron expressions
  - Backup types and retention policies
  - MySQL dump options per backup type
- **Dynamic Reloading**: Configuration changes take effect immediately without restart
- **UI-Based Management**: All configuration can be managed through the web interface

### Schedule Management Through UI

The admin interface now includes comprehensive schedule management:

- View and edit backup schedules directly in the UI
- Enable/disable schedules without restarting the application
- Validate cron expressions before saving
- Changes are immediately picked up by the scheduler
- Fixed hourly backup schedule to run at the top of each hour (0 * * * *)

## Recent Changes

### Multi-Server Backup with Manual Backup Type

The GoSQLGuard application now supports a more flexible manual backup system with these enhancements:

- Added a dedicated "manual" backup type with longer retention periods (90 days for local, 365 days for S3)
- Implemented multi-server selection in the backup form to backup multiple servers at once
- Added support for selecting multiple databases across servers
- Enhanced the backend to process comma-separated lists of servers and databases
- Modified the scheduler to filter backups by server and database
- Updated the UI to properly display grouped dropdown options
- Improved logging with clear identification of targeted servers and databases

### UI Enhancements for Multi-Server Support

The GoSQLGuard admin interface now includes comprehensive multi-server support:

- Added server selection dropdown to the backup form
- Added server filter on the backup status page
- Added server column to the backup listing table
- Implemented server filtering in the API endpoint
- Automatic collection of server names from both configuration and backup history

These changes allow users to easily filter and manage backups across multiple database servers from a unified interface.

### Improved MySQL 8.0+ Compatibility with Ubuntu-based Image

The GoSQLGuard Docker image has been updated to use Ubuntu instead of Alpine Linux, providing native support for MySQL 8.0+ authentication:

- Switched from `alpine:3.19` to `ubuntu:22.04` as the base runtime image
- Replaced Alpine's MySQL client with `mysql-client-core-8.0` package
- Removed the need for workarounds with authentication plugin symlinks
- Provided better compatibility with modern MySQL servers and authentication methods

### Added MySQL Authentication Plugin Support

The GoSQLGuard application now supports specifying MySQL authentication plugins for servers, particularly useful for MySQL 8.0+ servers that use the `caching_sha2_password` authentication plugin by default. Key changes include:

- Added `authPlugin` field to the database server configuration
- Updated the MySQL provider to use `--default-auth` when an authentication plugin is specified
- Configured the MySQL client to handle different authentication methods

This enhancement allows GoSQLGuard to properly connect to MySQL 8.0+ servers which use a different authentication plugin than older MySQL versions.

### Added Multi-Server Backup Support

The GoSQLGuard application now supports backing up multiple database servers through a single configuration. This feature allows:

- Configuring multiple MySQL servers (with different credentials, host/port settings)
- Organizing backups in a flexible folder structure (by server, by type, or both)
- Separate include/exclude database lists per server
- Server-specific MySQL dump options

#### Implementation Details

1. **Configuration Structure**:
   - Added `database_servers` array in the config to specify multiple servers
   - Added organization strategy options for both Local and S3 storage: `combined`, `server-only`, `type-only`
   - Created a clean, focused multi-server architecture

2. **Backup Organization**:
   - The `combined` strategy (default) creates two copies of each backup:
     - `by-server/<server-name>/<backup-type>/<database>-<timestamp>.sql.gz`
     - `by-type/<backup-type>/<server-name>_<database>-<timestamp>.sql.gz`
   - The `server-only` strategy organizes backups only by server name
   - The `type-only` strategy organizes backups only by backup type

3. **Metadata Updates**:
   - Added server name and server type to backup metadata
   - Updated storage paths to support multiple organization methods
   - Created a unified metadata model for all server types

4. **Example Configuration**:
   - Created `example-multi-server-config.yaml` to demonstrate configuration options
   - Shows how to set up multiple servers with different settings
   - Illustrates backup organization strategies

## Current Tasks

- Stabilize the database-driven configuration system
- Enhance error handling for database connectivity issues
- Begin work on React frontend Proof of Concept
- Consider adding PostgreSQL support for additional database types
- Improve UI error handling and edge cases

## Next Steps

- Implement backup verification features
- Add PostgreSQL database support
- Develop detailed implementation plan for React frontend
- Enhance reporting and analytics features
- Add authentication system for the admin UI

## Technical Decisions

### React Frontend Integration
1. We've chosen Next.js as the React framework for its server-side rendering capabilities and robust routing
2. Auth0 will be used for authentication to provide secure, scalable user management
3. MariaDB will replace the current file-based metadata storage for better scalability and query capabilities
4. A subscription model with tiered pricing will be implemented using Stripe
5. The existing Go backend will be maintained but transformed into a RESTful API service

### Multi-Server Implementation
The multi-server implementation takes a combined storage approach by default, which:
1. Preserves server-specific organization for when you need to focus on a single server
2. Provides type-specific organization when you need to compare backups across servers
3. Uses filepath isolation to prevent any filename collisions between servers

### MySQL Authentication
For the MySQL authentication plugin support:
1. We opted for an explicit `authPlugin` configuration rather than auto-detection, giving users the ability to control this behavior explicitly
2. We've made it an optional setting to maintain backward compatibility
3. The Docker image now uses Ubuntu 22.04 with mysql-client-core-8.0 for native MySQL 8.0+ support
4. The MySQL client supports the `--default-auth` parameter for specifying the authentication plugin

### Configuration Management
The transition to database-driven configuration was made to:
1. Eliminate the complexity of managing config files in containers
2. Enable dynamic configuration changes without restarts
3. Provide a better user experience with UI-based configuration
4. Simplify Kubernetes deployments by removing ConfigMaps
5. Centralize all configuration in a single source of truth
