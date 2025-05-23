# Makefile for GoSQLGuard Docker build and release
# Automatically increments RC version with each release

# Extract version from pkg/version/version.go
VERSION := $(shell grep 'Version.*=' pkg/version/version.go | sed 's/.*= "\(.*\)".*/\1/')

# Read current RC number or set to 1 if file doesn't exist
RC_NUMBER := $(shell if [ -f .rc-version ]; then cat .rc-version; else echo 1; fi)

# Increment for next build
NEXT_RC := $(shell echo $$(($(RC_NUMBER) + 1)))

# Image name
IMAGE := supporttools/gosqlguard

# Full image tag
TAG := $(VERSION)-rc$(RC_NUMBER)

# Git commit for build args
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build date for build args
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go build variables
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
LDFLAGS := -X github.com/supporttools/GoSQLGuard/pkg/version.Version=$(VERSION) \
           -X github.com/supporttools/GoSQLGuard/pkg/version.GitCommit=$(GIT_COMMIT) \
           -X github.com/supporttools/GoSQLGuard/pkg/version.BuildDate=$(BUILD_DATE)

.PHONY: help build build-go build-recovery push release clean increment-rc test

# Default target
help:
	@echo "GoSQLGuard Build System"
	@echo "-----------------------"
	@echo "Available targets:"
	@echo "  build-go     - Build GoSQLGuard binary"
	@echo "  build-recovery - Build metadata recovery tool"
	@echo "  build        - Build Docker image $(IMAGE):$(TAG)"
	@echo "  push         - Push Docker image to registry"
	@echo "  release      - Build and push Docker image"
	@echo "  test         - Run tests"
	@echo "  clean        - Remove build artifacts"
	@echo "  increment-rc - Increment RC number without building"
	@echo ""
	@echo "Current version: $(VERSION)"
	@echo "Current RC: $(RC_NUMBER)"
	@echo "Next RC will be: $(NEXT_RC)"

# Build GoSQLGuard binary
build-go:
	@echo "Building GoSQLGuard binary..."
	go build -ldflags "$(LDFLAGS)" -o gosqlguard main.go
	@echo "Binary built: gosqlguard"

# Build metadata recovery tool
build-recovery:
	@echo "Building metadata recovery tool..."
	go build -ldflags "$(LDFLAGS)" -o metadata-recovery cmd/metadata-recovery/main.go
	@echo "Binary built: metadata-recovery"

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Build the Docker image
build: build-go build-recovery
	@echo "Building $(IMAGE):$(TAG)..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(IMAGE):$(TAG) \
		-t $(IMAGE):latest \
		.
	@echo "Build complete: $(IMAGE):$(TAG)"

# Push the Docker image to registry
push:
	@echo "Pushing $(IMAGE):$(TAG) to registry..."
	docker push $(IMAGE):$(TAG)
	docker push $(IMAGE):latest
	@echo "Push complete"

# Build and push in one step
release: build push
	@echo "Release complete: $(IMAGE):$(TAG)"
	@echo "Incrementing RC number for next build..."
	@echo $(NEXT_RC) > .rc-version
	@echo "Next build will use RC=$(NEXT_RC)"

# Just increment the RC number without building
increment-rc:
	@echo "Incrementing RC number from $(RC_NUMBER) to $(NEXT_RC)"
	@echo $(NEXT_RC) > .rc-version

# Clean temporary files and build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f gosqlguard metadata-recovery
	@echo "Done"
