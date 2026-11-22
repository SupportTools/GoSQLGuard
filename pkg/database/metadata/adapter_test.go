package metadata

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestDBMetadataStoreSetBackupTimes (cluster-services-1560) pins the statements
// DBMetadataStore.SetBackupTimes sends: load the backup, then one update that
// sets created_at, completed_at, and expires_at counted from created_at.
func TestDBMetadataStoreSetBackupTimes(t *testing.T) {
	saved := config.CFG
	t.Cleanup(func() { config.CFG = saved })
	config.CFG.BackupTypes = map[string]config.BackupTypeConfig{
		"daily": {Local: config.LocalBackupConfig{Enabled: true, Retention: config.RetentionRule{Duration: "168h"}}},
	}

	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	store := &DBMetadataStore{repo: NewRepository(db), initialized: true}

	at := time.Date(2026, 10, 1, 3, 4, 5, 0, time.UTC)

	mock.ExpectQuery("SELECT * FROM `backups` WHERE id = ? ORDER BY `backups`.`id` LIMIT ?").
		WithArgs("b1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "backup_type"}).AddRow("b1", "daily"))
	mock.ExpectQuery("SELECT * FROM `local_paths` WHERE `local_paths`.`backup_id` = ?").
		WithArgs("b1").
		WillReturnRows(sqlmock.NewRows([]string{"backup_id"}))
	mock.ExpectQuery("SELECT * FROM `s3_keys` WHERE `s3_keys`.`backup_id` = ?").
		WithArgs("b1").
		WillReturnRows(sqlmock.NewRows([]string{"backup_id"}))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `backups` SET `completed_at`=?,`created_at`=?,`expires_at`=?,`retention_policy`=? WHERE id = ?").
		WithArgs(at, at, at.Add(168*time.Hour), "Keep for 168h (until 2026-10-08 03:04:05)", "b1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, store.SetBackupTimes("b1", at, at))
	require.NoError(t, mock.ExpectationsWereMet())
}
