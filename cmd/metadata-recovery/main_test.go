package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/supporttools/GoSQLGuard/pkg/backup"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
)

// fakeStore records what processRecoveredBackups writes. Only the methods
// recovery calls do anything; the rest satisfy types.MetadataStore.
type fakeStore struct {
	existing []types.BackupMeta
	created  map[string]*types.BackupMeta
	order    []string
	n        int
}

func newFakeStore() *fakeStore { return &fakeStore{created: map[string]*types.BackupMeta{}} }

func (f *fakeStore) CreateBackupMeta(serverName, serverType, database, backupType string) *types.BackupMeta {
	f.n++
	id := fmt.Sprintf("store-id-%d", f.n)
	m := &types.BackupMeta{ID: id, ServerName: serverName, ServerType: serverType, Database: database,
		BackupType: backupType, Status: types.StatusPending}
	f.created[id] = m
	f.order = append(f.order, id)
	return m
}

func (f *fakeStore) UpdateBackupStatus(id string, status types.BackupStatus, localPaths map[string]string, size int64, errorMsg string) error {
	m, ok := f.created[id]
	if !ok {
		return fmt.Errorf("backup %s not found", id)
	}
	m.Status, m.LocalPaths, m.Size, m.ErrorMessage = status, localPaths, size, errorMsg
	return nil
}

func (f *fakeStore) UpdateS3UploadStatus(id string, status types.BackupStatus, s3Keys map[string]string, errorMsg string) error {
	m, ok := f.created[id]
	if !ok {
		return fmt.Errorf("backup %s not found", id)
	}
	m.S3UploadStatus, m.S3Keys, m.S3UploadError = status, s3Keys, errorMsg
	return nil
}

func (f *fakeStore) SetBackupTimes(id string, createdAt, completedAt time.Time) error {
	m, ok := f.created[id]
	if !ok {
		return fmt.Errorf("backup %s not found", id)
	}
	m.CreatedAt = createdAt
	if !completedAt.IsZero() {
		m.CompletedAt = completedAt
	}
	return nil
}

func (f *fakeStore) GetBackups() []types.BackupMeta { return f.existing }
func (f *fakeStore) GetBackupsFiltered(string, string, string, bool) []types.BackupMeta {
	return nil
}
func (f *fakeStore) GetBackupByID(string) (types.BackupMeta, bool) { return types.BackupMeta{}, false }
func (f *fakeStore) MarkBackupDeleted(string) error                { return nil }
func (f *fakeStore) GetStats() map[string]interface{}              { return nil }
func (f *fakeStore) UpdateLogFilePath(string, string) error        { return nil }
func (f *fakeStore) PurgeDeletedBackups(time.Duration) int         { return 0 }
func (f *fakeStore) Load() error                                   { return nil }
func (f *fakeStore) Save() error                                   { return nil }

// withRecoveryConfig points config.CFG at a local root and S3 prefix, and resets
// the merge flag, for one test.
func withRecoveryConfig(t *testing.T, localRoot, s3Prefix string) {
	t.Helper()
	saved, savedMerge := config.CFG, *mergeMode
	t.Cleanup(func() { config.CFG, *mergeMode = saved, savedMerge })
	config.CFG.Local.Enabled = true
	config.CFG.Local.BackupDirectory = localRoot
	config.CFG.S3.Enabled = true
	config.CFG.S3.Prefix = s3Prefix
	config.CFG.S3.Bucket = "backup-bucket"
	*mergeMode = false
}

type wantBackup struct {
	server, db, backupType string
	at                     time.Time
	size                   int
	localPaths, s3Keys     map[string]string
}

