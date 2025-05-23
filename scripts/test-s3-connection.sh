#!/bin/bash
# Script to test S3 endpoint connectivity for GoSQLGuard

echo "=== Testing S3 Endpoint Connectivity ==="

S3_ENDPOINT="s3.support.tools:9000"
S3_BUCKET="gosqlguard"
S3_ACCESS_KEY="vuGNTXWipzZrWGNbBLWW"
S3_SECRET_KEY="O3KHT4VIkKcix02xbyZnRtAw2Nqe1jQ94hl6FDY9"
S3_REGION="us-central-1"
S3_PREFIX="controller1/mysql/backups"
USE_SSL="true"
CUSTOM_CA_PATH=""
SKIP_CERT_VALIDATION="false"

# First, install AWS CLI if not present
command -v aws >/dev/null 2>&1 || {
  echo "AWS CLI not found. Installing..."
  if command -v apt-get >/dev/null 2>&1; then
    # Debian/Ubuntu
    apt-get update && apt-get install -y awscli
  elif command -v yum >/dev/null 2>&1; then
    # RHEL/CentOS
    yum install -y awscli
  elif command -v apk >/dev/null 2>&1; then
    # Alpine
    apk add --no-cache aws-cli
  else
    echo "Unable to install AWS CLI. Please install it manually."
  fi
}

# Check direct HTTPS connection to endpoint
echo "Testing direct connection to S3 endpoint..."
curl -s -I "https://${S3_ENDPOINT}" && echo "✅ Endpoint is reachable" || echo "❌ Cannot reach endpoint"

# Test S3 connection using the container (for network connectivity)
echo "Testing S3 connection from GoSQLGuard container..."
if docker ps | grep -q gosqlguard-controller; then
  # Create a temporary AWS CLI config file in the container
  docker exec gosqlguard-controller sh -c "mkdir -p /root/.aws"
  docker exec gosqlguard-controller sh -c "cat > /root/.aws/credentials << EOF
[default]
aws_access_key_id = ${S3_ACCESS_KEY}
aws_secret_access_key = ${S3_SECRET_KEY}
EOF"

  docker exec gosqlguard-controller sh -c "cat > /root/.aws/config << EOF
[default]
region = ${S3_REGION}
EOF"

  # Test connection using curl
  docker exec gosqlguard-controller sh -c "curl -v https://${S3_ENDPOINT} --insecure"
  
  # Install AWS CLI in container if needed
  docker exec gosqlguard-controller sh -c "command -v aws >/dev/null 2>&1 || apk add --no-cache aws-cli"
  
  # Test S3 bucket connection
  docker exec gosqlguard-controller sh -c "aws s3 ls s3://${S3_BUCKET} --endpoint-url https://${S3_ENDPOINT} --no-verify-ssl" && {
    echo "✅ Successfully connected to S3 bucket ${S3_BUCKET}"
  } || {
    echo "❌ Failed to connect to S3 bucket ${S3_BUCKET}"
  }
  
  # Try creating a test file in the bucket
  echo "Trying to create a test file in S3 bucket..."
  docker exec gosqlguard-controller sh -c "echo 'GoSQLGuard test file' > /tmp/test.txt"
  docker exec gosqlguard-controller sh -c "aws s3 cp /tmp/test.txt s3://${S3_BUCKET}/test.txt --endpoint-url https://${S3_ENDPOINT} --no-verify-ssl" && {
    echo "✅ Successfully uploaded test file to S3 bucket"
  } || {
    echo "❌ Failed to upload test file to S3 bucket"
  }
else
  echo "⚠️ GoSQLGuard container not running, skipping container-based tests"
fi

# Check GoSQLGuard container configuration
echo ""
echo "Checking GoSQLGuard configuration for S3 connection..."
if ! docker ps | grep -q gosqlguard-controller; then
  echo "WARNING: GoSQLGuard controller is not running. Cannot verify S3 configuration."
