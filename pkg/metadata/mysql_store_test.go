package metadata

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestMySQLStoreInitialization tests MySQL store initialization
func TestMySQLStoreInitialization(t *testing.T) {
	// This test would require a real MySQL instance or extensive mocking
	// For now, we'll test the basic structure

	dbStore := &DBStore{
		db:          nil, // Would be a mock in real test
		initialized: true,
	}

	assert.NotNil(t, dbStore)
	assert.True(t, dbStore.initialized)
}

// newMockDBStore returns a DBStore on a sqlmock connection that matches SQL
// exactly (after whitespace normalization), so an expectation names the whole
// statement GORM sends, not just a keyword.
func newMockDBStore(t *testing.T) (*DBStore, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	require.NoError(t, err)

	return &DBStore{db: db, initialized: true}, mock
}

// TestMySQLStoreMockOperations pins the statements CreateBackupMeta and
// UpdateBackupStatus send, in order, inside their transactions. Any missing,
// extra or reordered statement, or a wrong argument, fails the test
// (cluster-services-1554: the previous version asserted nothing).
func TestMySQLStoreMockOperations(t *testing.T) {
	dbStore, mock := newMockDBStore(t)

	// CreateBackupMeta: one INSERT in GORM's default transaction.
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `backups` (`id`,`server_name`,`server_type`,`database_name`,`backup_type`,`created_at`,`completed_at`,`size`,`status`,`error_message`,`retention_policy`,`expires_at`,`log_file_path`,`s3_upload_status`,`s3_upload_error`,`s3_upload_complete`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)").
		WithArgs(sqlmock.AnyArg(), "server1", "mysql", "testdb", "daily", sqlmock.AnyArg(), nil, int64(0),
			string(types.StatusPending), "", "", nil, "", string(types.StatusPending), "", nil).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	backup := dbStore.CreateBackupMeta("server1", "mysql", "testdb", "daily")
	require.NotNil(t, backup)
	assert.Equal(t, "server1", backup.ServerName)
	assert.Equal(t, "testdb", backup.Database)
	assert.Equal(t, "daily", backup.BackupType)
	assert.Equal(t, types.StatusPending, backup.Status)
	require.NoError(t, mock.ExpectationsWereMet(), "CreateBackupMeta statements")

	backupID := backup.ID

	// UpdateBackupStatus: status update, replace local paths, recompute stats,
	// all in one transaction.
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `backups` SET `completed_at`=?,`error_message`=?,`size`=?,`status`=? WHERE id = ?").
		WithArgs(sqlmock.AnyArg(), "", int64(1024), string(types.StatusSuccess), backupID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `local_paths` WHERE backup_id = ?").
		WithArgs(backupID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO `local_paths` (`backup_id`,`organization`,`path`) VALUES (?,?,?)").
		WithArgs(backupID, "by-server", "/backup1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT COALESCE(SUM(size), 0) FROM `backups` WHERE status = ?").
		WithArgs(string(types.StatusSuccess)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(int64(1024)))
	mock.ExpectQuery("SELECT COALESCE(SUM(size), 0) FROM `backups` WHERE s3_upload_status = ?").
		WithArgs(string(types.StatusSuccess)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(int64(0)))
	mock.ExpectExec("UPDATE `metadata_stats` SET `last_updated`=?,`total_local_size`=?,`total_s3_size`=? WHERE id = ?").
		WithArgs(sqlmock.AnyArg(), int64(1024), int64(0), 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := dbStore.UpdateBackupStatus(backupID, types.StatusSuccess, map[string]string{"by-server": "/backup1"}, 1024, "")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet(), "UpdateBackupStatus statements")
}

// TestMySQLStoreUpdateBackupStatusRollsBack: a failing statement inside the
// transaction rolls it back and returns the error.
func TestMySQLStoreUpdateBackupStatusRollsBack(t *testing.T) {
	dbStore, mock := newMockDBStore(t)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `backups` SET `completed_at`=?,`error_message`=?,`size`=?,`status`=? WHERE id = ?").
		WithArgs(sqlmock.AnyArg(), "boom", int64(0), string(types.StatusError), "b1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `local_paths` WHERE backup_id = ?").
		WithArgs("b1").
		WillReturnError(errors.New("disk full"))
	mock.ExpectRollback()

	err := dbStore.UpdateBackupStatus("b1", types.StatusError, nil, 0, "boom")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestMySQLStoreMigration tests migration from file to database
func TestMySQLStoreMigration(t *testing.T) {
	// Create a file store with some data
	fileStore := &Store{
		metadata: Data{
			Backups: []types.BackupMeta{
				{
					ID:         "backup1",
					ServerName: "server1",
					Database:   "db1",
					Status:     types.StatusSuccess,
					Size:       1024,
					CreatedAt:  time.Now().Add(-24 * time.Hour),
				},
				{
					ID:           "backup2",
					ServerName:   "server2",
					Database:     "db2",
					Status:       types.StatusError,
					ErrorMessage: "Connection failed",
					CreatedAt:    time.Now().Add(-12 * time.Hour),
				},
			},
			Version:     "1.0",
			LastUpdated: time.Now(),
		},
	}

	// Create mock for migration
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	dialector := mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	})
	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)

	// Create DBStore for testing (unused in this mock test)
	_ = &DBStore{
		db:          db,
		initialized: true,
	}

	// In a real migration, we would insert the data into the database
	// For this test, we're just verifying the source data
	assert.Equal(t, 2, len(fileStore.metadata.Backups))
	assert.Equal(t, "backup1", fileStore.metadata.Backups[0].ID)
	assert.Equal(t, types.StatusSuccess, fileStore.metadata.Backups[0].Status)
	assert.Equal(t, "backup2", fileStore.metadata.Backups[1].ID)
	assert.Equal(t, types.StatusError, fileStore.metadata.Backups[1].Status)
}

