// metadata-recovery is a command-line tool to reconstruct GoSQLGuard metadata from existing backup files
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/supporttools/GoSQLGuard/pkg/backup"
	"github.com/supporttools/GoSQLGuard/pkg/config"
	"github.com/supporttools/GoSQLGuard/pkg/metadata"
	"github.com/supporttools/GoSQLGuard/pkg/metadata/types"
)

var (
	// Flags
	dryRun       = flag.Bool("dry-run", false, "Perform a dry run without writing metadata")
	verbose      = flag.Bool("verbose", false, "Enable verbose logging")
	scanLocal    = flag.Bool("local", true, "Scan local storage for backups")
	scanS3       = flag.Bool("s3", true, "Scan S3 storage for backups")
	forceRebuild = flag.Bool("force", false, "Force rebuild even if metadata exists")
	mergeMode    = flag.Bool("merge", false, "Merge with existing metadata instead of replacing")
)

// RecoveredBackup is one backup file (one copy) found during recovery.
//
// GoSQLGuard writes every backup twice per store, in the layouts
// backup.ConstructBackupPaths defines (by-server and by-type); the location is
// parsed with backup.ParseBackupPath, its inverse.
type RecoveredBackup struct {
	Filename     string
	Path         string // absolute local path, or the S3 key
	Size         int64
	ModTime      time.Time
	Organization string // backup.OrgByServer or backup.OrgByType
	ServerName   string
	Database     string
	BackupType   string
	Timestamp    string // raw, in backup.TimestampFormat
	CreatedAt    time.Time
	IsS3         bool
	S3Bucket     string
	S3Key        string

	// serverDatabase is the "<server>_<database>" stem; see backup.ParsedBackupPath.
	serverDatabase string
}

// RecoveredBackupSet is one backup run: every copy of the same dump, grouped
// across organizations and stores, with paths keyed the way the backup code
// records them (organization -> absolute local path / S3 key).
type RecoveredBackupSet struct {
	ServerName string
	Database   string
	BackupType string
	Timestamp  string
	CreatedAt  time.Time
	Size       int64
	LocalPaths map[string]string
	S3Keys     map[string]string
}

func main() {
	flag.Parse()

	// Load configuration from MySQL
	config.LoadConfiguration()

	// Initialize metadata system
	if err := metadata.Initialize(); err != nil {
		log.Fatalf("Failed to initialize metadata: %v", err)
	}

	// Check if metadata already exists
	existingBackups := metadata.DefaultStore.GetBackups()
	if len(existingBackups) > 0 && !*forceRebuild && !*mergeMode {
		log.Printf("Found existing metadata with %d backups. Use -force to rebuild or -merge to merge.", len(existingBackups))
		os.Exit(0)
	}

	log.Println("Starting metadata recovery process...")

	// Scan for backups
	var recoveredBackups []RecoveredBackup

	if *scanLocal && config.CFG.Local.Enabled {
		localBackups := scanLocalStorage(config.CFG.Local.BackupDirectory)
		recoveredBackups = append(recoveredBackups, localBackups...)
		log.Printf("Found %d backups in local storage", len(localBackups))
	}

	if *scanS3 && config.CFG.S3.Enabled {
		s3Backups := scanS3Storage()
		recoveredBackups = append(recoveredBackups, s3Backups...)
		log.Printf("Found %d backups in S3 storage", len(s3Backups))
	}

	// Process recovered backups
	processRecoveredBackups(metadata.DefaultStore, recoveredBackups)

	// Summary
	stats := metadata.DefaultStore.GetStats()
	log.Printf("\nRecovery Summary:")
	statusCounts, _ := stats["statusCounts"].(map[string]int)
	log.Printf("- Total backups recovered: %v", stats["totalCount"])
	log.Printf("- Successful backups: %d", statusCounts["success"])
	log.Printf("- Failed backups: %d", statusCounts["error"])
	log.Printf("- Total local size: %s", formatBytes(stats["totalLocalSize"].(int64)))
	log.Printf("- Total S3 size: %s", formatBytes(stats["totalS3Size"].(int64)))

	if !*dryRun {
		if err := metadata.DefaultStore.Save(); err != nil {
			log.Fatalf("Failed to save metadata: %v", err)
		}
		log.Println("Metadata saved successfully!")
	} else {
		log.Println("Dry run completed - no changes were saved")
	}
}

