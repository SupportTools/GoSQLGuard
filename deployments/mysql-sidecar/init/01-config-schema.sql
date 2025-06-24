-- GoSQLGuard Configuration Database Schema
-- This database stores all configuration for GoSQLGuard in a MySQL sidecar

CREATE DATABASE IF NOT EXISTS gosqlguard_config;
USE gosqlguard_config;

-- Global configuration settings
CREATE TABLE IF NOT EXISTS global_config (
    id INT PRIMARY KEY AUTO_INCREMENT,
    `key` VARCHAR(255) UNIQUE NOT NULL,
    `value` TEXT,
    `type` ENUM('string', 'boolean', 'integer', 'json') DEFAULT 'string',
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_key (`key`)
);

-- Database servers configuration
CREATE TABLE IF NOT EXISTS database_servers (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(255) UNIQUE NOT NULL,
    type ENUM('mysql', 'postgresql') NOT NULL,
    host VARCHAR(255) NOT NULL,
    port INT NOT NULL,
    username VARCHAR(255),
    password VARCHAR(255),
    auth_plugin VARCHAR(50),
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_name (name),
    INDEX idx_enabled (enabled)
);

-- Database inclusion/exclusion rules
CREATE TABLE IF NOT EXISTS database_filters (
    id INT PRIMARY KEY AUTO_INCREMENT,
    server_id INT NOT NULL,
    filter_type ENUM('include', 'exclude') NOT NULL,
    database_name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES database_servers(id) ON DELETE CASCADE,
    UNIQUE KEY unique_server_filter (server_id, filter_type, database_name),
    INDEX idx_server_id (server_id)
);

-- Storage configurations
CREATE TABLE IF NOT EXISTS storage_configs (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(255) UNIQUE NOT NULL,
    type ENUM('local', 's3') NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    config JSON NOT NULL COMMENT 'JSON configuration specific to storage type',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_name (name),
    INDEX idx_type (type),
    INDEX idx_enabled (enabled)
);

-- Backup schedules
CREATE TABLE IF NOT EXISTS backup_schedules (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(255) UNIQUE NOT NULL,
    backup_type VARCHAR(50) NOT NULL,
    cron_expression VARCHAR(255) NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_name (name),
    INDEX idx_enabled (enabled)
);

-- Retention policies
CREATE TABLE IF NOT EXISTS retention_policies (
    id INT PRIMARY KEY AUTO_INCREMENT,
    schedule_id INT NOT NULL,
    storage_id INT NOT NULL,
    retention_duration VARCHAR(50) COMMENT 'Duration in Go format (e.g., 24h, 168h)',
    keep_forever BOOLEAN DEFAULT FALSE,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (schedule_id) REFERENCES backup_schedules(id) ON DELETE CASCADE,
    FOREIGN KEY (storage_id) REFERENCES storage_configs(id) ON DELETE CASCADE,
    UNIQUE KEY unique_schedule_storage (schedule_id, storage_id),
    INDEX idx_schedule_id (schedule_id),
    INDEX idx_storage_id (storage_id)
);

-- MySQL dump options
CREATE TABLE IF NOT EXISTS mysql_dump_options (
    id INT PRIMARY KEY AUTO_INCREMENT,
    server_id INT COMMENT 'NULL for global options',
    option_name VARCHAR(255) NOT NULL,
    option_value VARCHAR(255),
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES database_servers(id) ON DELETE CASCADE,
    INDEX idx_server_id (server_id),
    INDEX idx_enabled (enabled)
);

-- Configuration change history
CREATE TABLE IF NOT EXISTS config_history (
    id INT PRIMARY KEY AUTO_INCREMENT,
    table_name VARCHAR(255) NOT NULL,
    record_id INT NOT NULL,
    action ENUM('INSERT', 'UPDATE', 'DELETE') NOT NULL,
    old_values JSON,
    new_values JSON,
    changed_by VARCHAR(255),
    changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_table_record (table_name, record_id),
    INDEX idx_changed_at (changed_at)
);

-- Active configuration version
CREATE TABLE IF NOT EXISTS config_versions (
    id INT PRIMARY KEY AUTO_INCREMENT,
    version VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    active BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_active (active),
    INDEX idx_version (version)
);