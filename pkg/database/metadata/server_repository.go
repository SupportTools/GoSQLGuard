package metadata

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

// ServerRepository handles database operations for server configurations
type ServerRepository struct {
	db *gorm.DB
}

// NewServerRepository creates a new ServerRepository instance
func NewServerRepository(db *gorm.DB) *ServerRepository {
	return &ServerRepository{db: db}
}

// GetAllServers retrieves all server configurations
func (r *ServerRepository) GetAllServers() ([]ServerConfig, error) {
	var servers []ServerConfig
	
	err := r.db.Preload("DatabaseFilters").Preload("MySQLOptions").Find(&servers).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get servers: %w", err)
	}
	
	return servers, nil
}

// GetServerByID retrieves a server configuration by ID
func (r *ServerRepository) GetServerByID(id string) (*ServerConfig, error) {
	var server ServerConfig
	
	err := r.db.Preload("DatabaseFilters").Preload("MySQLOptions").Where("id = ?", id).First(&server).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("server not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get server: %w", err)
	}
	
	return &server, nil
}

// GetServerByName retrieves a server configuration by name
func (r *ServerRepository) GetServerByName(name string) (*ServerConfig, error) {
	var server ServerConfig
	
	err := r.db.Preload("DatabaseFilters").Preload("MySQLOptions").Where("name = ?", name).First(&server).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("server not found: %s", name)
		}
		return nil, fmt.Errorf("failed to get server: %w", err)
	}
	
	return &server, nil
}

// CreateServer creates a new server configuration
func (r *ServerRepository) CreateServer(server *ServerConfig) error {
	// Generate a new UUID if not provided
	if server.ID == "" {
		server.ID = uuid.New().String()
	}
	
	// Set timestamps
	now := time.Now()
	server.CreatedAt = now
	server.UpdatedAt = now
	
	// Start a transaction
	tx := r.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}
	
	// Create the server
	if err := tx.Create(server).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create server: %w", err)
	}
	
	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

// UpdateServer updates an existing server configuration
func (r *ServerRepository) UpdateServer(server *ServerConfig) error {
	// Check if server exists
	exists, err := r.ServerExists(server.ID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("server not found: %s", server.ID)
	}
	
	// Update timestamp
	server.UpdatedAt = time.Now()
	
	// Start a transaction
	tx := r.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}
	
	// Delete existing database filters and MySQL options to replace them
	if err := tx.Where("server_id = ?", server.ID).Delete(&ServerDatabaseFilter{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete existing database filters: %w", err)
	}
	
	if err := tx.Where("server_id = ?", server.ID).Delete(&ServerMySQLOption{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete existing MySQL options: %w", err)
	}
	
	// Update the server
	if err := tx.Save(server).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update server: %w", err)
	}
	
	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

// DeleteServer deletes a server configuration
func (r *ServerRepository) DeleteServer(id string) error {
	// Start a transaction
	tx := r.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}
	
	// Delete the server (cascade will delete related filters and options)
	if err := tx.Delete(&ServerConfig{ID: id}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete server: %w", err)
	}
	
	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

// ServerExists checks if a server with the given ID exists
func (r *ServerRepository) ServerExists(id string) (bool, error) {
	var count int64
	err := r.db.Model(&ServerConfig{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if server exists: %w", err)
	}
	return count > 0, nil
}

// ServerExistsByName checks if a server with the given name exists
func (r *ServerRepository) ServerExistsByName(name string) (bool, error) {
	var count int64
	err := r.db.Model(&ServerConfig{}).Where("name = ?", name).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if server exists: %w", err)
	}
	return count > 0, nil
}
