#!/bin/bash
# Generate development TLS certificates for Developer Mesh
# These are self-signed certificates for development only!

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
CERT_DIR="${CERT_DIR:-./certs/dev}"
DAYS_VALID=3650  # 10 years for dev certs
KEY_SIZE=4096
COUNTRY="US"
STATE="Development"
LOCALITY="DevCity"
ORGANIZATION="Developer Mesh Development"
ORGANIZATIONAL_UNIT="Development"
COMMON_NAME_CA="Developer Mesh Dev CA"

echo -e "${GREEN}🔐 Generating development TLS certificates...${NC}"

# Create certificate directory
mkdir -p "$CERT_DIR"

# Function to generate a key and certificate
generate_cert() {
    local cert_type=$1
    local common_name=$2
    local san=$3
    
    echo -e "${YELLOW}Generating $cert_type certificate...${NC}"
    
    # Generate private key
    openssl genrsa -out "$CERT_DIR/${cert_type}-key.pem" $KEY_SIZE 2>/dev/null
    
    # Create certificate signing request
    openssl req -new \
        -key "$CERT_DIR/${cert_type}-key.pem" \
        -out "$CERT_DIR/${cert_type}.csr" \
        -subj "/C=$COUNTRY/ST=$STATE/L=$LOCALITY/O=$ORGANIZATION/OU=$ORGANIZATIONAL_UNIT/CN=$common_name" \
        2>/dev/null
    
    # Create extensions file for SAN
    cat > "$CERT_DIR/${cert_type}-ext.cnf" <<EOF
subjectAltName = $san
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth, clientAuth
EOF
    
    # Sign the certificate with our CA
    openssl x509 -req \
        -in "$CERT_DIR/${cert_type}.csr" \
        -CA "$CERT_DIR/ca-cert.pem" \
        -CAkey "$CERT_DIR/ca-key.pem" \
        -CAcreateserial \
        -out "$CERT_DIR/${cert_type}-cert.pem" \
        -days $DAYS_VALID \
        -sha256 \
        -extfile "$CERT_DIR/${cert_type}-ext.cnf" \
        2>/dev/null
    
    # Clean up
    rm -f "$CERT_DIR/${cert_type}.csr" "$CERT_DIR/${cert_type}-ext.cnf"
    
    # Set appropriate permissions
    chmod 600 "$CERT_DIR/${cert_type}-key.pem"
    chmod 644 "$CERT_DIR/${cert_type}-cert.pem"
}

# Generate Root CA
echo -e "${YELLOW}Generating Root CA...${NC}"
openssl genrsa -out "$CERT_DIR/ca-key.pem" $KEY_SIZE 2>/dev/null

# Create self-signed CA certificate
openssl req -new -x509 \
    -key "$CERT_DIR/ca-key.pem" \
    -out "$CERT_DIR/ca-cert.pem" \
    -days $DAYS_VALID \
    -subj "/C=$COUNTRY/ST=$STATE/L=$LOCALITY/O=$ORGANIZATION/OU=$ORGANIZATIONAL_UNIT/CN=$COMMON_NAME_CA" \
    2>/dev/null

# Set CA cert permissions
chmod 600 "$CERT_DIR/ca-key.pem"
chmod 644 "$CERT_DIR/ca-cert.pem"

# Generate server certificate for API server
# Include all possible hostnames and IPs for development
generate_cert "server" "localhost" "DNS:localhost,DNS:*.localhost,DNS:127.0.0.1,DNS:host.docker.internal,DNS:mcp-server,DNS:rest-api,IP:127.0.0.1,IP:::1"

# Generate client certificate for database connections
generate_cert "client" "developer-mesh-client" "DNS:localhost,IP:127.0.0.1"

# Generate certificate for Redis/ElastiCache connections
# This is used when connecting through SSH tunnel
generate_cert "redis" "redis.localhost" "DNS:localhost,DNS:127.0.0.1,DNS:*.cache.amazonaws.com,IP:127.0.0.1"

# Create a combined CA bundle for applications that need it
cat "$CERT_DIR/ca-cert.pem" > "$CERT_DIR/ca-bundle.pem"

# Create a PKCS12 bundle for applications that need it (like some Java apps)
echo -e "${YELLOW}Creating PKCS12 bundles...${NC}"
openssl pkcs12 -export \
    -out "$CERT_DIR/server.p12" \
    -inkey "$CERT_DIR/server-key.pem" \
    -in "$CERT_DIR/server-cert.pem" \
    -certfile "$CERT_DIR/ca-cert.pem" \
    -passout pass:developer-mesh \
    2>/dev/null

# Generate a truststore for Java applications
if command -v keytool &> /dev/null; then
    echo -e "${YELLOW}Creating Java truststore...${NC}"
    keytool -import -trustcacerts \
        -keystore "$CERT_DIR/truststore.jks" \
        -storepass developer-mesh \
        -alias developer-mesh-ca \
        -file "$CERT_DIR/ca-cert.pem" \
        -noprompt \
        2>/dev/null || true