// scanLocalStorage scans backupDir on the local filesystem for backup files.
// The directory is a parameter (rather than read from config.CFG inside) so the
// scan can be exercised without loading the full application configuration.
func scanLocalStorage(backupDir string) []RecoveredBackup {
	var backups []RecoveredBackup

	err := filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if *verbose {
				log.Printf("Error accessing path %s: %v", path, err)
			}
			return nil // Continue walking
		}

		// Skip directories and non-backup files
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".sql.gz") {
			return nil
		}

		// Skip metadata directory
		if strings.Contains(path, ".metadata") {
			return nil
		}

		rel, err := filepath.Rel(backupDir, path)
		if err != nil {
			return nil
		}
		parsed, ok := backup.ParseBackupPath(filepath.ToSlash(rel))
		if !ok {
			if *verbose {
				log.Printf("Skipping file outside the backup layout: %s", path)
			}
			return nil
		}

		b := newRecoveredBackup(parsed, info.Name(), path, info.Size(), info.ModTime())
		backups = append(backups, b)
		return nil
	})

	if err != nil {
		log.Printf("Error walking backup directory: %v", err)
	}

	return backups
}

// scanS3Storage scans S3 bucket for backup files
func scanS3Storage() []RecoveredBackup {
	var backups []RecoveredBackup

	// Create S3 session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(config.CFG.S3.Region),
		Credentials: credentials.NewStaticCredentials(
			config.CFG.S3.AccessKey,
			config.CFG.S3.SecretKey,
			"",
		),
		Endpoint:         aws.String(config.CFG.S3.Endpoint),
		S3ForcePathStyle: aws.Bool(config.CFG.S3.PathStyle),
	})
	if err != nil {
		log.Printf("Failed to create S3 session: %v", err)
		return backups
	}

	svc := s3.New(sess)

	// List objects under the prefix the backup code writes to
	prefix := backup.S3KeyPrefix()
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(config.CFG.S3.Bucket),
		Prefix: aws.String(prefix),
	}

	err = svc.ListObjectsV2Pages(params, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			key := *obj.Key

			// Skip non-backup files
			if !strings.HasSuffix(key, ".sql.gz") {
				continue
			}

			var modTime time.Time
			if obj.LastModified != nil {
				modTime = *obj.LastModified
			}
			b, ok := recoverS3Object(key, prefix, config.CFG.S3.Bucket, aws.Int64Value(obj.Size), modTime)
			if !ok {
				if *verbose {
					log.Printf("Skipping S3 object outside the backup layout: %s", key)
				}
				continue
			}
			backups = append(backups, b)
		}
		return true // Continue to next page
	})

	if err != nil {
		log.Printf("Error listing S3 objects: %v", err)
	}

	return backups
}

// recoverS3Object parses one S3 object key (prefix included) into a RecoveredBackup.
func recoverS3Object(key, prefix, bucket string, size int64, modTime time.Time) (RecoveredBackup, bool) {
	if !strings.HasPrefix(key, prefix) {
		return RecoveredBackup{}, false
	}
	parsed, ok := backup.ParseBackupPath(strings.TrimPrefix(key, prefix))
	if !ok {
		return RecoveredBackup{}, false
	}
	b := newRecoveredBackup(parsed, filepath.Base(key), key, size, modTime)
	b.IsS3 = true
	b.S3Bucket = bucket
	b.S3Key = key
	return b, true
}

// newRecoveredBackup builds a RecoveredBackup from a parsed location.
func newRecoveredBackup(p backup.ParsedBackupPath, filename, path string, size int64, modTime time.Time) RecoveredBackup {
	return RecoveredBackup{
		Filename:       filename,
		Path:           path,
		Size:           size,
		ModTime:        modTime,
		Organization:   p.Organization,
		ServerName:     p.ServerName,
		Database:       p.Database,
		BackupType:     p.BackupType,
		Timestamp:      p.Timestamp,
		CreatedAt:      p.CreatedAt,
		serverDatabase: p.ServerDatabase,
	}
}

