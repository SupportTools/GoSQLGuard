package config

import (
	"fmt"
	"time"
)

// LocalRetention returns the human-readable local retention policy and the
// expiry time for a backup of backupType created at createdAt, from
// CFG.BackupTypes. An unknown type or disabled local storage yields ("", zero);
// "keep forever" yields a zero expiry; an unparsable duration yields
// "Unknown retention policy" and a zero expiry.
func LocalRetention(backupType string, createdAt time.Time) (string, time.Time) {
	typeConfig, exists := CFG.BackupTypes[backupType]
	if !exists || !typeConfig.Local.Enabled {
		return "", time.Time{}
	}
	if typeConfig.Local.Retention.Forever {
		return "Keep forever", time.Time{}
	}
	duration, err := time.ParseDuration(typeConfig.Local.Retention.Duration)
	if err != nil {
		return "Unknown retention policy", time.Time{}
	}
	expiresAt := createdAt.Add(duration)
	return fmt.Sprintf("Keep for %s (until %s)",
		typeConfig.Local.Retention.Duration,
		expiresAt.Format("2006-01-02 15:04:05")), expiresAt
}
