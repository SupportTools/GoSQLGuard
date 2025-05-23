#!/bin/bash
# Script to set up PostgreSQL test databases for GoSQLGuard

echo "Creating PostgreSQL test databases for GoSQLGuard..."

# Wait for PostgreSQL to be ready
echo "Waiting for PostgreSQL to be ready..."
until docker exec gosqlguard-postgres pg_isready -U backup-user; do
    echo "PostgreSQL is unavailable - sleeping"
    sleep 2
done

echo "PostgreSQL is ready. Creating additional databases..."

# Create db2 and db3 databases (db1 is created automatically by Docker)
docker exec gosqlguard-postgres psql -U backup-user -c "CREATE DATABASE db2;"
docker exec gosqlguard-postgres psql -U backup-user -c "CREATE DATABASE db3;"

# Create test tables and data in db1
docker exec gosqlguard-postgres psql -U backup-user -d db1 -c "
CREATE TABLE IF NOT EXISTS test_table1 (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO test_table1 (name) VALUES ('PG Test Record 1'), ('PG Test Record 2');"

# Create test tables and data in db2
docker exec gosqlguard-postgres psql -U backup-user -d db2 -c "
CREATE TABLE IF NOT EXISTS test_table2 (
    id SERIAL PRIMARY KEY,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO test_table2 (description) VALUES ('PG Test Description 1'), ('PG Test Description 2');"

# Create test tables and data in db3
docker exec gosqlguard-postgres psql -U backup-user -d db3 -c "
CREATE TABLE IF NOT EXISTS test_table3 (
    id SERIAL PRIMARY KEY,
    value DECIMAL(10,2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO test_table3 (value) VALUES (123.45), (678.90);"

echo "PostgreSQL databases created and initialized successfully!"
echo "Created databases: db1, db2, db3"
echo "Each database has test tables with sample data."