// writeRealBackups lays out backups exactly where GoSQLGuard writes them, using
// backup.ConstructBackupPaths and backup.TimestampFormat, and returns what
// recovery should find plus the S3 keys (which are not on disk).
func writeRealBackups(t *testing.T) []wantBackup {
	t.Helper()
	base := time.Date(2026, 11, 22, 1, 2, 3, 0, time.UTC)
	specs := []struct{ server, db, backupType string }{
		{"server1", "testdb", "daily"},
		{"server1", "testdb", "hourly"},
		{"prod-db-01", "app_users", "hourly"},    // hyphenated server, underscored db
		{"mysql-a", "db-with-hyphens", "weekly"}, // hyphenated db
		{"legacy_srv", "orders", "monthly"},      // underscored server: by-type name is ambiguous
		{"srv", "2024-11-22", "manual"},          // db shaped like a date
	}

	var want []wantBackup
	for i, s := range specs {
		at := base.Add(time.Duration(i) * 37 * time.Minute)
		localPaths, s3Keys := backup.ConstructBackupPaths(s.server, s.backupType, s.db, at.Format(backup.TimestampFormat))
		require.Len(t, localPaths, 2)
		require.Len(t, s3Keys, 2)
		size := 100 + i
		for _, p := range localPaths {
			require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
			require.NoError(t, os.WriteFile(p, make([]byte, size), 0o600))
		}
		want = append(want, wantBackup{s.server, s.db, s.backupType, at, size, localPaths, s3Keys})
	}
	return want
}

// TestRecoveryRoundTrip is the regression test for cluster-services-1553: backups
// written by the real path constructor, locally and to S3, must all be recovered
// with the right server, database, type and timestamp, one metadata record per
// backup run holding every copy's path.
func TestRecoveryRoundTrip(t *testing.T) {
	root := filepath.Join(t.TempDir(), "backups")
	withRecoveryConfig(t, root, "gosqlguard")
	want := writeRealBackups(t)

	// Noise that must be ignored.
	for _, p := range []string{
		".metadata/by-server/s/daily/db-2026-11-22-01-02-03.sql.gz",
		"by-server/s/daily/notes.txt",
		"stray-2026-11-22-01-02-03.sql.gz",
	} {
		full := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
		require.NoError(t, os.WriteFile(full, []byte("x"), 0o600))
	}

	local := scanLocalStorage(root)
	require.Len(t, local, 2*len(want), "every by-server and by-type copy on disk")

	var s3Copies []RecoveredBackup
	prefix := backup.S3KeyPrefix()
	for _, w := range want {
		for _, k := range w.s3Keys {
			b, ok := recoverS3Object(k, prefix, "backup-bucket", int64(w.size), time.Time{})
			require.True(t, ok, "S3 key %s", k)
			s3Copies = append(s3Copies, b)
		}
	}
	_, ok := recoverS3Object("other-prefix/by-server/s/daily/db-2026-11-22-01-02-03.sql.gz", prefix, "b", 1, time.Time{})
	assert.False(t, ok, "keys outside the configured prefix are not backups")

	// Each parsed copy carries the right identity.
	byPath := map[string]RecoveredBackup{}
	for _, b := range append(append([]RecoveredBackup{}, local...), s3Copies...) {
		byPath[b.Path] = b
	}
	for _, w := range want {
		for org, p := range w.localPaths {
			b, ok := byPath[p]
			require.True(t, ok, "local copy %s not recovered", p)
			assert.Equal(t, org, b.Organization)
			assert.Equal(t, w.backupType, b.BackupType, p)
			assert.Equal(t, w.at.Format(backup.TimestampFormat), b.Timestamp, p)
			assert.True(t, w.at.Equal(b.CreatedAt), "%s: CreatedAt %v", p, b.CreatedAt)
			if org == backup.OrgByServer {
				assert.Equal(t, w.server, b.ServerName, p)
				assert.Equal(t, w.db, b.Database, p)
			}
		}
	}

	// Reconciled: one set per backup run, server/db exact even for the
	// underscored server (taken from the by-server copy).
	sets := reconcileBackups(append(local, s3Copies...))
	require.Len(t, sets, len(want))
	type id struct{ server, db, typ string }
	setBy := map[id]RecoveredBackupSet{}
	for _, s := range sets {
		setBy[id{s.ServerName, s.Database, s.BackupType}] = s
	}
	for _, w := range want {
		s, ok := setBy[id{w.server, w.db, w.backupType}]
		require.True(t, ok, "no recovered set for %s/%s/%s", w.server, w.db, w.backupType)
		assert.True(t, w.at.Equal(s.CreatedAt))
		assert.Equal(t, w.localPaths, s.LocalPaths)
		assert.Equal(t, w.s3Keys, s.S3Keys)
		assert.Equal(t, int64(w.size), s.Size)
	}

	// Processed into the store: the record IDs the store issued get the status
	// and paths (no name-derived ID the store never saw).
	store := newFakeStore()
	processRecoveredBackups(store, append(local, s3Copies...))
	require.Len(t, store.created, len(want))
	got := map[id]*types.BackupMeta{}
	for _, m := range store.created {
		got[id{m.ServerName, m.Database, m.BackupType}] = m
	}
	for _, w := range want {
		m, ok := got[id{w.server, w.db, w.backupType}]
		require.True(t, ok, "no metadata record for %s/%s/%s", w.server, w.db, w.backupType)
		assert.Equal(t, types.StatusSuccess, m.Status)
		assert.Equal(t, types.StatusSuccess, m.S3UploadStatus)
		assert.Equal(t, w.localPaths, m.LocalPaths)
		assert.Equal(t, w.s3Keys, m.S3Keys)
		assert.Equal(t, int64(w.size), m.Size)
		// cluster-services-1560: the record carries the backup's own time, not
		// the time recovery ran.
		assert.True(t, w.at.Equal(m.CreatedAt), "%s/%s/%s: CreatedAt %v, want %v", w.server, w.db, w.backupType, m.CreatedAt, w.at)
		assert.True(t, w.at.Equal(m.CompletedAt), "%s/%s/%s: CompletedAt %v, want %v", w.server, w.db, w.backupType, m.CompletedAt, w.at)
	}

	// Merge mode: a second pass over the same files adds nothing.
	*mergeMode = true
	again := newFakeStore()
	for _, m := range store.created {
		again.existing = append(again.existing, *m)
	}
	processRecoveredBackups(again, append(local, s3Copies...))
	assert.Empty(t, again.created, "merge mode must skip backups already in metadata")
}

