#!/bin/bash
# Example deployment script using vault secrets

set -e

echo "🚀 Deploying application..."

# Get secrets from vault
echo "Fetching secrets..."
DB_PASSWORD=$(vault-get db_password)
API_KEY=$(vault-get openai_api_key)
CLOUDFLARE_TOKEN=$(vault-get cloudflare_api_token)

# Export as environment variables
export DATABASE_URL="postgresql://myapp:$DB_PASSWORD@localhost:5432/myapp"
export OPENAI_API_KEY="$API_KEY"
export CLOUDFLARE_API_TOKEN="$CLOUDFLARE_TOKEN"

# Deploy with docker-compose
echo "Starting containers..."
docker-compose up -d

echo "✓ Deployment complete!"
echo ""
echo "Services:"
docker-compose ps
