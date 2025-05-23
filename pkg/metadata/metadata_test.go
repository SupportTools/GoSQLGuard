package metadata

import (
	"encoding/json"
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

// TestFileStoreInitialization tests that the file store initializes correctly
func TestFileStoreInitialization(t *testing.T) {
	// Create temporary directory
	tmpDir, err := ioutil.TempDir("", "metadata_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Set configuration
	config.CFG.Local.Enabled = true
	config.CFG.Local.BackupDirectory = tmpDir
	config.CFG.MetadataDB.Enabled = false

	// Initialize
	DefaultStore = nil
	err = Initialize()
	assert.NoError(t, err)
	assert.NotNil(t, DefaultStore)

	// Check that metadata file was created
	metadataPath := filepath.Join(tmpDir, "metadata.json")
	assert.FileExists(t, metadataPath)
}

// TestFileStoreSaveAndLoad tests saving and loading metadata
func TestFileStoreSaveAndLoad(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "metadata_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create store
	store := &Store{
		filepath: filepath.Join(tmpDir, "test_metadata.json"),
		metadata: MetadataStore{
			Backups:     make([]types.BackupMeta, 0),
			Version:     "1.0",
			LastUpdated: time.Now(),
		},
	}

	// Add some backups
	backup1 := store.CreateBackupMeta("server1", "mysql", "testdb", "daily")
	backup1.Status = types.StatusSuccess
	backup1.Size = 1024 * 1024
	store.UpdateBackupStatus(backup1.ID, types.StatusSuccess, map[string]string{"local": "/path/to/backup"}, backup1.Size, "")

	backup2 := store.CreateBackupMeta("server2", "mysql", "proddb", "hourly")
	backup2.Status = types.StatusError
	store.UpdateBackupStatus(backup2.ID, types.StatusError, nil, 0, "Connection failed")

	// Save
	err = store.Save()
	assert.NoError(t, err)

	// Create new store and load
	store2 := &Store{
		filepath: filepath.Join(tmpDir, "test_metadata.json"),
		metadata: MetadataStore{
			Backups: make([]types.BackupMeta, 0),
		},
	}

	err = store2.Load()
	assert.NoError(t, err)

	// Verify data
	assert.Equal(t, 2, len(store2.metadata.Backups))
	assert.Equal(t, "testdb", store2.metadata.Backups[0].Database)
	assert.Equal(t, types.StatusSuccess, store2.metadata.Backups[0].Status)
	assert.Equal(t, types.StatusError, store2.metadata.Backups[1].Status)
	assert.Equal(t, "Connection failed", store2.metadata.Backups[1].ErrorMessage)
}

// TestFileStoreCorruptedFile tests handling of corrupted metadata files
func TestFileStoreCorruptedFile(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "metadata_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	metadataPath := filepath.Join(tmpDir, "corrupted.json")

	// Write corrupted JSON
	err = ioutil.WriteFile(metadataPath, []byte(`{"backups": [{"id": "test", "status": "invalid json`), 0644)
	require.NoError(t, err)

	// Try to load
	store := &Store{
		filepath: metadataPath,
		metadata: MetadataStore{
			Backups: make([]types.BackupMeta, 0),
		},
	}

	err = store.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

// TestFileStorePartialWrite tests recovery from partial writes
func TestFileStorePartialWrite(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "metadata_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create store with data
	store := &Store{
		filepath: filepath.Join(tmpDir, "partial.json"),
		metadata: MetadataStore{
			Backups:     make([]types.BackupMeta, 0),
			Version:     "1.0",
			LastUpdated: time.Now(),
		},
	}

	// Add backup and save
	backup := store.CreateBackupMeta("server1", "mysql", "testdb", "daily")
	err = store.Save()
	require.NoError(t, err)

	// Read the good data
	goodData, err := ioutil.ReadFile(store.filepath)
	require.NoError(t, err)

	// Simulate partial write by truncating file
	truncatedData := goodData[:len(goodData)/2]
	err = ioutil.WriteFile(store.filepath, truncatedData, 0644)
	require.NoError(t, err)

	// Try to load - should fail
	store2 := &Store{
		filepath: store.filepath,
		metadata: MetadataStore{
			Backups: make([]types.BackupMeta, 0),
		},
	}
	err = store2.Load()
	assert.Error(t, err)

	// Recovery: write backup file
	backupPath := store.filepath + ".backup"
	err = ioutil.WriteFile(backupPath, goodData, 0644)
	require.NoError(t, err)

	// Implement recovery logic (this would be part of the actual recovery implementation)
	_, err = os.Stat(backupPath)
	if err == nil {
		// Restore from backup
		err = os.Rename(backupPath, store.filepath)
		require.NoError(t, err)
	}

	// Now load should work
	err = store2.Load()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(store2.metadata.Backups))
}

// TestFileStoreNonExistentFile tests creating metadata when file doesn't exist
func TestFileStoreNonExistentFile(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "metadata_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := &Store{
		filepath: filepath.Join(tmpDir, "new_metadata.json"),
		metadata: MetadataStore{
			Backups:     make([]types.BackupMeta, 0),
			Version:     "1.0",
			LastUpdated: time.Now(),
		},
	}

	// Load should create the file
	err = store.Load()
	assert.NoError(t, err)
	assert.FileExists(t, store.filepath)

	// Verify empty metadata was saved
	data, err := ioutil.ReadFile(store.filepath)
	require.NoError(t, err)

	var loaded MetadataStore
	err = json.Unmarshal(data, &loaded)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(loaded.Backups))
	assert.Equal(t, "1.0", loaded.Version)
}

