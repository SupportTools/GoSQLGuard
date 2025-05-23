#!/bin/bash
# Script to set up test databases for GoSQLGuard

echo "Creating test databases for GoSQLGuard..."

# Wait for MySQL to be ready
echo "Waiting for MySQL to be ready..."
# Wait a bit longer for MySQL to become fully operational
sleep 10
until docker exec gosqlguard-mysql mysqladmin ping -h localhost -u root -proot_password --silent; do
    echo "MySQL is unavailable - sleeping"
    sleep 5
done

echo "MySQL is ready. Creating additional databases..."

# Create the db2 and db3 databases (db1 is created automatically by Docker)
echo "Creating databases and setting permissions..."
docker exec gosqlguard-mysql mysql -u root -proot_password -e "
CREATE DATABASE IF NOT EXISTS db1;
CREATE DATABASE IF NOT EXISTS db2;
CREATE DATABASE IF NOT EXISTS db3;

-- Grant permissions to the backup user
GRANT ALL PRIVILEGES ON *.* TO 'backup-user'@'%';
FLUSH PRIVILEGES;
"

# Verify the databases exist
echo "Verifying databases..."
docker exec gosqlguard-mysql mysql -u root -proot_password -e "SHOW DATABASES;"

# Create some test tables and data
echo "Creating test tables and data..."
docker exec gosqlguard-mysql mysql -u root -proot_password -e "
-- DB1 Tables
USE db1;
CREATE TABLE IF NOT EXISTS test_table1 (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO test_table1 (name) VALUES ('Test Record 1'), ('Test Record 2');

-- DB2 Tables
USE db2;
CREATE TABLE IF NOT EXISTS test_table2 (
    id INT AUTO_INCREMENT PRIMARY KEY,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO test_table2 (description) VALUES ('Test Description 1'), ('Test Description 2');

-- DB3 Tables
USE db3;
CREATE TABLE IF NOT EXISTS test_table3 (
    id INT AUTO_INCREMENT PRIMARY KEY,
    value DECIMAL(10,2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO test_table3 (value) VALUES (123.45), (678.90);
"

# Verify tables
echo "Verifying tables in db1..."
docker exec gosqlguard-mysql mysql -u root -proot_password db1 -e "SHOW TABLES;"
echo "Verifying tables in db2..."
docker exec gosqlguard-mysql mysql -u root -proot_password db2 -e "SHOW TABLES;"
echo "Verifying tables in db3..."
docker exec gosqlguard-mysql mysql -u root -proot_password db3 -e "SHOW TABLES;"

# Verify backup user access
echo "Verifying backup user access..."
docker exec gosqlguard-mysql mysql -u backup-user -ptest-password -e "
SELECT DATABASE();
SHOW DATABASES;
"

echo "Databases created and initialized successfully!"
echo "Created databases: db1, db2, db3"
echo "Each database has test tables with sample data."
