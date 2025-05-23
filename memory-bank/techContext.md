# Technical Context for GoSQLGuard

## Technologies Used

### Core Technologies
- **Go (Golang)**: Primary implementation language for backend
- **React**: Frontend library for building user interfaces
- **Next.js**: React framework for SSR and static site generation
- **TypeScript**: Typed JavaScript for frontend development
- **MySQL**: Initial target database system for backups
- **MariaDB**: Database for application data and metadata storage
- **PostgreSQL**: Planned database system to support for backups
- **HTML/CSS/JavaScript**: Current web UI with minimal client-side JavaScript
- **Bootstrap**: Current UI framework for responsive design
- **YAML**: Configuration format

### Authentication and Payment
- **Auth0**: Authentication and authorization platform
- **Stripe**: Payment processing for subscription management

### Storage Technologies
- **Local filesystem**: For local backup storage
- **S3-compatible storage**: AWS S3 and other compatible providers (MinIO, etc.)

### Deployment and DevOps
- **Docker**: Containerization of application components
- **Kubernetes**: Container orchestration platform
- **Helm**: Package manager for Kubernetes
- **ArgoCD**: GitOps continuous delivery tool
- **GitHub Actions**: CI/CD pipeline

### Libraries and Dependencies

#### Backend (Go)
- **Go Standard Library**:
  - `net/http`: Web server implementation
  - `encoding/json`: JSON handling
  - `html/template`: HTML templating
  - `time`: Time and duration handling
  - `os/exec`: External command execution (mysqldump)

- **Third-party Go packages**:
  - `github.com/robfig/cron`: Cron-based scheduling
  - `github.com/aws/aws-sdk-go-v2`: AWS S3 SDK
  - `github.com/prometheus/client_golang`: Prometheus metrics
  - `github.com/google/uuid`: UUID generation for backup IDs
  - `github.com/dustin/go-humanize`: Human-readable numbers and dates
  - `gorm.io/gorm`: ORM for database access (planned)
  - `gorm.io/driver/mysql`: MySQL driver for GORM (planned)

#### Frontend (Planned)
- **React**: UI library
- **Next.js**: React framework
- **TypeScript**: Type-safe JavaScript
- **SWR/React Query**: Data fetching and caching
- **Auth0 React SDK**: Authentication integration
- **Stripe React Components**: Payment processing UI
- **Tailwind CSS**: Utility-first CSS framework

### External Dependencies
- **mysql-client-core-8.0**: MySQL 8.0 client for database operations and backups with native authentication support
- **mysqldump**: External command for creating MySQL backups, part of mysql-client-core-8.0
- **pg_dump**: External command for creating PostgreSQL backups (planned)
- **gzip**: For backup compression

## Development Setup

### Current Local Development Environment
- Go 1.19+ installed locally
- Docker for running MySQL and S3 (MinIO) instances during development
- Make for build automation
- Access to a MySQL server for testing backup functionality

### Planned Development Environment Additions
- Node.js for frontend development
- npm/yarn for frontend package management
- Auth0 development tenant
- Stripe test environment
- Docker Compose for local multi-service development
- Kubernetes development cluster (e.g., minikube or kind)

### Current Project Structure
```
GoSQLGuard/
├── cmd/           # Command-line entry points
├── pkg/           # Core packages
│   ├── adminserver/    # Web UI and API server
│   ├── backup/         # Backup operations
│   ├── config/         # Configuration management
│   ├── metadata/       # Backup metadata tracking
│   ├── metrics/        # Prometheus metrics
│   ├── pages/          # Web UI HTML templates
│   ├── scheduler/      # Backup scheduling
│   └── storage/        # Storage providers
│       ├── local/      # Local filesystem storage
│       └── s3/         # S3-compatible storage
├── scripts/       # Helper scripts
├── test/          # Integration tests
├── .gitignore
├── go.mod         # Go module definition
├── go.sum         # Go dependencies checksum
├── LICENSE
├── Makefile       # Build automation
└── README.md
```

