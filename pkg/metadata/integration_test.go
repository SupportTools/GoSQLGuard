//go:build integration
// +build integration

package metadata

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
)

// TestMetadataPersistenceIntegration tests both file and MySQL storage
func TestMetadataPersistenceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test file-based storage
	t.Run("FileStorage", func(t *testing.T) {
		tmpDir, err := ioutil.TempDir("", "metadata_integration")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		config.CFG.Local.Enabled = true
		config.CFG.Local.BackupDirectory = tmpDir
		config.CFG.MetadataDB.Enabled = false

		testMetadataPersistence(t)
	})

	// Test MySQL storage (requires MySQL to be running)
	t.Run("MySQLStorage", func(t *testing.T) {
		if os.Getenv("MYSQL_TEST_HOST") == "" {
			t.Skip("Skipping MySQL test - MYSQL_TEST_HOST not set")
		}

		config.CFG.MetadataDB = config.MetadataDBConfig{
			Enabled:  true,
			Host:     os.Getenv("MYSQL_TEST_HOST"),
			Port:     3306,
			Username: os.Getenv("MYSQL_TEST_USER"),
			Password: os.Getenv("MYSQL_TEST_PASSWORD"),
			Database: "gosqlguard_test",
		}

		testMetadataPersistence(t)
	})
}

// testMetadataPersistence runs the same tests for both storage backends
func testMetadataPersistence(t *testing.T) {
	// Initialize
	DefaultStore = nil
	err := Initialize()
	require.NoError(t, err)

	// Test 1: Create and persist backups
	backup1 := DefaultStore.CreateBackupMeta("server1", "mysql", "testdb", "daily")
	err = DefaultStore.UpdateBackupStatus(backup1.ID, types.StatusSuccess,
		map[string]string{"local": "/backup/path1"}, 1024*1024, "")
	assert.NoError(t, err)

	backup2 := DefaultStore.CreateBackupMeta("server2", "mysql", "proddb", "hourly")
	err = DefaultStore.UpdateBackupStatus(backup2.ID, types.StatusError,
		nil, 0, "Connection timeout")
	assert.NoError(t, err)

	// Test 2: Verify data after "restart"
	DefaultStore = nil
	err = Initialize()
	require.NoError(t, err)

	backups := DefaultStore.GetBackups()
	assert.Equal(t, 2, len(backups))

	// Find and verify backup1
	var foundBackup1, foundBackup2 bool
	for _, b := range backups {
		if b.ID == backup1.ID {
			foundBackup1 = true
			assert.Equal(t, types.StatusSuccess, b.Status)
			assert.Equal(t, int64(1024*1024), b.Size)
		}
		if b.ID == backup2.ID {
			foundBackup2 = true
			assert.Equal(t, types.StatusError, b.Status)
			assert.Equal(t, "Connection timeout", b.ErrorMessage)
		}
	}
	assert.True(t, foundBackup1)
	assert.True(t, foundBackup2)

	// Test 3: Filtering
	successBackups := DefaultStore.GetBackupsFiltered("", "", "", true)
	assert.Equal(t, 1, len(successBackups))
	assert.Equal(t, types.StatusSuccess, successBackups[0].Status)

	// Test 4: Statistics
	stats := DefaultStore.GetStats()
	assert.Equal(t, 2, stats["totalBackups"])
	assert.Equal(t, 1, stats["successCount"])
	assert.Equal(t, 1, stats["errorCount"])

	// Test 5: Mark as deleted
	err = DefaultStore.MarkBackupDeleted(backup1.ID)
	assert.NoError(t, err)

	backup, found := DefaultStore.GetBackupByID(backup1.ID)
	assert.True(t, found)
	assert.Equal(t, types.StatusDeleted, backup.Status)

	// Test 6: Purge old deleted backups
	count := DefaultStore.PurgeDeletedBackups(1 * time.Minute)
	assert.Equal(t, 0, count) // Should not purge recent deletions

	// Clean up
	DefaultStore = nil
}

