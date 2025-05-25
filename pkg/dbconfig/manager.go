// Package dbconfig provides functionality to manage configuration stored in the database
package dbconfig

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/supporttools/GoSQLGuard/pkg/configtypes"
	"github.com/supporttools/GoSQLGuard/pkg/database/metadata"
)

// ConfigRefresher interface for components that need to refresh when config changes
type ConfigRefresher interface {
	RefreshConfig()
}

// Manager handles loading and updating configuration from the database
type Manager struct {
	db                *gorm.DB
	serverRepository  *metadata.ServerRepository
	scheduleRepository *metadata.ScheduleRepository
	refreshListeners  []ConfigRefresher
}

// NewManager creates a new configuration manager
func NewManager(db *gorm.DB) *Manager {
	return &Manager{
		db:                db,
		serverRepository:  metadata.NewServerRepository(db),
		scheduleRepository: metadata.NewScheduleRepository(db),
		refreshListeners:  make([]ConfigRefresher, 0),
	}
}

// AddRefreshListener registers a component to be notified when config changes
func (m *Manager) AddRefreshListener(listener ConfigRefresher) {
	m.refreshListeners = append(m.refreshListeners, listener)
}

// NotifyConfigChanged informs all listeners about config changes
func (m *Manager) NotifyConfigChanged() {
	for _, listener := range m.refreshListeners {
		listener.RefreshConfig()
	}
}

// GetServers retrieves all server configurations from the database
func (m *Manager) GetServers() ([]configtypes.ServerConfig, error) {
	if m.db == nil {
		return nil, fmt.Errorf("metadata database is not initialized")
	}

	// Get all servers from the database
	servers, err := m.serverRepository.GetAllServers()
	if err != nil {
		return nil, fmt.Errorf("failed to load server configurations: %w", err)
	}

	// Convert database model to configtypes
	result := make([]configtypes.ServerConfig, 0, len(servers))
	for _, server := range servers {
		serverCfg := configtypes.ServerConfig{
			ID:         server.ID,
			Name:       server.Name,
			Type:       server.Type,
			Host:       server.Host,
			Port:       server.Port,
			Username:   server.Username,
			Password:   server.Password,
			AuthPlugin: server.AuthPlugin,
		}

		// Process include/exclude databases
		for _, filter := range server.DatabaseFilters {
			if filter.FilterType == "include" {
				serverCfg.IncludeDatabases = append(serverCfg.IncludeDatabases, filter.DatabaseName)
			} else if filter.FilterType == "exclude" {
				serverCfg.ExcludeDatabases = append(serverCfg.ExcludeDatabases, filter.DatabaseName)
			}
		}

		result = append(result, serverCfg)
	}

	return result, nil
}

// GetSchedules retrieves all schedule configurations from the database
func (m *Manager) GetSchedules() ([]configtypes.ScheduleConfig, error) {
	if m.db == nil {
		return nil, fmt.Errorf("metadata database is not initialized")
	}

	// Get all schedules from the database
	schedules, err := m.scheduleRepository.GetAllSchedules()
	if err != nil {
		return nil, fmt.Errorf("failed to load schedule configurations: %w", err)
	}

	// Convert database model to configtypes
	result := make([]configtypes.ScheduleConfig, 0, len(schedules))
	for _, schedule := range schedules {
		scheduleCfg := configtypes.ScheduleConfig{
			ID:             schedule.ID,
			Name:           schedule.Name,
			BackupType:     schedule.BackupType,
			CronExpression: schedule.CronExpression,
			Enabled:        schedule.Enabled,
			LocalStorage: configtypes.StorageConfig{
				Enabled:     false,
				Duration:    "24h",
				KeepForever: false,
			},
			S3Storage: configtypes.StorageConfig{
				Enabled:     false,
				Duration:    "24h",
				KeepForever: false,
			},
		}

		// Process retention policies
		for _, policy := range schedule.RetentionPolicies {
			if policy.StorageType == "local" {
				scheduleCfg.LocalStorage.Enabled = true
				scheduleCfg.LocalStorage.Duration = policy.Duration
				scheduleCfg.LocalStorage.KeepForever = policy.KeepForever
			} else if policy.StorageType == "s3" {
				scheduleCfg.S3Storage.Enabled = true
				scheduleCfg.S3Storage.Duration = policy.Duration
				scheduleCfg.S3Storage.KeepForever = policy.KeepForever
			}
		}

		result = append(result, scheduleCfg)
	}

	return result, nil
}

