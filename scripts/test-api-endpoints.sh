#!/bin/bash

# Script to test all API endpoints

set -e

echo "Running API endpoint tests..."

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to run tests for a package
run_tests() {
    local package=$1
    local description=$2
    
    echo -e "\n${GREEN}Testing ${description}...${NC}"
    if go test -v "${package}" -count=1; then
        echo -e "${GREEN}✓ ${description} tests passed${NC}"
    else
        echo -e "${RED}✗ ${description} tests failed${NC}"
        exit 1
    fi
}

# Change to project directory
cd "$(dirname "$0")/.."

# Run tests for each API package
run_tests "./pkg/api" "API handlers (S3, MySQL options, PostgreSQL options, Server, Schedule)"
run_tests "./pkg/adminserver" "Admin server endpoints (Backup triggering, Retention)"

echo -e "\n${GREEN}All API endpoint tests passed!${NC}"

# Optional: Generate coverage report
if [ "$1" == "--coverage" ]; then
    echo -e "\n${GREEN}Generating coverage report...${NC}"
    go test -coverprofile=coverage.out ./pkg/api ./pkg/adminserver
    go tool cover -html=coverage.out -o coverage.html
    echo "Coverage report generated: coverage.html"
fi