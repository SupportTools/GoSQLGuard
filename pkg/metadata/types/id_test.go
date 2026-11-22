package types

import (
	"regexp"
	"testing"
	"time"
)

func TestNewBackupIDUniqueWithinOneSecond(t *testing.T) {
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := NewBackupID("server", "db", "daily", now)
		if seen[id] {
			t.Fatalf("duplicate ID %q after %d calls with the same timestamp", id, i)
		}
		seen[id] = true
	}
}

func TestNewBackupIDFormat(t *testing.T) {
	now := time.Date(2026, 11, 21, 12, 34, 56, 0, time.UTC)
	id := NewBackupID("prod-server", "users", "hourly", now)
	if !regexp.MustCompile(`^prod-server-users-hourly-20261121-123456-[0-9a-f]{8}$`).MatchString(id) {
		t.Fatalf("unexpected ID format: %q", id)
	}
}
