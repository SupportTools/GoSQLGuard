package types

import "time"

// PageData holds common data for all pages
type PageData struct {
	Title       string
	Description string
	Time        string
	AppName     string
	Version     string
	NavLinks    []NavLink
	Content     interface{}
}

// NavLink represents a navigation link
type NavLink struct {
	URL      string
	Name     string
	Active   bool
	Icon     string
	External bool
}

// BackupInfo represents a backup entry
type BackupInfo struct {
	ID           string
	BackupType   string
	Date         time.Time
	Size         int64
	Path         string
	StorageType  string
	ServerName   string
	DatabaseName string
	Status       string
	Error        string
	Duration     time.Duration
}

// ServerInfo represents a database server
type ServerInfo struct {
	Name         string
	Type         string
	Host         string
	Port         int
	Status       string
	LastBackup   time.Time
	DatabaseCount int
}

// DatabaseInfo represents a database
type DatabaseInfo struct {
	Name       string
	ServerName string
	Size       int64
	LastBackup time.Time
	Status     string
}

// StorageInfo represents storage status
type StorageInfo struct {
	Type        string
	Path        string
	Used        int64
	Available   int64
	Total       int64
	BackupCount int
}