package metadata

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
)

// withDailyRetention configures a "daily" type kept locally for 7 days.
func withDailyRetention(t *testing.T) {
	t.Helper()
	saved := config.CFG
	t.Cleanup(func() { config.CFG = saved })
	config.CFG.BackupTypes = map[string]config.BackupTypeConfig{
		"daily": {Local: config.LocalBackupConfig{Enabled: true, Retention: config.RetentionRule{Duration: "168h"}}},
	}
}

// TestFileStoreSetBackupTimes (cluster-services-1560): a recovered record takes
// the backup's own time for CreatedAt and CompletedAt, its expiry counts from
// that time, and all three survive a save and reload.
func TestFileStoreSetBackupTimes(t *testing.T) {
	withDailyRetention(t)
	path := filepath.Join(t.TempDir(), "metadata.json")
	store := &Store{filepath: path, metadata: Data{Backups: []types.BackupMeta{}, Version: "1.0"}}

	meta := store.CreateBackupMeta("server1", "mysql", "testdb", "daily")
	require.NoError(t, store.UpdateBackupStatus(meta.ID, types.StatusSuccess, map[string]string{"by-server": "/b"}, 10, ""))

	at := time.Date(2026, 10, 1, 3, 4, 5, 0, time.UTC)
	require.NoError(t, store.SetBackupTimes(meta.ID, at, at))

	reloaded := &Store{filepath: path, metadata: Data{Backups: []types.BackupMeta{}}}
	require.NoError(t, reloaded.Load())
	got, ok := reloaded.GetBackupByID(meta.ID)
	require.True(t, ok)
	assert.True(t, at.Equal(got.CreatedAt), "CreatedAt %v", got.CreatedAt)
	assert.True(t, at.Equal(got.CompletedAt), "CompletedAt %v", got.CompletedAt)
	assert.True(t, at.Add(168*time.Hour).Equal(got.ExpiresAt), "ExpiresAt %v", got.ExpiresAt)
	assert.Equal(t, "Keep for 168h (until 2026-10-08 03:04:05)", got.RetentionPolicy)
	assert.Equal(t, types.StatusSuccess, got.Status, "status untouched")

	// A zero completedAt leaves CompletedAt alone.
	before := got.CompletedAt
	require.NoError(t, store.SetBackupTimes(meta.ID, at, time.Time{}))
	got, _ = store.GetBackupByID(meta.ID)
	assert.True(t, before.Equal(got.CompletedAt))

	assert.Error(t, store.SetBackupTimes("no-such-id", at, at))
}

// TestMySQLStoreSetBackupTimes pins the statements DBStore.SetBackupTimes sends:
// read the backup type, then one update of the four time/retention columns.
func TestMySQLStoreSetBackupTimes(t *testing.T) {
	withDailyRetention(t)
	dbStore, mock := newMockDBStore(t)
	at := time.Date(2026, 10, 1, 3, 4, 5, 0, time.UTC)

	mock.ExpectQuery("SELECT `id`,`backup_type` FROM `backups` WHERE id = ? ORDER BY `backups`.`id` LIMIT ?").
		WithArgs("b1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "backup_type"}).AddRow("b1", "daily"))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `backups` SET `completed_at`=?,`created_at`=?,`expires_at`=?,`retention_policy`=? WHERE id = ?").
		WithArgs(at, at, at.Add(168*time.Hour), "Keep for 168h (until 2026-10-08 03:04:05)", "b1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, dbStore.SetBackupTimes("b1", at, at))
	require.NoError(t, mock.ExpectationsWereMet())
}
