#!/bin/bash
# Main script to run the GoSQLGuard test environment

echo "=== Starting GoSQLGuard Test Environment ==="

# We're now directly mounting test-config.yaml, no need to copy it

# Clean up any existing containers
echo "Cleaning up any existing containers..."
docker-compose down -v

# Start the services defined in docker-compose.yml
echo "Starting containers..."
docker-compose up -d

# Wait for services to be healthy
echo "Waiting for services to be ready..."
echo "This may take a minute or two..."

# Run the setup scripts
echo "Setting up MySQL test databases..."
./setup-test-db.sh

echo "Setting up PostgreSQL test databases..."
./setup-postgres-db.sh

echo "Testing S3 endpoint connectivity..."
./test-s3-connection.sh

# Show running containers
echo "=== Running Containers ==="
docker-compose ps

# Check GoSQLGuard logs
echo "=== GoSQLGuard Logs ==="
docker-compose logs gosqlguard

# Access the admin interface
echo ""
echo "=== GoSQLGuard Admin Interface ==="
echo "The admin interface should be available at: http://localhost:8888"
echo ""

# Instructions for running tests
echo "=== Running Tests ==="
echo "To run MySQL tests:"
echo "  go test -v ./mysql_test.go"
echo ""
echo "To run PostgreSQL tests:" 
echo "  TEST_DB_TYPE=postgres go test -v ./pkg/test/integration/postgresql/..."
echo ""
echo "To run all tests:"
echo "  go test -v ./..."
echo ""

# Show how to access the databases for verification
echo "=== Database Access ==="
echo "To access MySQL:"
echo "  docker exec -it gosqlguard-mysql mysql -u backup-user -ptest-password"
echo ""
echo "To access PostgreSQL:"
echo "  docker exec -it gosqlguard-postgres psql -U backup-user"
echo ""

echo "=== MinIO Access ==="
echo "MinIO Console: http://localhost:9001"
echo "  Username: minioadmin"
echo "  Password: minioadmin"
echo ""

echo "=== Test Environment Ready ==="
echo "To stop the environment: docker-compose down"
echo ""

echo "To run just MySQL tests:"
echo "  ./run-mysql-tests.sh"
echo ""
echo "To run just PostgreSQL tests:"
echo "  ./run-postgres-tests.sh"
echo ""

# Ask if we should run tests now
read -p "Would you like to run tests now? (m=MySQL/p=PostgreSQL/a=All/n=No): " RUN_TESTS
if [[ "$RUN_TESTS" =~ ^[Mm]$ ]]; then
    echo "Running MySQL tests..."
    ./run-mysql-tests.sh
elif [[ "$RUN_TESTS" =~ ^[Pp]$ ]]; then
    echo "Running PostgreSQL tests..."
    ./run-postgres-tests.sh
elif [[ "$RUN_TESTS" =~ ^[Aa]$ ]]; then
    echo "Running all tests..."
    go test -v ./...
fi
