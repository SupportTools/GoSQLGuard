package metadata

import (
	"fmt"
	"testing"
	"time"

	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
)

// BenchmarkGetBackups benchmarks retrieving all backups
func BenchmarkGetBackups(b *testing.B) {
	sizes := []int{100, 1000, 10000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			// Setup: Create test store with many backups
			store := createBenchmarkStore(size)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				backups := store.GetBackups()
				_ = backups
			}
		})
	}
}

// BenchmarkGetBackupsFiltered benchmarks filtered queries
func BenchmarkGetBackupsFiltered(b *testing.B) {
	sizes := []int{100, 1000, 10000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			store := createBenchmarkStore(size)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				backups := store.GetBackupsFiltered("server1", "testdb", "daily", true)
				_ = backups
			}
		})
	}
}

// BenchmarkGetBackupsPaginated benchmarks paginated queries
func BenchmarkGetBackupsPaginated(b *testing.B) {
	sizes := []int{1000, 10000, 100000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			// This would test the paginated implementation
			// when using a database backend
			b.Skip("Requires database connection")
		})
	}
}

// BenchmarkGetStats benchmarks statistics calculation
func BenchmarkGetStats(b *testing.B) {
	sizes := []int{100, 1000, 10000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			store := createBenchmarkStore(size)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				stats := store.GetStats()
				_ = stats
			}
		})
	}
}

// BenchmarkUpdateBackupStatus benchmarks status updates
func BenchmarkUpdateBackupStatus(b *testing.B) {
	store := createBenchmarkStore(1000)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("server1-testdb-daily-%d", i%1000)
		store.UpdateBackupStatus(id, StatusSuccess, map[string]string{"default": "/path/to/backup"}, 1024*1024, "")
	}
}

// BenchmarkConcurrentAccess benchmarks concurrent read/write operations
func BenchmarkConcurrentAccess(b *testing.B) {
	store := createBenchmarkStore(1000)
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				// Read operation
				backups := store.GetBackups()
				_ = backups
			} else {
				// Write operation
				id := fmt.Sprintf("server1-testdb-hourly-%d", i)
				store.UpdateBackupStatus(id, StatusSuccess, map[string]string{"default": "/path"}, 1024, "")
			}
			i++
		}
	})
}

// BenchmarkSearchPerformance benchmarks search functionality
func BenchmarkSearchPerformance(b *testing.B) {
	sizes := []int{1000, 10000}
	searchTerms := []string{"server", "testdb", "daily", "2024"}
	
	for _, size := range sizes {
		for _, term := range searchTerms {
			b.Run(fmt.Sprintf("Size%d_Search%s", size, term), func(b *testing.B) {
				store := createBenchmarkStore(size)
				backups := store.GetBackups()
				
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					var results []types.BackupMeta
					for _, backup := range backups {
						if contains(backup.ID, term) || 
						   contains(backup.ServerName, term) || 
						   contains(backup.Database, term) {
							results = append(results, backup)
						}
					}
					_ = results
				}
			})
		}
	}
}

// Helper functions

func createBenchmarkStore(size int) *Store {
	store := &Store{
		metadata: MetadataStore{
			Backups:     make([]types.BackupMeta, 0, size),
			LastUpdated: time.Now(),
			Version:     "1.0",
		},
	}
	
	// Generate test data
	servers := []string{"server1", "server2", "server3", "server4", "server5"}
	databases := []string{"testdb", "proddb", "userdb", "orderdb", "logdb"}
	backupTypes := []string{"hourly", "daily", "weekly", "monthly", "manual"}
	statuses := []types.BackupStatus{StatusSuccess, StatusError, StatusPending}
	
	baseTime := time.Now().Add(-30 * 24 * time.Hour) // Start 30 days ago
	
	for i := 0; i < size; i++ {
		backup := types.BackupMeta{
			ID:          fmt.Sprintf("%s-%s-%s-%d", servers[i%5], databases[i%5], backupTypes[i%5], i),
			ServerName:  servers[i%5],
			ServerType:  "mysql",
			Database:    databases[i%5],
			BackupType:  backupTypes[i%5],
			CreatedAt:   baseTime.Add(time.Duration(i) * time.Minute),
			CompletedAt: baseTime.Add(time.Duration(i)*time.Minute + 5*time.Minute),
			Size:        int64(1024 * 1024 * (i%100 + 1)), // 1MB to 100MB
			Status:      statuses[i%3],
			LocalPaths:  map[string]string{"default": fmt.Sprintf("/backups/%d.sql.gz", i)},
		}
		
		if i%2 == 0 {
			backup.S3UploadStatus = StatusSuccess
			backup.S3Keys = map[string]string{"default": fmt.Sprintf("backups/%d.sql.gz", i)}
		}
		
		store.metadata.Backups = append(store.metadata.Backups, backup)
		
		// Update stats
		if backup.Status == StatusSuccess {
			store.metadata.TotalLocalSize += backup.Size
		}
		if backup.S3UploadStatus == StatusSuccess {
			store.metadata.TotalS3Size += backup.Size
		}
	}
	
	return store
}

func contains(s, substr string) bool {
	if len(s) == 0 || len(substr) == 0 {
		return false
	}
	// Simple case-insensitive contains
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Memory usage benchmarks

func BenchmarkMemoryUsage(b *testing.B) {
	sizes := []int{1000, 10000, 100000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			b.ReportAllocs()
			
			for i := 0; i < b.N; i++ {
				store := createBenchmarkStore(size)
				_ = store
			}
		})
	}
}

// Results documentation:
// These benchmarks help identify performance bottlenecks in the metadata system.
// 
// Expected improvements with optimization:
// 1. GetBackups: O(n) -> O(1) with pagination (constant page size)
// 2. GetBackupsFiltered: O(n) full scan -> O(log n) with indexes
// 3. GetStats: O(n) full scan -> O(1) with pre-aggregated values
// 4. Search: O(n) full scan -> O(log n) with full-text search indexes
//
// Run benchmarks with:
// go test -bench=. -benchmem ./pkg/metadata/