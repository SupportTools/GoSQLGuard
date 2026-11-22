package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLocalRetention(t *testing.T) {
	saved := CFG
	t.Cleanup(func() { CFG = saved })
	CFG.BackupTypes = map[string]BackupTypeConfig{
		"hourly":  {Local: LocalBackupConfig{Enabled: true, Retention: RetentionRule{Duration: "24h"}}},
		"forever": {Local: LocalBackupConfig{Enabled: true, Retention: RetentionRule{Forever: true}}},
		"bad":     {Local: LocalBackupConfig{Enabled: true, Retention: RetentionRule{Duration: "soon"}}},
		"s3only":  {S3: S3BackupConfig{Enabled: true, Retention: RetentionRule{Duration: "24h"}}},
	}
	at := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

	text, exp := LocalRetention("hourly", at)
	assert.Equal(t, at.Add(24*time.Hour), exp, "expiry counts from the given time")
	assert.Equal(t, "Keep for 24h (until 2026-11-02 10:00:00)", text)

	text, exp = LocalRetention("forever", at)
	assert.Equal(t, "Keep forever", text)
	assert.True(t, exp.IsZero())

	text, exp = LocalRetention("bad", at)
	assert.Equal(t, "Unknown retention policy", text)
	assert.True(t, exp.IsZero())

	for _, typ := range []string{"s3only", "missing"} {
		text, exp = LocalRetention(typ, at)
		assert.Empty(t, text, typ)
		assert.True(t, exp.IsZero(), typ)
	}
}
