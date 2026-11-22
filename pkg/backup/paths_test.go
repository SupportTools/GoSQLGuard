package backup

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/supporttools/GoSQLGuard/pkg/config"
)

// withBackupConfig points config.CFG at a local root and an S3 prefix for one test.
func withBackupConfig(t *testing.T, localRoot, s3Prefix string) {
	t.Helper()
	saved := config.CFG
	t.Cleanup(func() { config.CFG = saved })
	config.CFG.Local.Enabled = true
	config.CFG.Local.BackupDirectory = localRoot
	config.CFG.S3.Enabled = true
	config.CFG.S3.Prefix = s3Prefix
}

// TestParseBackupPathRoundTrip parses every path ConstructBackupPaths produces and
// expects the inputs back, for names that contain the separators the layout uses.
func TestParseBackupPathRoundTrip(t *testing.T) {
	root := filepath.Join(t.TempDir(), "backups")
	withBackupConfig(t, root, "gosqlguard") // no trailing slash: S3KeyPrefix adds it

	when := time.Date(2026, 11, 22, 7, 5, 9, 0, time.UTC)
	cases := []struct{ server, backupType, db string }{
		{"server1", "daily", "testdb"},
		{"prod-db-01", "hourly", "app_users"},
		{"mysql-a", "weekly", "db-with-hyphens"},
		{"mysql-a", "manual", "2024-11-22"}, // database name shaped like a date
		{"srv", "custom-type", "x"},
	}

	for _, c := range cases {
		ts := when.Format(TimestampFormat)
		localPaths, s3Keys := ConstructBackupPaths(c.server, c.backupType, c.db, ts)
		require.Len(t, localPaths, 2)
		require.Len(t, s3Keys, 2)

		check := func(org, rel string) {
			t.Helper()
			got, ok := ParseBackupPath(rel)
			require.True(t, ok, "ParseBackupPath(%q)", rel)
			assert.Equal(t, org, got.Organization, rel)
			assert.Equal(t, c.server, got.ServerName, rel)
			assert.Equal(t, c.db, got.Database, rel)
			assert.Equal(t, c.backupType, got.BackupType, rel)
			assert.Equal(t, ts, got.Timestamp, rel)
			assert.True(t, when.Equal(got.CreatedAt), "%s: CreatedAt %v", rel, got.CreatedAt)
			assert.Equal(t, c.server+"_"+c.db, got.ServerDatabase, rel)
		}

		for org, p := range localPaths {
			rel, err := filepath.Rel(root, p)
			require.NoError(t, err)
			check(org, filepath.ToSlash(rel))
		}
		for org, k := range s3Keys {
			require.True(t, strings.HasPrefix(k, "gosqlguard/"), k)
			check(org, strings.TrimPrefix(k, "gosqlguard/"))
		}
	}
}

// TestParseBackupPathByTypeUnderscoreServer documents the one ambiguity in the
// layout: a by-type file name joins server and database with "_", so a server
// name containing "_" cannot be told apart from the name alone. The split takes
// the first "_"; ServerDatabase keeps the exact stem for callers that match it
// against the by-server copy.
func TestParseBackupPathByTypeUnderscoreServer(t *testing.T) {
	got, ok := ParseBackupPath("by-type/daily/legacy_srv_orders-2026-11-22-01-02-03.sql.gz")
	require.True(t, ok)
	assert.Equal(t, "legacy", got.ServerName)
	assert.Equal(t, "srv_orders", got.Database)
	assert.Equal(t, "legacy_srv_orders", got.ServerDatabase)

	got, ok = ParseBackupPath("by-server/legacy_srv/daily/orders-2026-11-22-01-02-03.sql.gz")
	require.True(t, ok)
	assert.Equal(t, "legacy_srv", got.ServerName)
	assert.Equal(t, "orders", got.Database)
	assert.Equal(t, "legacy_srv_orders", got.ServerDatabase)
}

func TestParseBackupPathRejects(t *testing.T) {
	for _, rel := range []string{
		"",
		"db-2026-11-22-01-02-03.sql.gz", // no organization
		"by-server/srv/db-2026-11-22-01-02-03.sql.gz",             // missing type dir
		"by-server/srv/daily/extra/db-2026-11-22-01-02-03.sql.gz", // too deep
		"by-server/srv/daily/db-2026-11-22-01-02-03.sql",          // wrong suffix
		"by-server/srv/daily/db-20261122-010203.sql.gz",           // old timestamp layout
		"by-server/srv/daily/db-2026-13-22-01-02-03.sql.gz",       // month 13
		"by-server/srv/daily/-2026-11-22-01-02-03.sql.gz",         // empty database
		"by-server/srv/daily/db_2026-11-22-01-02-03.sql.gz",       // no "-" before timestamp
		"by-server//daily/db-2026-11-22-01-02-03.sql.gz",          // empty server
		"by-type/daily/nounderscore-2026-11-22-01-02-03.sql.gz",   // no server prefix
		"by-type/daily/_db-2026-11-22-01-02-03.sql.gz",            // empty server
		"by-type/daily/srv_-2026-11-22-01-02-03.sql.gz",           // empty database
		"other/srv/daily/db-2026-11-22-01-02-03.sql.gz",           // unknown organization
		"server1-database1-hourly-20250523-120000.sql.gz",         // the format recovery used to expect
		".metadata/by-type/daily/s_db-2026-11-22-01-02-03.sql.gz", // not under the root layout
	} {
		_, ok := ParseBackupPath(rel)
		assert.False(t, ok, "ParseBackupPath(%q) should be rejected", rel)
	}
}
