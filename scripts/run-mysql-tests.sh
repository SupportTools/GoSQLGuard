#!/bin/bash
# Script to run MySQL tests

echo "=== Running MySQL Tests ==="

# Run the MySQL tests
go test -v ./mysql_test.go

# Check the test result
TEST_RESULT=$?
if [ $TEST_RESULT -eq 0 ]; then
    echo -e "\n✅ MySQL tests PASSED"
else
    echo -e "\n❌ MySQL tests FAILED (exit code: $TEST_RESULT)"
fi

exit $TEST_RESULT
