// Package metrics provides Prometheus metrics for MySQL backup operations.
package metrics

import (
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus metrics
var (
	// BackupCount tracks the total number of MySQL backups performed
	BackupCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mysql_backup_total",
		Help: "The total number of MySQL backups performed",
	}, []string{"type", "database", "status"})

	// BackupDuration measures time taken to perform MySQL backup
	BackupDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mysql_backup_duration_seconds",
		Help:    "Time taken to perform MySQL backup",
		Buckets: prometheus.DefBuckets,
	}, []string{"type", "database"})

	// BackupSize tracks size of the backup file in bytes
	BackupSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mysql_backup_size_bytes",
		Help: "Size of the backup file in bytes",
	}, []string{"type", "database", "storage"})

	// BackupRetentionDeletes counts backups deleted by retention policy
	BackupRetentionDeletes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mysql_backup_deletions_total",
		Help: "The total number of backups deleted by retention policy",
	}, []string{"type", "storage"})

	// LastBackupTimestamp records timestamp of the last successful backup
	LastBackupTimestamp = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mysql_backup_last_timestamp",
		Help: "Timestamp of the last successful backup",
	}, []string{"type", "database"})

	// S3UploadCount tracks the total number of S3 uploads performed
	S3UploadCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mysql_backup_s3_upload_total",
		Help: "The total number of S3 uploads performed",
	}, []string{"type", "database", "status"})

	// S3UploadDuration measures time taken to upload backup to S3
	S3UploadDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mysql_backup_s3_upload_duration_seconds",
		Help:    "Time taken to upload backup to S3",
		Buckets: prometheus.DefBuckets,
	}, []string{"type", "database"})
)

// StartMetricsServer starts the HTTP server for metrics and health check endpoints
func StartMetricsServer(port string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Starting metrics server on port %s", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start metrics server: %v", err)
	}
}
