-- Default configuration data for GoSQLGuard
USE gosqlguard_config;

-- Insert global configuration
INSERT INTO global_config (`key`, `value`, `type`, description) VALUES
('debug', 'false', 'boolean', 'Enable debug mode'),
('metrics_port', '8080', 'string', 'Port for metrics and admin UI'),
('config_version', '1.0', 'string', 'Configuration version');

-- Insert default storage configurations
INSERT INTO storage_configs (name, type, enabled, config) VALUES
('local-storage', 'local', TRUE, JSON_OBJECT(
    'backupDirectory', '/backups',
    'organizationStrategy', 'combined'
)),
('s3-storage', 's3', FALSE, JSON_OBJECT(
    'bucket', 'gosqlguard-backups',
    'region', 'us-east-1',
    'endpoint', '',
    'accessKey', '',
    'secretKey', '',
    'prefix', '',
    'useSSL', true,
    'organizationStrategy', 'combined'
));

-- Insert default backup schedules
INSERT INTO backup_schedules (name, backup_type, cron_expression, enabled) VALUES
('hourly', 'hourly', '0 * * * *', TRUE),
('daily', 'daily', '0 2 * * *', TRUE),
('weekly', 'weekly', '0 3 * * 0', TRUE);

-- Insert default retention policies
INSERT INTO retention_policies (schedule_id, storage_id, retention_duration, keep_forever, enabled)
SELECT 
    s.id as schedule_id,
    st.id as storage_id,
    CASE s.backup_type
        WHEN 'hourly' THEN '24h'
        WHEN 'daily' THEN '168h'
        WHEN 'weekly' THEN '720h'
    END as retention_duration,
    FALSE as keep_forever,
    TRUE as enabled
FROM backup_schedules s
CROSS JOIN storage_configs st
WHERE st.name = 'local-storage';

-- Insert default MySQL dump options (global)
INSERT INTO mysql_dump_options (server_id, option_name, option_value, enabled) VALUES
(NULL, '--single-transaction', NULL, TRUE),
(NULL, '--routines', NULL, TRUE),
(NULL, '--triggers', NULL, TRUE),
(NULL, '--events', NULL, TRUE),
(NULL, '--add-drop-database', NULL, TRUE),
(NULL, '--databases', NULL, TRUE),
(NULL, '--no-tablespaces', NULL, TRUE);

-- Create default configuration version
INSERT INTO config_versions (version, description, active) VALUES
('1.0', 'Initial configuration', TRUE);

-- Create a stored procedure to add a new database server
DELIMITER //
CREATE PROCEDURE add_database_server(
    IN p_name VARCHAR(255),
    IN p_type ENUM('mysql', 'postgresql'),
    IN p_host VARCHAR(255),
    IN p_port INT,
    IN p_username VARCHAR(255),
    IN p_password VARCHAR(255),
    IN p_databases JSON
)
BEGIN
    DECLARE server_id INT;
    DECLARE db_name VARCHAR(255);
    DECLARE i INT DEFAULT 0;
    DECLARE db_count INT;
    
    -- Insert the server
    INSERT INTO database_servers (name, type, host, port, username, password, enabled)
    VALUES (p_name, p_type, p_host, p_port, p_username, p_password, TRUE);
    
    SET server_id = LAST_INSERT_ID();
    
    -- Insert database filters if provided
    IF p_databases IS NOT NULL THEN
        SET db_count = JSON_LENGTH(p_databases);
        WHILE i < db_count DO
            SET db_name = JSON_UNQUOTE(JSON_EXTRACT(p_databases, CONCAT('$[', i, ']')));
            INSERT INTO database_filters (server_id, filter_type, database_name)
            VALUES (server_id, 'include', db_name);
            SET i = i + 1;
        END WHILE;
    END IF;
END//
DELIMITER ;

-- Create a stored procedure to get complete configuration as JSON
DELIMITER //
CREATE PROCEDURE get_configuration_json()
BEGIN
    SELECT JSON_OBJECT(
        'global', (
            SELECT JSON_OBJECTAGG(`key`, 
                CASE `type`
                    WHEN 'boolean' THEN CAST(`value` AS JSON)
                    WHEN 'integer' THEN CAST(`value` AS SIGNED)
                    WHEN 'json' THEN CAST(`value` AS JSON)
                    ELSE `value`
                END
            )
            FROM global_config
        ),
        'servers', (
            SELECT JSON_ARRAYAGG(
                JSON_OBJECT(
                    'id', s.id,
                    'name', s.name,
                    'type', s.type,
                    'host', s.host,
                    'port', s.port,
                    'username', s.username,
                    'password', s.password,
                    'enabled', s.enabled,
                    'databases', (
                        SELECT JSON_ARRAYAGG(database_name)
                        FROM database_filters
                        WHERE server_id = s.id AND filter_type = 'include'
                    )
                )
            )
            FROM database_servers s
            WHERE s.enabled = TRUE
        ),
        'storage', (
            SELECT JSON_OBJECTAGG(name, 
                JSON_MERGE_PATCH(
                    JSON_OBJECT('type', type, 'enabled', enabled),
                    config
                )
            )
            FROM storage_configs
            WHERE enabled = TRUE
        ),
        'schedules', (
            SELECT JSON_ARRAYAGG(
                JSON_OBJECT(
                    'name', s.name,
                    'type', s.backup_type,
                    'cron', s.cron_expression,
                    'retention', (
                        SELECT JSON_OBJECTAGG(
                            st.name,
                            JSON_OBJECT(
                                'duration', r.retention_duration,
                                'keepForever', r.keep_forever
                            )
                        )
                        FROM retention_policies r
                        JOIN storage_configs st ON r.storage_id = st.id
                        WHERE r.schedule_id = s.id AND r.enabled = TRUE
                    )
                )
            )
            FROM backup_schedules s
            WHERE s.enabled = TRUE
        )
    ) AS configuration;
END//
DELIMITER ;