// TestFileStoreConcurrentAccess tests thread-safe access to metadata
func TestFileStoreConcurrentAccess(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "metadata_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := &Store{
		filepath: filepath.Join(tmpDir, "concurrent.json"),
		metadata: MetadataStore{
			Backups:     make([]types.BackupMeta, 0),
			Version:     "1.0",
			LastUpdated: time.Now(),
		},
		mutex: sync.RWMutex{},
	}

	// Concurrent writes
	var wg sync.WaitGroup
	errors := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			backup := store.CreateBackupMeta(
				"server"+string(rune(idx)),
				"mysql",
				"db"+string(rune(idx)),
				"daily",
			)
			err := store.UpdateBackupStatus(
				backup.ID,
				types.StatusSuccess,
				map[string]string{"local": "/path/" + backup.ID},
				1024*int64(idx),
				"",
			)
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// Check no errors
	for _, err := range errors {
		assert.NoError(t, err)
	}

	// Verify all backups were added
	assert.Equal(t, 10, len(store.metadata.Backups))

	// Save and reload
	err = store.Save()
	assert.NoError(t, err)

	store2 := &Store{
		filepath: store.filepath,
		metadata: MetadataStore{
			Backups: make([]types.BackupMeta, 0),
		},
	}
	err = store2.Load()
	assert.NoError(t, err)
	assert.Equal(t, 10, len(store2.metadata.Backups))
}

// TestFileStorePersistenceAcrossRestarts simulates app restart
func TestFileStorePersistenceAcrossRestarts(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "metadata_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// First "application run"
	config.CFG.Local.Enabled = true
	config.CFG.Local.BackupDirectory = tmpDir
	config.CFG.MetadataDB.Enabled = false

	// Initialize
	DefaultStore = nil
	err = Initialize()
	require.NoError(t, err)

	// Add backups
	backup1 := DefaultStore.CreateBackupMeta("server1", "mysql", "testdb", "daily")
	DefaultStore.UpdateBackupStatus(backup1.ID, types.StatusSuccess, map[string]string{"local": "/backup1"}, 1024, "")

	backup2 := DefaultStore.CreateBackupMeta("server2", "mysql", "proddb", "hourly")
	DefaultStore.UpdateBackupStatus(backup2.ID, types.StatusSuccess, map[string]string{"local": "/backup2"}, 2048, "")

	// Get stats before "restart"
	statsBefore := DefaultStore.GetStats()

	// Simulate restart by clearing DefaultStore
	DefaultStore = nil

	// Second "application run"
	err = Initialize()
	require.NoError(t, err)

	// Verify data persisted
	backups := DefaultStore.GetBackups()
	assert.Equal(t, 2, len(backups))

	// Verify stats match
	statsAfter := DefaultStore.GetStats()
	assert.Equal(t, statsBefore["totalBackups"], statsAfter["totalBackups"])
	assert.Equal(t, statsBefore["successCount"], statsAfter["successCount"])

	// Verify we can still add new backups
	backup3 := DefaultStore.CreateBackupMeta("server3", "mysql", "newdb", "weekly")
	DefaultStore.UpdateBackupStatus(backup3.ID, types.StatusSuccess, map[string]string{"local": "/backup3"}, 4096, "")

	backups = DefaultStore.GetBackups()
	assert.Equal(t, 3, len(backups))
}

// TestFileStoreMetadataIntegrity tests metadata calculations remain consistent
func TestFileStoreMetadataIntegrity(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "metadata_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store := &Store{
		filepath: filepath.Join(tmpDir, "integrity.json"),
		metadata: MetadataStore{
			Backups:     make([]types.BackupMeta, 0),
			Version:     "1.0",
			LastUpdated: time.Now(),
		},
	}

	// Add various backups
	sizes := []int64{1024, 2048, 4096, 8192}
	for i, size := range sizes {
		backup := store.CreateBackupMeta("server", "mysql", "db", "daily")
		status := types.StatusSuccess
		if i == 2 {
			status = types.StatusError
		}
		store.UpdateBackupStatus(backup.ID, status, map[string]string{"local": "/backup"}, size, "")
	}

	// Verify totals
	assert.Equal(t, int64(1024+2048+8192), store.metadata.TotalLocalSize) // Excludes error backup
	assert.Equal(t, int64(0), store.metadata.TotalS3Size)                  // No S3 uploads

	// Save and reload
	err = store.Save()
	require.NoError(t, err)

	store2 := &Store{
		filepath: store.filepath,
		metadata: MetadataStore{
			Backups: make([]types.BackupMeta, 0),
		},
	}
	err = store2.Load()
	require.NoError(t, err)

	// Verify totals are recalculated correctly
	assert.Equal(t, store.metadata.TotalLocalSize, store2.metadata.TotalLocalSize)
	assert.Equal(t, store.metadata.TotalS3Size, store2.metadata.TotalS3Size)
}