// TestMySQLStoreErrorHandling tests error scenarios
func TestMySQLStoreErrorHandling(t *testing.T) {
	// Test with nil database
	dbStore := &DBStore{
		db:          nil,
		initialized: false,
	}

	// Operations should work with cache even if DB is nil
	backup := dbStore.CreateBackupMeta("server1", "mysql", "testdb", "daily")
	assert.NotNil(t, backup)

	// GetBackups should return empty when not initialized
	backups := dbStore.GetBackups()
	assert.Equal(t, 0, len(backups))
}

// TestMySQLStoreStatistics tests statistics calculations
func TestMySQLStoreStatistics(t *testing.T) {
	// Create a file store for testing statistics
	fileStore := &Store{
		metadata: Data{
			Backups: []types.BackupMeta{
				{
					ID:         "backup1",
					ServerName: "server1",
					Database:   "db1",
					BackupType: "daily",
					Status:     types.StatusSuccess,
					Size:       1024,
					CreatedAt:  time.Now(),
				},
				{
					ID:         "backup2",
					ServerName: "server1",
					Database:   "db2",
					BackupType: "hourly",
					Status:     types.StatusSuccess,
					Size:       2048,
					CreatedAt:  time.Now(),
				},
				{
					ID:         "backup3",
					ServerName: "server2",
					Database:   "db1",
					BackupType: "daily",
					Status:     types.StatusError,
					Size:       0,
					CreatedAt:  time.Now(),
				},
			},
			TotalLocalSize: 3072,
			LastUpdated:    time.Now(),
		},
	}

	stats := fileStore.GetStats()

	// GetStats reports totalCount plus per-status counts under statusCounts; the
	// dashboard templates read those keys. ("totalBackups"/"successCount"/
	// "errorCount" were never emitted by any store.)
	assert.Equal(t, 3, stats["totalCount"])
	statusCounts := stats["statusCounts"].(map[string]int)
	assert.Equal(t, 2, statusCounts["success"])
	assert.Equal(t, 1, statusCounts["error"])
	assert.Equal(t, int64(3072), stats["totalLocalSize"])

	// Check type distribution
	typeDistribution := stats["typeDistribution"].(map[string]int)
	assert.Equal(t, 2, typeDistribution["daily"])
	assert.Equal(t, 1, typeDistribution["hourly"])

	// Check server distribution
	serverDistribution := stats["serverDistribution"].(map[string]int)
	assert.Equal(t, 2, serverDistribution["server1"])
	assert.Equal(t, 1, serverDistribution["server2"])
}

// TestDatabaseConnectionError tests fallback when database connection fails
func TestDatabaseConnectionError(t *testing.T) {
	// Save original config
	originalConfig := config.CFG.MetadataDB

	// Set invalid database config
	config.CFG.MetadataDB = config.MetadataDBConfig{
		Enabled:  true,
		Host:     "invalid-host",
		Port:     3306,
		Username: "test",
		Password: "test",
		Database: "test",
	}

	// Initialize should fall back to file store
	DefaultStore = nil
	err := Initialize()

	// Should succeed with file store
	assert.NoError(t, err)
	assert.NotNil(t, DefaultStore)

	// Verify it's using file store, not DB store
	_, isDBStore := DefaultStore.(*DBStore)
	assert.False(t, isDBStore)

	// Restore config
	config.CFG.MetadataDB = originalConfig
}
