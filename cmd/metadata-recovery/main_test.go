package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackupFilePattern(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		shouldMatch bool
		expected  map[string]string
	}{
		{
			name:        "Valid hourly backup",
			filename:    "server1-database1-hourly-20250523-120000.sql.gz",
			shouldMatch: true,
			expected: map[string]string{
				"server":   "server1",
				"database": "database1",
				"type":     "hourly",
				"timestamp": "20250523-120000",
			},
		},
		{
			name:        "Valid daily backup with hyphen in server name",
			filename:    "prod-server-users-daily-20250523-000000.sql.gz",
			shouldMatch: true,
			expected: map[string]string{
				"server":   "prod-server",
				"database": "users",
				"type":     "daily",
				"timestamp": "20250523-000000",
			},
		},
		{
			name:        "Valid manual backup",
			filename:    "test-data-manual-20250523-143052.sql.gz",
			shouldMatch: true,
			expected: map[string]string{
				"server":   "test",
				"database": "data",
				"type":     "manual",
				"timestamp": "20250523-143052",
			},
		},
		{
			name:        "Invalid extension",
			filename:    "server1-database1-hourly-20250523-120000.sql",
			shouldMatch: false,
		},
		{
			name:        "Invalid format - missing type",
			filename:    "server1-database1-20250523-120000.sql.gz",
			shouldMatch: false,
		},
		{
			name:        "Invalid format - wrong timestamp",
			filename:    "server1-database1-hourly-2025-05-23.sql.gz",
			shouldMatch: false,
		},
		{
			name:        "Invalid type",
			filename:    "server1-database1-custom-20250523-120000.sql.gz",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := backupFilePattern.FindStringSubmatch(tt.filename)
			
			if tt.shouldMatch {
				require.NotNil(t, matches, "Expected pattern to match %s", tt.filename)
				assert.Equal(t, tt.expected["server"], matches[1])
				assert.Equal(t, tt.expected["database"], matches[2])
				assert.Equal(t, tt.expected["type"], matches[3])
				assert.Equal(t, tt.expected["timestamp"], matches[4])
			} else {
				assert.Nil(t, matches, "Expected pattern not to match %s", tt.filename)
			}
		})
	}
}

func TestScanLocalStorage(t *testing.T) {
	// Create temporary directory structure
	tempDir, err := os.MkdirTemp("", "gosqlguard_recovery_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test backup files
	testFiles := []struct {
		path     string
		content  string
		valid    bool
	}{
		{
			path:    "hourly/server1-db1-hourly-20250523-100000.sql.gz",
			content: "backup data",
			valid:   true,
		},
		{
			path:    "daily/server1-db1-daily-20250523-000000.sql.gz",
			content: "backup data",
			valid:   true,
		},
		{
			path:    "manual/prod-server-users-manual-20250523-143052.sql.gz",
			content: "backup data",
			valid:   true,
		},
		{
			path:    "invalid/server1-db1.sql.gz",
			content: "backup data",
			valid:   false,
		},
		{
			path:    ".metadata/metadata.json",
			content: "{}",
			valid:   false,
		},
		{
			path:    "other/README.md",
			content: "readme",
			valid:   false,
		},
	}

	for _, tf := range testFiles {
		fullPath := filepath.Join(tempDir, tf.path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		
		err = os.WriteFile(fullPath, []byte(tf.content), 0644)
		require.NoError(t, err)
	}

	// Mock config for testing
	oldBackupDir := backupDir
	backupDir = tempDir
	defer func() { backupDir = oldBackupDir }()

	// Scan local storage
	backups := scanLocalStorage()

	// Verify results
	expectedCount := 3
	assert.Len(t, backups, expectedCount, "Expected %d valid backups", expectedCount)

	// Check specific backups
	backupMap := make(map[string]RecoveredBackup)
	for _, b := range backups {
		backupMap[b.Filename] = b
	}

	// Verify hourly backup
	hourly, ok := backupMap["server1-db1-hourly-20250523-100000.sql.gz"]
	assert.True(t, ok, "Expected to find hourly backup")
	assert.Equal(t, "server1", hourly.ServerName)
	assert.Equal(t, "db1", hourly.Database)
	assert.Equal(t, "hourly", hourly.BackupType)
	assert.Equal(t, "20250523-100000", hourly.Timestamp)
	assert.False(t, hourly.IsS3)

	// Verify manual backup with hyphenated server name
	manual, ok := backupMap["prod-server-users-manual-20250523-143052.sql.gz"]
	assert.True(t, ok, "Expected to find manual backup")
	assert.Equal(t, "prod-server", manual.ServerName)
	assert.Equal(t, "users", manual.Database)
}

func TestReconcileBackups(t *testing.T) {
	now := time.Now()
	
	testBackups := []RecoveredBackup{
		// Duplicate - local
		{
			Filename:   "server1-db1-hourly-20250523-100000.sql.gz",
			Path:       "/backups/hourly/server1-db1-hourly-20250523-100000.sql.gz",
			Size:       1024,
			ModTime:    now,
			ServerName: "server1",
			Database:   "db1",
			BackupType: "hourly",
			Timestamp:  "20250523-100000",
			IsS3:       false,
		},
		// Duplicate - S3
		{
			Filename:   "server1-db1-hourly-20250523-100000.sql.gz",
			Path:       "backups/hourly/server1-db1-hourly-20250523-100000.sql.gz",
			Size:       1024,
			ModTime:    now,
			ServerName: "server1",
			Database:   "db1",
			BackupType: "hourly",
			Timestamp:  "20250523-100000",
			IsS3:       true,
			S3Bucket:   "backup-bucket",
			S3Key:      "backups/hourly/server1-db1-hourly-20250523-100000.sql.gz",
		},
		// Unique backup
		{
			Filename:   "server2-db2-daily-20250523-000000.sql.gz",
			Path:       "/backups/daily/server2-db2-daily-20250523-000000.sql.gz",
			Size:       2048,
			ModTime:    now,
			ServerName: "server2",
			Database:   "db2",
			BackupType: "daily",
			Timestamp:  "20250523-000000",
			IsS3:       false,
		},
	}

	reconciled := reconcileBackups(testBackups)

	// Should have 2 unique backups
	assert.Len(t, reconciled, 2)

	// Find the reconciled duplicate
	var merged RecoveredBackup
	for _, b := range reconciled {
		if b.ServerName == "server1" && b.Database == "db1" {
			merged = b
			break
		}
	}

	// Verify merged backup has info from both sources
	assert.Equal(t, "server1", merged.ServerName)
	assert.Equal(t, "db1", merged.Database)
	assert.Equal(t, "/backups/hourly/server1-db1-hourly-20250523-100000.sql.gz", merged.Path)
	assert.Equal(t, "backup-bucket", merged.S3Bucket)
	assert.Equal(t, "backups/hourly/server1-db1-hourly-20250523-100000.sql.gz", merged.S3Key)
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
		{0, "0 B"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d bytes", tt.bytes), func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper variable for testing
var backupDir string

// Override scanLocalStorage for testing
func init() {
	// Save original function if needed
}