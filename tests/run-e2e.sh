#!/bin/bash

# E2E Test Runner for AuthMesh
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🧪 AuthMesh E2E Test Suite"
echo "=========================="

# Configuration
export APP_URL="http://localhost:8080"
export REDIS_ADDR="localhost:6379"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to wait for service
wait_for_service() {
    local url=$1
    local name=$2
    local max_attempts=30
    local attempt=1

    print_status $YELLOW "⏳ Waiting for $name to be ready..."
    
    while [ $attempt -le $max_attempts ]; do
        if curl -sf "$url" > /dev/null 2>&1; then
            print_status $GREEN "✅ $name is ready"
            return 0
        fi
        
        echo "   Attempt $attempt/$max_attempts..."
        sleep 2
        ((attempt++))
    done
    
    print_status $RED "❌ $name failed to start within timeout"
    return 1
}

# Function to cleanup
cleanup() {
    print_status $YELLOW "🧹 Cleaning up..."
    docker-compose -f docker-compose.test.yml down -v > /dev/null 2>&1 || true
}

# Set up cleanup trap
trap cleanup EXIT

# Start services
print_status $YELLOW "🚀 Starting test services..."
docker-compose -f docker-compose.test.yml up -d

# Wait for services to be ready
wait_for_service "$APP_URL/health" "Application"
wait_for_service "http://localhost:6379" "Redis" || true  # Redis doesn't have HTTP endpoint

# Small additional delay to ensure everything is fully ready
sleep 3

print_status $GREEN "🎯 All services ready, starting tests..."

# Run the tests
print_status $YELLOW "📋 Running E2E Tests..."

# Test categories
declare -a test_packages=(
    "./e2e"
)

# Run tests with proper environment
export CGO_ENABLED=0

for package in "${test_packages[@]}"; do
    print_status $YELLOW "🧪 Running tests in $package..."
    
    if go test -v -timeout=5m "$package"; then
        print_status $GREEN "✅ Tests in $package passed"
    else
        print_status $RED "❌ Tests in $package failed"
        exit 1
    fi
done

print_status $GREEN "🎉 All E2E tests passed!"

# Optional: Show container logs for debugging
if [ "${SHOW_LOGS:-}" = "true" ]; then
    print_status $YELLOW "📝 Container logs:"
    docker-compose -f docker-compose.test.yml logs
fi

print_status $GREEN "✨ E2E test suite completed successfully!"
