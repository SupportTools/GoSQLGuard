package mysql

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/supporttools/GoSQLGuard/pkg/config"
)

// GetAllDatabases returns a list of all databases from the MySQL server
// excluding system databases
func GetAllDatabases() ([]string, error) {
	// Check if we're using multi-server configuration
	if len(config.CFG.DatabaseServers) > 0 {
		// Use the first MySQL server as the default for UI display
		var mysqlServer *config.DatabaseServerConfig
		for i, server := range config.CFG.DatabaseServers {
			if server.Type == "mysql" {
				mysqlServer = &config.CFG.DatabaseServers[i]
				break
			}
		}
		
		// If we found a MySQL server in the configuration
		if mysqlServer != nil {
			log.Printf("Using server %s for database list", mysqlServer.Name)
			return GetDatabasesFromServer(*mysqlServer)
		}
	}
	
	// Fall back to legacy config if no database servers are configured
	// or if no MySQL servers were found
	if config.CFG.MySQL.Host != "" {
		log.Printf("Using legacy MySQL configuration for database list")
		host := config.CFG.MySQL.Host
		port := config.CFG.MySQL.Port
		username := config.CFG.MySQL.Username
		password := config.CFG.MySQL.Password
		
		// Create connection string
		connStr := fmt.Sprintf("%s:%s@tcp(%s:%s)/", username, password, host, port)
		return connectAndListDatabases(connStr, config.CFG.MySQL.ExcludeDatabases)
	}
	
	return []string{}, fmt.Errorf("no MySQL configuration found")
}

// GetDatabasesFromServer returns a list of all databases from a specific MySQL server
func GetDatabasesFromServer(server config.DatabaseServerConfig) ([]string, error) {
	if server.Type != "mysql" {
		return nil, fmt.Errorf("server %s is not a MySQL server", server.Name)
	}
	
	// Create connection string
	connStr := fmt.Sprintf("%s:%s@tcp(%s:%s)/", 
		server.Username, server.Password, server.Host, server.Port)
	
	return connectAndListDatabases(connStr, server.ExcludeDatabases)
}

// connectAndListDatabases handles the common database connection and listing logic
func connectAndListDatabases(connStr string, excludeList []string) ([]string, error) {
	
	// Create connection
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return nil, fmt.Errorf("error connecting to MySQL server: %w", err)
	}
	defer db.Close()

	// Check connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to MySQL server: %w", err)
	}

	// Get list of databases
	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		return nil, fmt.Errorf("error querying databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	var excludeDatabases = map[string]bool{
		"information_schema": true,
		"performance_schema": true,
		"mysql":              true,
		"sys":                true,
	}

	// Add user-configured excludes
	for _, db := range excludeList {
		excludeDatabases[strings.ToLower(db)] = true
	}

	// Process rows
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			log.Printf("Error scanning database name: %v", err)
			continue
		}

		// Skip system databases
		if excludeDatabases[strings.ToLower(dbName)] {
			continue
		}

		databases = append(databases, dbName)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating database rows: %w", err)
	}

	return databases, nil
}
