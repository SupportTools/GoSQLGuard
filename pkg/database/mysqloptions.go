// Package database provides database-related functionality
package database

import (
	"log"
)

// MySQLDumpOptions defines optional flags that can be passed to mysqldump
type MySQLDumpOptions struct {
	// Transaction and locking options
	SingleTransaction bool `json:"single_transaction" yaml:"singleTransaction"` // --single-transaction
	Quick             bool `json:"quick" yaml:"quick"`                          // --quick
	LockTables        bool `json:"lock_tables" yaml:"lockTables"`               // --lock-tables
	SkipLockTables    bool `json:"skip_lock_tables" yaml:"skipLockTables"`      // --skip-lock-tables
	SkipAddLocks      bool `json:"skip_add_locks" yaml:"skipAddLocks"`          // --skip-add-locks

	// Output formatting options
	SkipComments       bool `json:"skip_comments" yaml:"skipComments"`              // --skip-comments
	CompleteInsert     bool `json:"complete_insert" yaml:"completeInsert"`          // --complete-insert
	ExtendedInsert     bool `json:"extended_insert" yaml:"extendedInsert"`          // --extended-insert
	SkipExtendedInsert bool `json:"skip_extended_insert" yaml:"skipExtendedInsert"` // --skip-extended-insert

	// Performance options
	Compress bool `json:"compress" yaml:"compress"` // --compress

	// Schema options
	Triggers bool `json:"triggers" yaml:"triggers"` // --triggers
	Routines bool `json:"routines" yaml:"routines"` // --routines
	Events   bool `json:"events" yaml:"events"`     // --events

	// Additional custom options (for future extensibility)
	CustomOptions []string `json:"custom_options" yaml:"customOptions"` // Any additional options
}

// GetCommandLineArgs converts the options to command-line arguments
func (o *MySQLDumpOptions) GetCommandLineArgs() []string {
	var args []string

	// Transaction and locking options
	if o.SingleTransaction {
		args = append(args, "--single-transaction")
	}
	if o.Quick {
		args = append(args, "--quick")
	}
	if o.LockTables {
		args = append(args, "--lock-tables")
	}
	if o.SkipLockTables {
		args = append(args, "--skip-lock-tables")
	}
	if o.SkipAddLocks {
		args = append(args, "--skip-add-locks")
	}

	// Output formatting options
	if o.SkipComments {
		args = append(args, "--skip-comments")
	}
	if o.CompleteInsert {
		args = append(args, "--complete-insert")
	}
	if o.ExtendedInsert {
		args = append(args, "--extended-insert")
	}
	if o.SkipExtendedInsert {
		args = append(args, "--skip-extended-insert")
	}

	// Performance options
	if o.Compress {
		args = append(args, "--compress")
	}

	// Schema options
	if o.Triggers {
		args = append(args, "--triggers")
	}
	if o.Routines {
		args = append(args, "--routines")
	}
	if o.Events {
		args = append(args, "--events")
	}

	// Debug logging for custom options
	if len(o.CustomOptions) > 0 {
		log.Printf("DEBUG: Adding custom options to mysqldump args: %v", o.CustomOptions)
	}

	// Add any custom options
	args = append(args, o.CustomOptions...)

	return args
}

// DefaultMySQLDumpOptions returns a set of recommended default options
func DefaultMySQLDumpOptions() MySQLDumpOptions {
	return MySQLDumpOptions{
		SingleTransaction:  true,
		Quick:              true,
		LockTables:         false,
		SkipAddLocks:       false,
		SkipLockTables:     false,
		SkipComments:       false,
		CompleteInsert:     false,
		ExtendedInsert:     true,
		SkipExtendedInsert: false,
		Compress:           false,
		Triggers:           true,
		Routines:           true,
		Events:             true,
		CustomOptions:      []string{},
	}
}