### Planned Project Structure
```
GoSQLGuard/
├── backend/       # Go backend API
│   ├── cmd/           # Command-line entry points
│   ├── pkg/           # Core packages
│   │   ├── api/            # API handlers and middleware
│   │   ├── backup/         # Backup operations
│   │   ├── config/         # Configuration management
│   │   ├── models/         # Data models
│   │   ├── repository/     # Data access layer
│   │   ├── metadata/       # Backup metadata tracking
│   │   ├── metrics/        # Prometheus metrics
│   │   ├── scheduler/      # Backup scheduling
│   │   └── storage/        # Storage providers
│   │       ├── local/      # Local filesystem storage
│   │       └── s3/         # S3-compatible storage
│   ├── scripts/       # Helper scripts
│   ├── test/          # Integration tests
│   ├── .gitignore
│   ├── go.mod         # Go module definition
│   └── go.sum         # Go dependencies checksum
├── frontend/      # React/Next.js frontend
│   ├── components/    # React components
│   ├── pages/         # Next.js pages
│   ├── services/      # API client services
│   ├── hooks/         # Custom React hooks
│   ├── styles/        # CSS/SCSS styles
│   ├── public/        # Static assets
│   ├── types/         # TypeScript type definitions
│   ├── package.json   # npm package definition
│   └── tsconfig.json  # TypeScript configuration
├── charts/        # Helm charts for deployment
├── .github/       # GitHub Actions workflows
├── LICENSE
└── README.md
```

### Build Process
1. Dependency management via Go modules and npm/yarn
2. Build using standard Go toolchain and Next.js build system
3. Containerization with Docker for both backend and frontend
4. Deployment via Helm charts to Kubernetes

### Testing Approach
- Backend unit tests using Go's testing package
- Frontend unit tests using Jest and React Testing Library
- Integration tests for API endpoints
- End-to-end tests using Cypress or Playwright
- Manual testing for complex UI interactions

## Technical Constraints

### Performance Considerations
- Backup operations are I/O intensive
- Network transfer to S3 can be a bottleneck
- Metadata operations should be lightweight
- Web UI should remain responsive during backup operations
- Database-specific optimizations may be needed for different engines
- Frontend must be optimized for bundle size and initial load performance

### Scalability Aspects
- Designed for moderate scale (dozens of databases)
- Not intended for massive-scale deployments
- Vertical scaling preferred over horizontal scaling
- Separate scaling of frontend and backend components in Kubernetes

### Security Requirements
- Database credentials must be stored securely
- S3 credentials must be protected
- No sensitive information in logs or metrics
- Admin UI will be secured with Auth0 authentication
- JWT-based API security with proper scope validation
- Secure Stripe integration for payments

### Compliance Needs
- Data retention policies must be strictly enforced
- Audit trail of backup operations maintained
- User data management compliance with privacy regulations
- Secure handling of payment information

## Deployment Patterns

### Current Container Deployment
- Docker container for GoSQLGuard
- Volume mounts for:
  - Configuration
  - Local backup storage
  - Metadata persistence
- Environment variables for configuration overrides

### Planned Kubernetes Deployment
- Separate deployments for frontend and backend
- StatefulSet for MariaDB database
- ConfigMap for configuration
- Secrets for credentials (database, Auth0, Stripe)
- PersistentVolumeClaim for local storage
- Ingress for routing
- ArgoCD for GitOps-based continuous deployment

### Traditional Deployment
- Single binary with external configuration
- Systemd service configuration
- NFS or local storage for backups

## Operational Considerations

### Monitoring
- Prometheus metrics for:
  - Backup success/failure counts
  - Backup duration
  - Backup size
  - Storage utilization
  - Last successful backup time
  - API endpoint performance
  - Frontend performance metrics

### Logging
- Structured logging to stdout/stderr
- Log levels: Debug, Info, Warning, Error
- Key events logged:
  - Backup start/complete/fail
  - Storage operations
  - Configuration changes
  - Retention policy enforcement
  - Authentication events
  - Subscription changes

### Backup Verification
- Not built into core (separate process recommended)
- Integrates with external testing via API

### Disaster Recovery
- Metadata is persisted to database
- Regular database backups
- In case of complete failure, backups can be re-indexed by scanning storage

## External Interfaces

### Current Admin Web UI
- Dashboard for backup status
- Filters for viewing backups by type, database, status
- Manual backup triggering
- Retention policy enforcement

### Planned React Frontend
- Modern dashboard with real-time updates
- Advanced filtering and search capabilities
- User management and team features
- Subscription management interface
- Mobile-responsive design

### API Endpoints
- Current:
  - `/api/backups/run` - Trigger manual backup
  - `/api/retention/run` - Trigger retention policy enforcement
  - Status endpoints for monitoring integration

- Planned:
  - RESTful API with comprehensive CRUD operations
  - Standardized response format with pagination
  - Authentication via JWT
  - Versioned API (e.g., `/api/v1/...`)

### Command Line Interface
- Environment variable configuration
- Debug flags
- Path overrides

### Prometheus Metrics
- Exposed on `/metrics` endpoint
- Standard Prometheus format
- Custom metrics for backup operations