fi

# Create environment variable export file
cat > "$CERT_DIR/env-exports.sh" <<EOF
# Source this file to set certificate environment variables
# source $CERT_DIR/env-exports.sh

# CA Certificate
export TLS_CA_CERT="$CERT_DIR/ca-cert.pem"
export TLS_CA_BUNDLE="$CERT_DIR/ca-bundle.pem"

# Server certificates (for API server HTTPS)
export TLS_SERVER_CERT="$CERT_DIR/server-cert.pem"
export TLS_SERVER_KEY="$CERT_DIR/server-key.pem"
export TLS_CERT_FILE="$CERT_DIR/server-cert.pem"
export TLS_KEY_FILE="$CERT_DIR/server-key.pem"

# Client certificates (for database connections)
export TLS_CLIENT_CERT="$CERT_DIR/client-cert.pem"
export TLS_CLIENT_KEY="$CERT_DIR/client-key.pem"
export DATABASE_TLS_CERT="$CERT_DIR/client-cert.pem"
export DATABASE_TLS_KEY="$CERT_DIR/client-key.pem"
export DATABASE_TLS_CA="$CERT_DIR/ca-cert.pem"

# Redis/ElastiCache certificates
export REDIS_TLS_CERT="$CERT_DIR/redis-cert.pem"
export REDIS_TLS_KEY="$CERT_DIR/redis-key.pem"
export REDIS_CA_CERT="$CERT_DIR/ca-cert.pem"

# RDS certificates (using the same client cert)
export RDS_CA_CERT="$CERT_DIR/ca-cert.pem"

# Java truststore (if needed)
export TRUSTSTORE_PATH="$CERT_DIR/truststore.jks"
export TRUSTSTORE_PASSWORD="developer-mesh"
EOF

# Create .env additions file
cat > "$CERT_DIR/env-additions.txt" <<EOF
# Development TLS Certificates
# Add these to your .env file

# CA Certificate
TLS_CA_CERT="$CERT_DIR/ca-cert.pem"
TLS_CA_BUNDLE="$CERT_DIR/ca-bundle.pem"

# Server certificates (for API server HTTPS)
TLS_SERVER_CERT="$CERT_DIR/server-cert.pem"
TLS_SERVER_KEY="$CERT_DIR/server-key.pem"
TLS_CERT_FILE="$CERT_DIR/server-cert.pem"
TLS_KEY_FILE="$CERT_DIR/server-key.pem"

# Client certificates (for database connections)
TLS_CLIENT_CERT="$CERT_DIR/client-cert.pem"
TLS_CLIENT_KEY="$CERT_DIR/client-key.pem"
DATABASE_TLS_CERT="$CERT_DIR/client-cert.pem"
DATABASE_TLS_KEY="$CERT_DIR/client-key.pem"
DATABASE_TLS_CA="$CERT_DIR/ca-cert.pem"

# Redis/ElastiCache certificates
REDIS_TLS_CERT="$CERT_DIR/redis-cert.pem"
REDIS_TLS_KEY="$CERT_DIR/redis-key.pem"
REDIS_CA_CERT="$CERT_DIR/ca-cert.pem"

# RDS certificates
RDS_CA_CERT="$CERT_DIR/ca-cert.pem"
EOF

# Summary
echo -e "${GREEN}✅ Development certificates generated successfully!${NC}"
echo
echo "Certificate directory: $CERT_DIR"
echo
echo "Generated files:"
echo "  - CA certificate: ca-cert.pem"
echo "  - Server certificate: server-cert.pem, server-key.pem"
echo "  - Client certificate: client-cert.pem, client-key.pem"
echo "  - Redis certificate: redis-cert.pem, redis-key.pem"
echo "  - PKCS12 bundle: server.p12 (password: developer-mesh)"
if [ -f "$CERT_DIR/truststore.jks" ]; then
    echo "  - Java truststore: truststore.jks (password: developer-mesh)"
fi
echo
echo -e "${YELLOW}To use these certificates:${NC}"
echo "1. Add the contents of $CERT_DIR/env-additions.txt to your .env file"
echo "2. Or source the environment variables:"
echo "   source $CERT_DIR/env-exports.sh"
echo
echo -e "${YELLOW}For AWS ElastiCache through SSH tunnel:${NC}"
echo "The Redis certificates are configured to work with localhost connections."
echo "Keep 'insecure_skip_verify: true' in development config when using SSH tunnel."
echo
echo -e "${RED}⚠️  These are self-signed certificates for DEVELOPMENT ONLY!${NC}"
echo -e "${RED}Never use these in production. Use cert-manager.io in Kubernetes.${NC}"