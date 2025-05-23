// Package database provides an abstraction layer for database operations
package database

// Database interface defines higher-level database operations
// used by the application for backup and restore operations
type Database interface {
	Backup(databaseName string) error
	Restore(backupLocation string, databaseName string) error
	GetSchema(databaseName string) (string, error)
	Close() error
}
