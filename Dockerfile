# Use golang alpine image as the builder stage
FROM golang:1.24.2-alpine AS builder

# Install git and other necessary tools
RUN apk update && apk add --no-cache git bash mysql-client postgresql-client

# Set the Current Working Directory inside the container
WORKDIR /src

# Copy go.mod and go.sum files first to leverage Docker cache
COPY go.mod go.sum ./

# Download dependencies with module and build cache
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the rest of the application source code
COPY . .

# Build arguments for versioning
ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE

# Build the Go app with static linking and versioning information
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-w -s -X github.com/supporttools/GoSQLGuard/pkg/version.Version=${VERSION} \
    -X github.com/supporttools/GoSQLGuard/pkg/version.GitCommit=${GIT_COMMIT} \
    -X github.com/supporttools/GoSQLGuard/pkg/version.BuildTime=${BUILD_DATE}" \
    -o /gosqlguard

# Build the metadata recovery tool
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-w -s -X github.com/supporttools/GoSQLGuard/pkg/version.Version=${VERSION} \
    -X github.com/supporttools/GoSQLGuard/pkg/version.GitCommit=${GIT_COMMIT} \
    -X github.com/supporttools/GoSQLGuard/pkg/version.BuildTime=${BUILD_DATE}" \
    -o /metadata-recovery ./cmd/metadata-recovery/main.go

# Use Ubuntu as the runtime base image
FROM ubuntu:22.04

# Install required packages for backup operations including MySQL 8.0 client
# Set noninteractive to avoid tzdata configuration prompts
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    mysql-client-core-8.0 \
    postgresql-client \
    ca-certificates \
    openssl \
    && rm -rf /var/lib/apt/lists/*

# Container metadata
LABEL org.opencontainers.image.title="GoSQLGuard" \
    org.opencontainers.image.description="A tool for automating database backups and retention with support for MySQL and PostgreSQL." \
    org.opencontainers.image.source="https://github.com/supporttools/GoSQLGuard" \
    org.opencontainers.image.vendor="SupportTools" \
    org.opencontainers.image.licenses="MIT"

# Copy the binaries from the builder stage
COPY --from=builder /gosqlguard /usr/local/bin/gosqlguard
COPY --from=builder /metadata-recovery /usr/local/bin/metadata-recovery

# Set working directory 
WORKDIR /app

# Create directories for backups and metadata
RUN mkdir -p /app/data/backups /app/data/metadata

# Set permissions on directories
RUN chmod -R 755 /app/data

# Set the entrypoint
ENTRYPOINT ["/usr/local/bin/gosqlguard"]