// SaveServer saves a server configuration to the database
func (m *Manager) SaveServer(serverCfg configtypes.ServerConfig) error {
	if m.db == nil {
		return fmt.Errorf("metadata database is not initialized")
	}

	// Convert configtypes to database model
	server := metadata.ServerConfig{
		Name:       serverCfg.Name,
		Type:       serverCfg.Type,
		Host:       serverCfg.Host,
		Port:       serverCfg.Port,
		Username:   serverCfg.Username,
		Password:   serverCfg.Password,
		AuthPlugin: serverCfg.AuthPlugin,
	}

	// Check if server already exists
	existingServer, err := m.serverRepository.GetServerByName(serverCfg.Name)
	if err == nil {
		// Server exists, update it
		server.ID = existingServer.ID
		server.CreatedAt = existingServer.CreatedAt

		// Add include databases
		for _, dbName := range serverCfg.IncludeDatabases {
			server.DatabaseFilters = append(server.DatabaseFilters, metadata.ServerDatabaseFilter{
				ServerID:     server.ID,
				FilterType:   "include",
				DatabaseName: dbName,
				CreatedAt:    time.Now(),
			})
		}

		// Add exclude databases
		for _, dbName := range serverCfg.ExcludeDatabases {
			server.DatabaseFilters = append(server.DatabaseFilters, metadata.ServerDatabaseFilter{
				ServerID:     server.ID,
				FilterType:   "exclude",
				DatabaseName: dbName,
				CreatedAt:    time.Now(),
			})
		}

		// Update the server
		if err := m.serverRepository.UpdateServer(&server); err != nil {
			return fmt.Errorf("failed to update server configuration: %w", err)
		}
	} else {
		// Server doesn't exist, create it
		// Generate ID for new server if not provided
		if serverCfg.ID == "" {
			server.ID = uuid.New().String()
		} else {
			server.ID = serverCfg.ID
		}
		
		// Add include databases
		for _, dbName := range serverCfg.IncludeDatabases {
			server.DatabaseFilters = append(server.DatabaseFilters, metadata.ServerDatabaseFilter{
				ServerID:     server.ID,
				FilterType:   "include",
				DatabaseName: dbName,
				CreatedAt:    time.Now(),
			})
		}

		// Add exclude databases
		for _, dbName := range serverCfg.ExcludeDatabases {
			server.DatabaseFilters = append(server.DatabaseFilters, metadata.ServerDatabaseFilter{
				ServerID:     server.ID,
				FilterType:   "exclude",
				DatabaseName: dbName,
				CreatedAt:    time.Now(),
			})
		}

		// Create the server
		if err := m.serverRepository.CreateServer(&server); err != nil {
			return fmt.Errorf("failed to create server configuration: %w", err)
		}
	}

	m.NotifyConfigChanged()
	return nil
}

// DeleteServer deletes a server configuration from the database
func (m *Manager) DeleteServer(serverName string) error {
	if m.db == nil {
		return fmt.Errorf("metadata database is not initialized")
	}

	// Find the server by name
	server, err := m.serverRepository.GetServerByName(serverName)
	if err != nil {
		return fmt.Errorf("failed to find server configuration: %w", err)
	}

	// Delete the server
	if err := m.serverRepository.DeleteServer(server.ID); err != nil {
		return fmt.Errorf("failed to delete server configuration: %w", err)
	}

	m.NotifyConfigChanged()
	return nil
}

