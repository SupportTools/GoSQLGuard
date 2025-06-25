// metadata-recovery is a command-line tool to reconstruct GoSQLGuard metadata from existing backup files
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
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
	
	// Regex pattern for backup filenames
	// Format: {server}-{database}-{type}-{timestamp}.sql.gz
	backupFilePattern = regexp.MustCompile(`^(.+?)-(.+?)-(hourly|daily|weekly|monthly|yearly|manual)-(\d{8}-\d{6})\.sql\.gz$`)
)

// RecoveredBackup represents a backup found during recovery
type RecoveredBackup struct {
	Filename   string
	Path       string
	Size       int64
	ModTime    time.Time
	ServerName string
	Database   string
	BackupType string
	Timestamp  string
	IsS3       bool
	S3Bucket   string
	S3Key      string
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
		localBackups := scanLocalStorage()
		recoveredBackups = append(recoveredBackups, localBackups...)
		log.Printf("Found %d backups in local storage", len(localBackups))
	}

	if *scanS3 && config.CFG.S3.Enabled {
		s3Backups := scanS3Storage()
		recoveredBackups = append(recoveredBackups, s3Backups...)
		log.Printf("Found %d backups in S3 storage", len(s3Backups))
	}

	// Process recovered backups
	processRecoveredBackups(recoveredBackups)

	// Summary
	stats := metadata.DefaultStore.GetStats()
	log.Printf("\nRecovery Summary:")
	log.Printf("- Total backups recovered: %d", stats["totalBackups"])
	log.Printf("- Successful backups: %d", stats["successCount"])
	log.Printf("- Failed backups: %d", stats["errorCount"])
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

// scanLocalStorage scans local filesystem for backup files
func scanLocalStorage() []RecoveredBackup {
	var backups []RecoveredBackup
	backupDir := config.CFG.Local.BackupDirectory

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

		// Parse filename
		matches := backupFilePattern.FindStringSubmatch(info.Name())
		if matches == nil {
			if *verbose {
				log.Printf("Skipping file with non-standard name: %s", info.Name())
			}
			return nil
		}

		backup := RecoveredBackup{
			Filename:   info.Name(),
			Path:       path,
			Size:       info.Size(),
			ModTime:    info.ModTime(),
			ServerName: matches[1],
			Database:   matches[2],
			BackupType: matches[3],
			Timestamp:  matches[4],
			IsS3:       false,
		}

		backups = append(backups, backup)
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

	// List objects in bucket
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(config.CFG.S3.Bucket),
		Prefix: aws.String(config.CFG.S3.Prefix),
	}

	err = svc.ListObjectsV2Pages(params, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			key := *obj.Key
			
			// Skip non-backup files
			if !strings.HasSuffix(key, ".sql.gz") {
				continue
			}

			// Extract filename from key
			filename := filepath.Base(key)
			
			// Parse filename
			matches := backupFilePattern.FindStringSubmatch(filename)
			if matches == nil {
				if *verbose {
					log.Printf("Skipping S3 object with non-standard name: %s", key)
				}
				continue
			}

			backup := RecoveredBackup{
				Filename:   filename,
				Path:       key,
				Size:       *obj.Size,
				ModTime:    *obj.LastModified,
				ServerName: matches[1],
				Database:   matches[2],
				BackupType: matches[3],
				Timestamp:  matches[4],
				IsS3:       true,
				S3Bucket:   config.CFG.S3.Bucket,
				S3Key:      key,
			}

			backups = append(backups, backup)
		}
		return true // Continue to next page
	})

	if err != nil {
		log.Printf("Error listing S3 objects: %v", err)
	}

	return backups
}

// processRecoveredBackups processes recovered backups and updates metadata
func processRecoveredBackups(backups []RecoveredBackup) {
	existingIDs := make(map[string]bool)
	
	// Get existing backup IDs if in merge mode
	if *mergeMode {
		for _, backup := range metadata.DefaultStore.GetBackups() {
			existingIDs[backup.ID] = true
		}
	}

	// Process each recovered backup
	for _, backup := range backups {
		// Generate ID
		id := fmt.Sprintf("%s-%s-%s-%s", backup.ServerName, backup.Database, backup.BackupType, backup.Timestamp)
		
		// Skip if already exists in merge mode
		if existingIDs[id] {
			if *verbose {
				log.Printf("Skipping existing backup: %s", id)
			}
			continue
		}

		// Parse timestamp
		createdAt, err := time.Parse("20060102-150405", backup.Timestamp)
		if err != nil {
			log.Printf("Failed to parse timestamp for %s: %v", backup.Filename, err)
			createdAt = backup.ModTime // Fallback to file modification time
		}

		// Create backup metadata entry
		backupMeta := metadata.DefaultStore.CreateBackupMeta(
			backup.ServerName,
			"mysql", // Assume MySQL for now
			backup.Database,
			backup.BackupType,
		)

		// Set additional fields
		backupMeta.ID = id
		backupMeta.CreatedAt = createdAt
		backupMeta.CompletedAt = createdAt.Add(5 * time.Minute) // Estimate
		backupMeta.Size = backup.Size

		// Set paths based on storage location
		if backup.IsS3 {
			backupMeta.S3Keys = map[string]string{
				"default": backup.S3Key,
			}
			backupMeta.S3UploadStatus = types.StatusSuccess
		} else {
			// Make path relative to backup directory
			relPath, err := filepath.Rel(config.CFG.Local.BackupDirectory, backup.Path)
			if err != nil {
				relPath = backup.Path
			}
			backupMeta.LocalPaths = map[string]string{
				"default": relPath,
			}
		}

		// Update status
		metadata.DefaultStore.UpdateBackupStatus(
			backupMeta.ID,
			types.StatusSuccess,
			backupMeta.LocalPaths,
			backup.Size,
			"",
		)

		// Update S3 status if applicable
		if backup.IsS3 {
			metadata.DefaultStore.UpdateS3UploadStatus(
				backupMeta.ID,
				types.StatusSuccess,
				backupMeta.S3Keys,
				"",
			)
		}

		if *verbose {
			log.Printf("Recovered backup: %s", id)
		}
	}
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

// reconcileBackups reconciles local and S3 backups to handle duplicates
func reconcileBackups(backups []RecoveredBackup) []RecoveredBackup {
	// Group backups by ID
	backupMap := make(map[string][]RecoveredBackup)
	
	for _, backup := range backups {
		id := fmt.Sprintf("%s-%s-%s-%s", backup.ServerName, backup.Database, backup.BackupType, backup.Timestamp)
		backupMap[id] = append(backupMap[id], backup)
	}

	// Reconcile duplicates
	var reconciled []RecoveredBackup
	for id, group := range backupMap {
		if len(group) == 1 {
			reconciled = append(reconciled, group[0])
			continue
		}

		// Multiple copies found - merge information
		merged := group[0]
		hasLocal := false
		hasS3 := false

		for _, backup := range group {
			if backup.IsS3 {
				hasS3 = true
				merged.S3Bucket = backup.S3Bucket
				merged.S3Key = backup.S3Key
			} else {
				hasLocal = true
				merged.Path = backup.Path
			}
		}

		// Update merged backup info
		if hasLocal && hasS3 {
			if *verbose {
				log.Printf("Found backup %s in both local and S3 storage", id)
			}
		}

		reconciled = append(reconciled, merged)
	}

	return reconciled
}