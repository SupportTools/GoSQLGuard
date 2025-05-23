#!/bin/bash
# Script to run PostgreSQL tests with the required environment variable

echo "=== Running PostgreSQL Tests ==="
echo "Setting TEST_DB_TYPE=postgres to activate PostgreSQL tests"

# Set environment variable and run the tests
TEST_DB_TYPE=postgres go test -v ./pkg/test/integration/postgresql/...

# Check the test result
TEST_RESULT=$?
if [ $TEST_RESULT -eq 0 ]; then
    echo -e "\n✅ PostgreSQL tests PASSED"
else
    echo -e "\n❌ PostgreSQL tests FAILED (exit code: $TEST_RESULT)"
fi

exit $TEST_RESULT