// SaveSchedule saves a schedule configuration to the database
func (m *Manager) SaveSchedule(scheduleCfg configtypes.ScheduleConfig) error {
	if m.db == nil {
		return fmt.Errorf("metadata database is not initialized")
	}

	// Convert configtypes to database model
	schedule := metadata.BackupSchedule{
		Name:           scheduleCfg.Name,
		BackupType:     scheduleCfg.BackupType,
		CronExpression: scheduleCfg.CronExpression,
		Enabled:        scheduleCfg.Enabled,
	}

	// Add retention policies
	if scheduleCfg.LocalStorage.Enabled {
		schedule.RetentionPolicies = append(schedule.RetentionPolicies, metadata.ScheduleRetentionPolicy{
			StorageType:  "local",
			Duration:     scheduleCfg.LocalStorage.Duration,
			KeepForever:  scheduleCfg.LocalStorage.KeepForever,
			CreatedAt:    time.Now(),
		})
	}

	if scheduleCfg.S3Storage.Enabled {
		schedule.RetentionPolicies = append(schedule.RetentionPolicies, metadata.ScheduleRetentionPolicy{
			StorageType:  "s3",
			Duration:     scheduleCfg.S3Storage.Duration,
			KeepForever:  scheduleCfg.S3Storage.KeepForever,
			CreatedAt:    time.Now(),
		})
	}

	// Check if schedule already exists
	existingSchedule, err := m.scheduleRepository.GetScheduleByName(scheduleCfg.Name)
	if err == nil {
		// Schedule exists, update it
		schedule.ID = existingSchedule.ID
		schedule.CreatedAt = existingSchedule.CreatedAt

		// Update the schedule
		if err := m.scheduleRepository.UpdateSchedule(&schedule); err != nil {
			return fmt.Errorf("failed to update schedule configuration: %w", err)
		}
	} else {
		// Schedule doesn't exist, create it
		// Generate ID for new schedule if not provided
		if scheduleCfg.ID == "" {
			schedule.ID = uuid.New().String()
		} else {
			schedule.ID = scheduleCfg.ID
		}
		
		if err := m.scheduleRepository.CreateSchedule(&schedule); err != nil {
			return fmt.Errorf("failed to create schedule configuration: %w", err)
		}
	}

	m.NotifyConfigChanged()
	return nil
}

// DeleteSchedule deletes a schedule configuration from the database
func (m *Manager) DeleteSchedule(scheduleName string) error {
	if m.db == nil {
		return fmt.Errorf("metadata database is not initialized")
	}

	// Find the schedule by name
	schedule, err := m.scheduleRepository.GetScheduleByName(scheduleName)
	if err != nil {
		return fmt.Errorf("failed to find schedule configuration: %w", err)
	}

	// Delete the schedule
	if err := m.scheduleRepository.DeleteSchedule(schedule.ID); err != nil {
		return fmt.Errorf("failed to delete schedule configuration: %w", err)
	}

	m.NotifyConfigChanged()
	return nil
}

// TestServerConnection tests the connection to a server
func (m *Manager) TestServerConnection(serverCfg configtypes.ServerConfig) error {
	// Implementation will depend on the database type
	if serverCfg.Type == "mysql" {
		// Test MySQL connection
		// This would call into the MySQL provider to test the connection
		// For now, we'll just return nil to indicate success
		return nil
	} else if serverCfg.Type == "postgresql" {
		// Test PostgreSQL connection
		// This would call into the PostgreSQL provider to test the connection
		// For now, we'll just return nil to indicate success
		return nil
	}

	return fmt.Errorf("unsupported database type: %s", serverCfg.Type)
}

// Global instance
var instance *Manager

// GetInstance returns the singleton manager instance
func GetInstance() *Manager {
	if instance == nil && metadata.DB != nil {
		instance = NewManager(metadata.DB)
	}
	return instance
}