// TestReconcileByTypeOnly: with no by-server copy, a by-type copy still recovers,
// split at the first "_" (the documented best guess).
func TestReconcileByTypeOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "backups")
	withRecoveryConfig(t, root, "")
	p := filepath.Join(root, "by-type", "daily", "srv1_app_db-2026-11-22-01-02-03.sql.gz")
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
	require.NoError(t, os.WriteFile(p, []byte("x"), 0o600))

	sets := reconcileBackups(scanLocalStorage(root))
	require.Len(t, sets, 1)
	assert.Equal(t, "srv1", sets[0].ServerName)
	assert.Equal(t, "app_db", sets[0].Database)
	assert.Equal(t, "daily", sets[0].BackupType)
	assert.Equal(t, map[string]string{backup.OrgByType: p}, sets[0].LocalPaths)
	assert.Empty(t, sets[0].S3Keys)
}

// TestOldFlatNamesAreNotBackups: the flat {server}-{db}-{type}-{ts} name the tool
// used to expect is not a layout GoSQLGuard writes.
func TestOldFlatNamesAreNotBackups(t *testing.T) {
	root := t.TempDir()
	withRecoveryConfig(t, root, "")
	p := filepath.Join(root, "hourly", "server1-db1-hourly-20250523-100000.sql.gz")
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
	require.NoError(t, os.WriteFile(p, []byte("x"), 0o600))
	assert.Empty(t, scanLocalStorage(root))
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
