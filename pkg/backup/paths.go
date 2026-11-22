package backup

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/config"
)

// TimestampFormat is the time layout embedded in every backup file name.
// backupDatabase formats the backup start time with it, and ParseBackupPath
// reads it back; the two must stay in step.
const TimestampFormat = "2006-01-02-15-04-05"

// Organization names used as keys of the path maps ConstructBackupPaths returns.
const (
	OrgByServer = "by-server"
	OrgByType   = "by-type"
)

const backupFileSuffix = ".sql.gz"

// ConstructBackupPaths returns the local paths and S3 keys a backup is written to,
// keyed by organization:
//
//	by-server: <root>/by-server/<server>/<type>/<db>-<timestamp>.sql.gz
//	by-type:   <root>/by-type/<type>/<server>_<db>-<timestamp>.sql.gz
//
// <root> is config.CFG.Local.BackupDirectory for local paths and the S3 prefix for
// S3 keys. A store that is disabled in the configuration gets no entries.
// ParseBackupPath is the inverse.
func ConstructBackupPaths(serverName, backupType, dbName, timestamp string) (map[string]string, map[string]string) {
	localPaths := make(map[string]string)
	s3Keys := make(map[string]string)

	// Base file name (without server prefix)
	filename := fmt.Sprintf("%s-%s%s", dbName, timestamp, backupFileSuffix)

	// Server-prefixed filename for by-type organization
	serverPrefixedFilename := fmt.Sprintf("%s_%s", serverName, filename)

	if config.CFG.Local.Enabled {
		localPaths[OrgByServer] = filepath.Join(
			config.CFG.Local.BackupDirectory, OrgByServer, serverName, backupType, filename)
		localPaths[OrgByType] = filepath.Join(
			config.CFG.Local.BackupDirectory, OrgByType, backupType, serverPrefixedFilename)
	}

	if config.CFG.S3.Enabled {
		prefix := S3KeyPrefix()
		s3Keys[OrgByServer] = fmt.Sprintf("%s%s/%s/%s/%s", prefix, OrgByServer, serverName, backupType, filename)
		s3Keys[OrgByType] = fmt.Sprintf("%s%s/%s/%s", prefix, OrgByType, backupType, serverPrefixedFilename)
	}

	return localPaths, s3Keys
}

// S3KeyPrefix returns the configured S3 prefix normalized to end in "/" (or "" when unset).
func S3KeyPrefix() string {
	prefix := config.CFG.S3.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix
}

// ParsedBackupPath is what ParseBackupPath recovers from a backup's location.
type ParsedBackupPath struct {
	Organization string // OrgByServer or OrgByType
	ServerName   string
	Database     string
	BackupType   string
	Timestamp    string // raw, in TimestampFormat
	CreatedAt    time.Time

	// ServerDatabase is the "<server>_<database>" stem of a by-type file name.
	// Both names may contain "_", so the split into ServerName/Database is a
	// guess for by-type paths (first "_" wins). Callers that also see the
	// by-server copy should match on this field instead of trusting the split.
	ServerDatabase string
}

// ParseBackupPath parses a backup location relative to the storage root (the local
// backup directory or the S3 prefix), using "/" separators. It accepts exactly the
// two layouts ConstructBackupPaths produces and reports false for anything else.
func ParseBackupPath(rel string) (ParsedBackupPath, bool) {
	parts := strings.Split(strings.TrimPrefix(rel, "/"), "/")
	for _, p := range parts {
		if p == "" {
			return ParsedBackupPath{}, false
		}
	}

	switch {
	case len(parts) == 4 && parts[0] == OrgByServer:
		db, ts, createdAt, ok := splitBackupFileName(parts[3])
		if !ok {
			return ParsedBackupPath{}, false
		}
		return ParsedBackupPath{
			Organization:   OrgByServer,
			ServerName:     parts[1],
			Database:       db,
			BackupType:     parts[2],
			Timestamp:      ts,
			CreatedAt:      createdAt,
			ServerDatabase: parts[1] + "_" + db,
		}, true

	case len(parts) == 3 && parts[0] == OrgByType:
		stem, ts, createdAt, ok := splitBackupFileName(parts[2])
		if !ok {
			return ParsedBackupPath{}, false
		}
		server, db, found := strings.Cut(stem, "_")
		if !found || server == "" || db == "" {
			return ParsedBackupPath{}, false
		}
		return ParsedBackupPath{
			Organization:   OrgByType,
			ServerName:     server,
			Database:       db,
			BackupType:     parts[1],
			Timestamp:      ts,
			CreatedAt:      createdAt,
			ServerDatabase: stem,
		}, true
	}

	return ParsedBackupPath{}, false
}

// splitBackupFileName splits "<stem>-<timestamp>.sql.gz" into stem and timestamp.
// The timestamp has a fixed width, so a stem containing "-" is unambiguous.
func splitBackupFileName(name string) (stem, ts string, createdAt time.Time, ok bool) {
	base, found := strings.CutSuffix(name, backupFileSuffix)
	if !found || len(base) < len(TimestampFormat)+2 {
		return "", "", time.Time{}, false
	}
	cut := len(base) - len(TimestampFormat)
	if base[cut-1] != '-' {
		return "", "", time.Time{}, false
	}
	stem, ts = base[:cut-1], base[cut:]
	createdAt, err := time.Parse(TimestampFormat, ts)
	if err != nil {
		return "", "", time.Time{}, false
	}
	return stem, ts, createdAt, true
}