else
  echo "GoSQLGuard container is running. Verifying environment variables..."
  s3_access_key=$(docker exec gosqlguard-controller sh -c "echo \$S3_ACCESS_KEY")
  s3_secret_key=$(docker exec gosqlguard-controller sh -c "echo \$S3_SECRET_KEY")
  
  echo "S3_ACCESS_KEY: ${s3_access_key:-[not set]}"
  echo "S3_SECRET_KEY: ${s3_secret_key:-[not set]}"
  
  # If environment variables not set, explicitly set them
  if [ -z "$s3_access_key" ] || [ -z "$s3_secret_key" ]; then
    echo "Setting S3 credentials in the container..."
    docker exec gosqlguard-controller sh -c "export S3_ACCESS_KEY=minioadmin S3_SECRET_KEY=minioadmin"
    
    # Verify they're set now
    echo "Verifying updated environment variables..."
    s3_access_key=$(docker exec gosqlguard-controller sh -c "echo \$S3_ACCESS_KEY")
    s3_secret_key=$(docker exec gosqlguard-controller sh -c "echo \$S3_SECRET_KEY")
    echo "Updated S3_ACCESS_KEY: ${s3_access_key:-[still not set]}"
    echo "Updated S3_SECRET_KEY: ${s3_secret_key:-[still not set]}"
  fi
  
  # Check if test-config.yaml is properly mounted
  echo "Verifying configuration file..."
  if docker exec gosqlguard-controller sh -c "ls -la /app/test-config.yaml"; then
    echo "Configuration file exists. Checking S3 configuration..."
    s3_config=$(docker exec gosqlguard-controller sh -c "grep -A10 's3:' /app/test-config.yaml")
    echo "$s3_config"
    
    # Fix the config file if needed
    if [[ "$s3_config" != *"accessKey: \"minioadmin\""* ]]; then
      echo "Updating S3 access keys in configuration file..."
      docker exec gosqlguard-controller sh -c "sed -i 's/accessKey:.*/accessKey: \"minioadmin\"/' /app/test-config.yaml"
      docker exec gosqlguard-controller sh -c "sed -i 's/secretKey:.*/secretKey: \"minioadmin\"/' /app/test-config.yaml"
      
      # Verify the changes
      s3_config=$(docker exec gosqlguard-controller sh -c "grep -A10 's3:' /app/test-config.yaml")
      echo "Updated configuration:"
      echo "$s3_config"
    fi
    
    if [[ "$s3_config" == *"enabled: true"* ]]; then
      echo "✅ S3 is enabled in the configuration."
    else
      echo "Enabling S3 in configuration..."
      docker exec gosqlguard-controller sh -c "sed -i 's/enabled: false/enabled: true/' /app/test-config.yaml"
      echo "✅ S3 is now enabled in the configuration."
    fi
  else
    echo "❌ Configuration file not found in container. Creating with proper S3 settings..."
    docker exec gosqlguard-controller sh -c "cat > /app/test-config.yaml << 'EOF'
# S3 storage settings
s3:
  enabled: true
  bucket: \"gosqlguard-backups\"
  region: \"us-east-1\"
  endpoint: \"http://minio:9000\"
  accessKey: \"minioadmin\"
  secretKey: \"minioadmin\"
  prefix: \"mysql/backups\"
  useSSL: false
EOF"
  fi
  
  # Test the S3 connection
  echo ""
  echo "Testing S3 connection from GoSQLGuard container..."
  docker exec gosqlguard-controller sh -c "curl -v http://minio:9000/"
fi

echo ""
echo "=== S3 Connection Test Complete ==="
echo "Endpoint: https://${S3_ENDPOINT}"
echo "Bucket: ${S3_BUCKET}"
echo "Region: ${S3_REGION}"
echo "Prefix: ${S3_PREFIX}"
echo "SSL Enabled: ${USE_SSL}"
echo "Custom CA Path: ${CUSTOM_CA_PATH:-None}"
echo "Skip Certificate Validation: ${SKIP_CERT_VALIDATION}"
