#!/bin/bash
# Script to verify that backup functionality is working correctly

echo "=== GoSQLGuard Backup Verification Test ==="

# Ensure containers are running
if ! docker ps | grep -q gosqlguard-controller; then
  echo "ERROR: GoSQLGuard containers are not running. Start them with ./run-tests.sh first."
  exit 1
fi

# Helper function to check if files exist
check_backups() {
  echo "Checking for backup files..."
  BACKUP_COUNT=$(ls -la ./backups/hourly/ | grep -c ".sql.gz")
  if [ $BACKUP_COUNT -gt 0 ]; then
    echo "✅ Found $BACKUP_COUNT backup file(s) in local storage"
    return 0
  else
    echo "❌ No backup files found in local storage"
    return 1
  fi
}

# Helper function to trigger manual backup
trigger_backup() {
  echo "Triggering a manual backup..."
  # Check the admin interface is available
  if ! curl -s http://localhost:8888 > /dev/null; then
    echo "❌ Admin interface not responding at http://localhost:8888"
    exit 1
  fi
  
  # Trigger a manual backup via the API
  RESPONSE=$(curl -s -X POST http://localhost:8888/api/backups/run)
  if [[ $RESPONSE == *"success"* ]]; then
    echo "✅ Manual backup triggered successfully"
    return 0
  else
    echo "❌ Failed to trigger manual backup: $RESPONSE"
    return 1
  fi
}

# Test the backup process
run_backup_test() {
  echo "=== Running Backup Test ==="
  
  # Check if any backups already exist
  PRE_COUNT=$(ls -la ./backups/hourly/ 2>/dev/null | grep -c ".sql.gz" || echo "0")
  echo "Initial backup count: $PRE_COUNT"
  
  # Trigger a manual backup
  trigger_backup
  
  # Wait for the backup to complete (up to 60 seconds)
  echo "Waiting for backup to complete..."
  for i in {1..30}; do
    POST_COUNT=$(ls -la ./backups/hourly/ 2>/dev/null | grep -c ".sql.gz" || echo "0")
    if [ $POST_COUNT -gt $PRE_COUNT ]; then
      echo "✅ New backup files created"
      break
    fi
    if [ $i -eq 30 ]; then
      echo "❌ Timed out waiting for backup to complete"
      return 1
    fi
    sleep 2
    echo -n "."
  done
  echo ""
  
  # Verify backup files
  check_backups
  if [ $? -ne 0 ]; then
    return 1
  fi
  
  # Check if metadata is being tracked
  echo "Checking metadata storage..."
  if [ -f ./backups/metadata.json ]; then
    echo "✅ Metadata file exists"
    if grep -q "db1" ./backups/metadata.json; then
      echo "✅ Database metadata found in metadata file"
    else
      echo "❌ Database metadata not found in metadata file"
      return 1
    fi
  else
    echo "❌ Metadata file not found"
    return 1
  fi
  
  # Check if backup contains expected database
  echo "Verifying backup content..."
  LATEST_BACKUP=$(ls -t ./backups/hourly/*.sql.gz | head -1)
  if [ -n "$LATEST_BACKUP" ]; then
    BACKUP_CONTENT=$(zcat "$LATEST_BACKUP" | head -20)
    if [[ $BACKUP_CONTENT == *"test_table"* ]]; then
      echo "✅ Backup contains expected database tables"
    else
      echo "❌ Backup does not contain expected database content"
      return 1
    fi
  else
    echo "❌ No backup files found for verification"
    return 1
  fi
  
  echo "✅ All backup tests passed!"
  return 0
}

# Run the tests
run_backup_test
if [ $? -eq 0 ]; then
  echo "=== Backup Tests PASSED ==="
  exit 0
else
  echo "=== Backup Tests FAILED ==="
  # Show logs to help diagnose the issue
  echo "=== GoSQLGuard Logs ==="
  docker-compose logs gosqlguard
  exit 1
fi
