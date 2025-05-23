#!/bin/bash

# Script to run metadata persistence tests

set -e

echo "=== Running GoSQLGuard Metadata Persistence Tests ==="
echo

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Change to the project directory
cd "$(dirname "$0")/.."

# Install test dependencies if needed
echo "Installing test dependencies..."
go get -t ./pkg/metadata/...

# Run unit tests
echo
echo -e "${YELLOW}Running unit tests...${NC}"
go test -v ./pkg/metadata -timeout 30s

# Run unit tests with race detection
echo
echo -e "${YELLOW}Running unit tests with race detection...${NC}"
go test -race -v ./pkg/metadata -timeout 60s

# Run integration tests if MySQL is available
if [ ! -z "$MYSQL_TEST_HOST" ]; then
    echo
    echo -e "${YELLOW}Running integration tests with MySQL...${NC}"
    go test -v ./pkg/metadata -tags=integration -timeout 120s
else
    echo
    echo -e "${YELLOW}Skipping MySQL integration tests (MYSQL_TEST_HOST not set)${NC}"
    echo "To run MySQL tests, set environment variables:"
    echo "  export MYSQL_TEST_HOST=localhost"
    echo "  export MYSQL_TEST_USER=root"
    echo "  export MYSQL_TEST_PASSWORD=password"
fi

# Run integration tests without MySQL (file-based only)
echo
echo -e "${YELLOW}Running file-based integration tests...${NC}"
go test -v ./pkg/metadata -tags=integration -run="FileStorage" -timeout 120s

# Generate coverage report
echo
echo -e "${YELLOW}Generating coverage report...${NC}"
go test -coverprofile=coverage.out ./pkg/metadata
go tool cover -html=coverage.out -o coverage.html

echo
echo -e "${GREEN}✓ All metadata persistence tests completed!${NC}"
echo "Coverage report generated: coverage.html"

# Test summary
echo
echo "=== Test Summary ==="
echo "- Unit tests: PASSED"
echo "- Race condition tests: PASSED"
echo "- File-based integration tests: PASSED"
if [ ! -z "$MYSQL_TEST_HOST" ]; then
    echo "- MySQL integration tests: PASSED"
fi

echo
echo "To run specific tests:"
echo "  go test -v ./pkg/metadata -run TestFileStorePersistenceAcrossRestarts"
echo "  go test -v ./pkg/metadata -run TestMetadataCorruptionRecovery -tags=integration"