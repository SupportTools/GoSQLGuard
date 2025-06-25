// Package database provides database-related functionality
package database

import (
	"fmt"
	"log"
)

// PostgreSQLDumpOptions defines optional flags that can be passed to pg_dump
type PostgreSQLDumpOptions struct {
	// Output format options
	Format     string `json:"format" yaml:"format"`          // -F, --format (plain, custom, directory, tar)
	Verbose    bool   `json:"verbose" yaml:"verbose"`        // -v, --verbose
	NoComments bool   `json:"no_comments" yaml:"noComments"` // --no-comments

	// Object selection options
	SchemaOnly bool `json:"schema_only" yaml:"schemaOnly"` // -s, --schema-only
	DataOnly   bool `json:"data_only" yaml:"dataOnly"`     // -a, --data-only
	Blobs      bool `json:"blobs" yaml:"blobs"`            // -b, --blobs
	NoBlobs    bool `json:"no_blobs" yaml:"noBlobs"`       // -B, --no-blobs

	// Output control options
	Clean         bool `json:"clean" yaml:"clean"`                  // -c, --clean
	Create        bool `json:"create" yaml:"create"`                // -C, --create
	IfExists      bool `json:"if_exists" yaml:"ifExists"`           // --if-exists
	NoOwner       bool `json:"no_owner" yaml:"noOwner"`             // -O, --no-owner
	NoPrivileges  bool `json:"no_privileges" yaml:"noPrivileges"`   // -x, --no-privileges
	NoTablespaces bool `json:"no_tablespaces" yaml:"noTablespaces"` // --no-tablespaces

	// Connection options
	NoPassword bool `json:"no_password" yaml:"noPassword"` // -w, --no-password

	// Dump options
	InsertColumns       bool `json:"insert_columns" yaml:"insertColumns"`               // --column-inserts
	OnConflictDoNothing bool `json:"on_conflict_do_nothing" yaml:"onConflictDoNothing"` // --on-conflict-do-nothing

	// Performance options
	Jobs     int `json:"jobs" yaml:"jobs"`         // -j, --jobs
	Compress int `json:"compress" yaml:"compress"` // -Z, --compress (0-9)

	// Additional custom options
	CustomOptions []string `json:"custom_options" yaml:"customOptions"` // Any additional options
}

// GetCommandLineArgs converts the options to command-line arguments
func (o *PostgreSQLDumpOptions) GetCommandLineArgs() []string {
	var args []string

	// Output format options
	if o.Format != "" && o.Format != "plain" {
		args = append(args, "-F", o.Format)
	}
	if o.Verbose {
		args = append(args, "--verbose")
	}
	if o.NoComments {
		args = append(args, "--no-comments")
	}

	// Object selection options
	if o.SchemaOnly {
		args = append(args, "--schema-only")
	}
	if o.DataOnly {
		args = append(args, "--data-only")
	}
	if o.Blobs {
		args = append(args, "--blobs")
	}
	if o.NoBlobs {
		args = append(args, "--no-blobs")
	}

	// Output control options
	if o.Clean {
		args = append(args, "--clean")
	}
	if o.Create {
		args = append(args, "--create")
	}
	if o.IfExists {
		args = append(args, "--if-exists")
	}
	if o.NoOwner {
		args = append(args, "--no-owner")
	}
	if o.NoPrivileges {
		args = append(args, "--no-privileges")
	}
	if o.NoTablespaces {
		args = append(args, "--no-tablespaces")
	}

	// Connection options
	if o.NoPassword {
		args = append(args, "--no-password")
	}

	// Dump options
	if o.InsertColumns {
		args = append(args, "--column-inserts")
	}
	if o.OnConflictDoNothing {
		args = append(args, "--on-conflict-do-nothing")
	}

	// Performance options
	if o.Jobs > 0 {
		args = append(args, "-j", fmt.Sprintf("%d", o.Jobs))
	}
	if o.Compress > 0 && o.Compress <= 9 {
		args = append(args, "-Z", fmt.Sprintf("%d", o.Compress))
	}

	// Debug logging for custom options
	if len(o.CustomOptions) > 0 {
		log.Printf("DEBUG: Adding custom options to pg_dump args: %v", o.CustomOptions)
	}

	// Add any custom options
	args = append(args, o.CustomOptions...)

	return args
}

// DefaultPostgreSQLDumpOptions returns a set of recommended default options
func DefaultPostgreSQLDumpOptions() PostgreSQLDumpOptions {
	return PostgreSQLDumpOptions{
		Format:              "custom", // Custom format for better compression and flexibility
		Verbose:             false,
		NoComments:          false,
		SchemaOnly:          false,
		DataOnly:            false,
		Blobs:               true, // Include large objects
		NoBlobs:             false,
		Clean:               false,
		Create:              false,
		IfExists:            false,
		NoOwner:             true, // Avoid permission issues on restore
		NoPrivileges:        true, // Avoid permission issues on restore
		NoTablespaces:       true, // Avoid tablespace issues
		NoPassword:          true, // Use .pgpass or environment variables
		InsertColumns:       false,
		OnConflictDoNothing: false,
		Jobs:                1, // Single-threaded by default
		Compress:            6, // Medium compression
		CustomOptions:       []string{},
	}
}