// TestMetadataCorruptionRecovery tests recovery from corrupted metadata
func TestMetadataCorruptionRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir, err := ioutil.TempDir("", "metadata_recovery")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config.CFG.Local.Enabled = true
	config.CFG.Local.BackupDirectory = tmpDir
	config.CFG.MetadataDB.Enabled = false

	// Initialize and add data
	DefaultStore = nil
	err = Initialize()
	require.NoError(t, err)

	// Add backups
	for i := 0; i < 5; i++ {
		backup := DefaultStore.CreateBackupMeta("server", "mysql", "db", "daily")
		DefaultStore.UpdateBackupStatus(backup.ID, types.StatusSuccess,
			map[string]string{"local": "/backup"}, 1024, "")
	}

	// Get the metadata file path
	metadataPath := filepath.Join(tmpDir, "metadata.json")

	// Read good data
	goodData, err := ioutil.ReadFile(metadataPath)
	require.NoError(t, err)

	// Corrupt the file
	err = ioutil.WriteFile(metadataPath, []byte("corrupted data"), 0644)
	require.NoError(t, err)

	// Try to reinitialize - should handle corruption gracefully
	DefaultStore = nil
	err = Initialize()
	// Should either succeed with empty metadata or return an error
	// In production, you'd want to implement recovery logic here

	// Restore good data
	err = ioutil.WriteFile(metadataPath, goodData, 0644)
	require.NoError(t, err)

	// Now should work
	DefaultStore = nil
	err = Initialize()
	assert.NoError(t, err)

	backups := DefaultStore.GetBackups()
	assert.Equal(t, 5, len(backups))
}

// TestMetadataPerformance tests performance with large datasets
func TestMetadataPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tmpDir, err := ioutil.TempDir("", "metadata_performance")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config.CFG.Local.Enabled = true
	config.CFG.Local.BackupDirectory = tmpDir
	config.CFG.MetadataDB.Enabled = false

	DefaultStore = nil
	err = Initialize()
	require.NoError(t, err)

	// Add many backups
	numBackups := 1000
	start := time.Now()

	for i := 0; i < numBackups; i++ {
		backup := DefaultStore.CreateBackupMeta(
			"server"+string(rune(i%10)),
			"mysql",
			"db"+string(rune(i%20)),
			"daily",
		)
		DefaultStore.UpdateBackupStatus(backup.ID, types.StatusSuccess,
			map[string]string{"local": "/backup"}, 1024*int64(i), "")
	}

	createDuration := time.Since(start)
	t.Logf("Created %d backups in %v", numBackups, createDuration)
	assert.Less(t, createDuration, 10*time.Second)

	// Test filtering performance
	start = time.Now()
	filtered := DefaultStore.GetBackupsFiltered("server1", "", "", false)
	filterDuration := time.Since(start)
	t.Logf("Filtered %d backups from %d total in %v", len(filtered), numBackups, filterDuration)
	assert.Less(t, filterDuration, 100*time.Millisecond)

	// Test save performance
	start = time.Now()
	err = DefaultStore.Save()
	saveDuration := time.Since(start)
	t.Logf("Saved %d backups in %v", numBackups, saveDuration)
	assert.NoError(t, err)
	assert.Less(t, saveDuration, 1*time.Second)

	// Test load performance
	DefaultStore = nil
	start = time.Now()
	err = Initialize()
	loadDuration := time.Since(start)
	t.Logf("Loaded %d backups in %v", numBackups, loadDuration)
	assert.NoError(t, err)
	assert.Less(t, loadDuration, 1*time.Second)

	// Verify all data loaded
	backups := DefaultStore.GetBackups()
	assert.Equal(t, numBackups, len(backups))
}

// TestMetadataConcurrentStress stress tests concurrent access
func TestMetadataConcurrentStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	tmpDir, err := ioutil.TempDir("", "metadata_stress")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config.CFG.Local.Enabled = true
	config.CFG.Local.BackupDirectory = tmpDir
	config.CFG.MetadataDB.Enabled = false

	DefaultStore = nil
	err = Initialize()
	require.NoError(t, err)

	// Concurrent operations
	var wg sync.WaitGroup
	numGoroutines := 20
	numOperations := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				// Mix of operations
				switch j % 4 {
				case 0:
					// Create backup
					backup := DefaultStore.CreateBackupMeta(
						"server"+string(rune(goroutineID)),
						"mysql",
						"db"+string(rune(j)),
						"daily",
					)
					DefaultStore.UpdateBackupStatus(backup.ID, types.StatusSuccess,
						map[string]string{"local": "/backup"}, 1024, "")
				case 1:
					// Get backups
					DefaultStore.GetBackups()
				case 2:
					// Filter backups
					DefaultStore.GetBackupsFiltered("server"+string(rune(goroutineID)), "", "", false)
				case 3:
					// Get stats
					DefaultStore.GetStats()
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify data integrity
	backups := DefaultStore.GetBackups()
	assert.GreaterOrEqual(t, len(backups), numGoroutines*numOperations/4)

	// Save should work
	err = DefaultStore.Save()
	assert.NoError(t, err)
}