// reconcileBackups groups the copies of each backup run into one RecoveredBackupSet.
//
// Copies belong together when backup type, timestamp and "<server>_<database>"
// agree. Server and database come from a by-server copy when there is one,
// because its directory names the server exactly; a by-type file name only
// carries "<server>_<database>", which is split at the first "_" when no
// by-server copy exists. Sets are returned sorted by server, database, type and
// timestamp so the output is deterministic.
func reconcileBackups(backups []RecoveredBackup) []RecoveredBackupSet {
	type key struct{ backupType, timestamp, serverDatabase string }
	sets := make(map[key]*RecoveredBackupSet)

	// by-server copies first, so they fix each set's server and database.
	ordered := make([]RecoveredBackup, 0, len(backups))
	for _, b := range backups {
		if b.Organization == backup.OrgByServer {
			ordered = append(ordered, b)
		}
	}
	for _, b := range backups {
		if b.Organization != backup.OrgByServer {
			ordered = append(ordered, b)
		}
	}

	for _, b := range ordered {
		k := key{b.BackupType, b.Timestamp, b.serverDatabase}
		set, ok := sets[k]
		if !ok {
			set = &RecoveredBackupSet{
				ServerName: b.ServerName,
				Database:   b.Database,
				BackupType: b.BackupType,
				Timestamp:  b.Timestamp,
				CreatedAt:  b.CreatedAt,
				LocalPaths: make(map[string]string),
				S3Keys:     make(map[string]string),
			}
			sets[k] = set
		}
		if b.Size > set.Size {
			set.Size = b.Size
		}
		if b.IsS3 {
			set.S3Keys[b.Organization] = b.S3Key
		} else {
			set.LocalPaths[b.Organization] = b.Path
		}
	}

	result := make([]RecoveredBackupSet, 0, len(sets))
	for _, set := range sets {
		result = append(result, *set)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.ServerName != b.ServerName {
			return a.ServerName < b.ServerName
		}
		if a.Database != b.Database {
			return a.Database < b.Database
		}
		if a.BackupType != b.BackupType {
			return a.BackupType < b.BackupType
		}
		return a.Timestamp < b.Timestamp
	})
	return result
}

// processRecoveredBackups writes one metadata record per recovered backup run.
//
// The record ID is the one the store generates. (Recovery used to overwrite it
// on the returned struct with a name-derived ID, which the store never saw, so
// the status update that followed targeted an ID that did not exist.)
// In merge mode a run is skipped when an existing record already points at any
// of its local paths or S3 keys.
func processRecoveredBackups(store types.MetadataStore, backups []RecoveredBackup) {
	known := make(map[string]bool)
	if *mergeMode {
		for _, existing := range store.GetBackups() {
			for _, p := range existing.LocalPaths {
				known[p] = true
			}
			if existing.LocalPath != "" {
				known[existing.LocalPath] = true
			}
			for _, k := range existing.S3Keys {
				known[k] = true
			}
		}
	}

	for _, set := range reconcileBackups(backups) {
		label := fmt.Sprintf("%s/%s/%s@%s", set.ServerName, set.Database, set.BackupType, set.Timestamp)

		if alreadyKnown(known, set) {
			if *verbose {
				log.Printf("Skipping backup already in metadata: %s", label)
			}
			continue
		}

		meta := store.CreateBackupMeta(set.ServerName, "mysql", set.Database, set.BackupType)

		if err := store.UpdateBackupStatus(meta.ID, types.StatusSuccess, set.LocalPaths, set.Size, ""); err != nil {
			log.Printf("Error updating backup status for %s: %v", label, err)
		}

		if len(set.S3Keys) > 0 {
			if err := store.UpdateS3UploadStatus(meta.ID, types.StatusSuccess, set.S3Keys, ""); err != nil {
				log.Printf("Error updating S3 upload status for %s: %v", label, err)
			}
		}

		if *verbose {
			log.Printf("Recovered backup %s (created %s) as %s", label, set.CreatedAt.Format(time.RFC3339), meta.ID)
		}
	}
}

func alreadyKnown(known map[string]bool, set RecoveredBackupSet) bool {
	for _, p := range set.LocalPaths {
		if known[p] {
			return true
		}
	}
	for _, k := range set.S3Keys {
		if known[k] {
			return true
		}
	}
	return false
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
