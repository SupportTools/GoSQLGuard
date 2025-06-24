-- Initialize default backup schedules in the database
-- This script populates the backup_schedules table with default values

-- Insert default schedules
INSERT INTO backup_schedules (id, name, backup_type, cron_expression, enabled, created_at, updated_at)
VALUES 
    (UUID(), 'manual', 'manual', '', true, NOW(), NOW()),
    (UUID(), 'hourly', 'hourly', '0 * * * *', true, NOW(), NOW()),
    (UUID(), 'daily', 'daily', '0 2 * * *', true, NOW(), NOW()),
    (UUID(), 'weekly', 'weekly', '0 3 * * 0', true, NOW(), NOW())
ON DUPLICATE KEY UPDATE updated_at = NOW();

-- Insert retention policies for each schedule
INSERT INTO schedule_retention_policies (schedule_id, storage_type, duration, keep_forever, created_at)
SELECT 
    s.id,
    'local',
    CASE s.backup_type
        WHEN 'manual' THEN '7d'
        WHEN 'hourly' THEN '24h'
        WHEN 'daily' THEN '7d'
        WHEN 'weekly' THEN '30d'
    END,
    false,
    NOW()
FROM backup_schedules s
WHERE NOT EXISTS (
    SELECT 1 FROM schedule_retention_policies 
    WHERE schedule_id = s.id AND storage_type = 'local'
);

-- Optional: Add S3 retention policies (disabled by default)
INSERT INTO schedule_retention_policies (schedule_id, storage_type, duration, keep_forever, created_at)
SELECT 
    s.id,
    's3',
    CASE s.backup_type
        WHEN 'manual' THEN '30d'
        WHEN 'hourly' THEN '7d'
        WHEN 'daily' THEN '30d'
        WHEN 'weekly' THEN '90d'
    END,
    false,
    NOW()
FROM backup_schedules s
WHERE NOT EXISTS (
    SELECT 1 FROM schedule_retention_policies 
    WHERE schedule_id = s.id AND storage_type = 's3'
);