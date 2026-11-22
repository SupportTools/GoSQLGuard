package types

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// NewBackupID returns the metadata ID for a new backup:
// {server}-{database}-{type}-{YYYYMMDD-HHMMSS}-{8 hex chars}.
//
// The ID used to end at the timestamp, which only has one-second resolution, so two
// backups of the same server/database/type started within one second (a manual
// backup next to a scheduled one, or a fast loop) got the same ID. The file store
// then updated the first record twice and left the second pending forever, and the
// database store failed the second INSERT on the primary key. The random suffix
// keeps IDs unique without changing the readable prefix.
func NewBackupID(serverName, database, backupType string, now time.Time) string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand does not fail on supported platforms; fall back to the
		// sub-second clock rather than returning a colliding ID.
		return fmt.Sprintf("%s-%s-%s-%s-%09d", serverName, database, backupType,
			now.Format("20060102-150405"), now.Nanosecond())
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", serverName, database, backupType,
		now.Format("20060102-150405"), hex.EncodeToString(b[:]))
}
