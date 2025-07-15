# AuthMesh Library Makefile
# This file contains all development commands referenced in CONTRIBUTING.md

# Variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=authmesh
COVERAGE_OUT=coverage.out

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse HEAD)
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Phony targets
.PHONY: dev-up dev-down dev-reset test test-integration test-e2e test-all benchmark \
        lint fmt security coverage build clean install help \
        docs docs-serve release-dry release coverage-html coverage-func \
        coverage-summary coverage-ci coverage-check coverage-badge

## Development Environment
dev-up:
	@echo "🚀 Starting development environment..."
	@cd examples/basic-app && docker-compose up -d
	@echo "✅ Development environment started"
	@echo "   🔗 Keycloak: http://localhost:9443"
	@echo "   📊 Redis: localhost:6379"
	@echo "   📈 Prometheus: http://localhost:9090"

dev-down:
	@echo "🛑 Stopping development environment..."
	@cd examples/basic-app && docker-compose down
	@echo "✅ Development environment stopped"

dev-reset:
	@echo "🔄 Resetting development environment..."
	@cd examples/basic-app && docker-compose down -v
	@cd examples/basic-app && docker-compose up -d
	@echo "✅ Development environment reset"

## Testing
test:
	@echo "🧪 Running unit tests..."
	@$(GOTEST) -v ./pkg/...

test-integration:
	@echo "🔗 Running integration tests..."
	@$(GOTEST) -v ./tests/integration/... -tags=integration

test-e2e:
	@echo "🎯 Running E2E tests..."
	@$(GOTEST) -v ./tests/e2e/... -tags=e2e

test-all:
	@echo "🧪 Running all tests with verbose output..."
	@$(GOTEST) -v ./pkg/... ./tests/...

benchmark:
	@echo "⚡ Running performance benchmarks..."
	@$(GOTEST) -bench=. -benchmem ./tests/benchmarks/...

## Code Quality
lint:
	@echo "🔍 Running linters..."
	@golangci-lint run ./...

fmt:
	@echo "✨ Formatting code..."
	@gofmt -s -w .
	@$(GOMOD) tidy

security:
	@echo "🔒 Running security scans..."
	@gosec ./...
	@nancy sleuth

## Coverage
## Coverage
coverage:
	@echo "🧪 Running tests with coverage..."
	@$(GOTEST) -cover ./pkg/...

# Generate comprehensive coverage reports
coverage-full:
	@echo "📊 Generating comprehensive coverage reports..."
	@$(GOTEST) -coverprofile=$(COVERAGE_OUT) -covermode=atomic ./pkg/...
	@go tool cover -func=$(COVERAGE_OUT) > coverage-func.txt
	@go tool cover -html=$(COVERAGE_OUT) -o coverage.html
	@echo "✅ Coverage reports generated:"
	@echo "   📄 coverage.html (open in browser)"
	@echo "   📊 coverage-func.txt (function details)"
	@echo "   💾 coverage.out (profile data)"

# Generate HTML coverage report
coverage-html: coverage-full
	@echo "🌐 Opening HTML coverage report..."
	@open coverage.html 2>/dev/null || xdg-open coverage.html 2>/dev/null || echo "📄 Open coverage.html in your browser"

# Function-level coverage report
coverage-func:
	@echo "📋 Function-level coverage report:"
	@$(GOTEST) -coverprofile=$(COVERAGE_OUT) ./pkg/... >/dev/null 2>&1
	@go tool cover -func=$(COVERAGE_OUT)

# Coverage summary with percentage
coverage-summary:
	@echo "📊 Coverage Summary"
	@echo "==================="
	@$(GOTEST) -coverprofile=$(COVERAGE_OUT) ./pkg/... >/dev/null 2>&1
	@TOTAL_COVERAGE=$$(go tool cover -func=$(COVERAGE_OUT) | tail -1 | awk '{print $$3}' | sed 's/%//'); \
	echo "Total Coverage: $$TOTAL_COVERAGE%"; \
	if [ $$(echo "$$TOTAL_COVERAGE >= 70" | bc -l) -eq 1 ]; then \
		echo "Status: 🟢 Excellent"; \
	elif [ $$(echo "$$TOTAL_COVERAGE >= 50" | bc -l) -eq 1 ]; then \
		echo "Status: 🟡 Good"; \
	elif [ $$(echo "$$TOTAL_COVERAGE >= 30" | bc -l) -eq 1 ]; then \
		echo "Status: 🟠 Needs Improvement"; \
	else \
		echo "Status: 🔴 Critical - Add More Tests"; \
	fi
	@echo ""
	@echo "📦 Package Breakdown:"
	@go tool cover -func=$(COVERAGE_OUT) | grep "pkg/" | awk '{print $$1 ": " $$3}' | sed 's/github.com\/AuthMesh\/authmesh\///'

# Coverage check with threshold (for CI)
coverage-ci:
	@echo "🎯 Coverage CI Check"
	@$(GOTEST) -coverprofile=$(COVERAGE_OUT) ./pkg/...
	@COVERAGE=$$(go tool cover -func=$(COVERAGE_OUT) | tail -1 | awk '{print $$3}' | sed 's/%//'); \
	THRESHOLD=80; \
	echo "Coverage: $$COVERAGE% | Threshold: $$THRESHOLD%"; \
	if [ $$(echo "$$COVERAGE >= $$THRESHOLD" | bc -l) -eq 1 ]; then \
		echo "✅ Coverage meets threshold"; \
		exit 0; \
	else \
		echo "❌ Coverage below threshold"; \
		exit 1; \
	fi

# Generate coverage badge URL
coverage-badge:
	@$(GOTEST) -coverprofile=$(COVERAGE_OUT) ./pkg/... >/dev/null 2>&1
	@COVERAGE=$$(go tool cover -func=$(COVERAGE_OUT) | tail -1 | awk '{print $$3}' | sed 's/%//'); \
	COLOR="red"; \
	if [ $$(echo "$$COVERAGE >= 80" | bc -l) -eq 1 ]; then \
		COLOR="brightgreen"; \
	elif [ $$(echo "$$COVERAGE >= 60" | bc -l) -eq 1 ]; then \
		COLOR="green"; \
	elif [ $$(echo "$$COVERAGE >= 40" | bc -l) -eq 1 ]; then \
		COLOR="yellow"; \
	elif [ $$(echo "$$COVERAGE >= 20" | bc -l) -eq 1 ]; then \
		COLOR="orange"; \
	fi; \
	echo "📛 Badge URL: https://img.shields.io/badge/coverage-$$COVERAGE%25-$$COLOR"

## Build & Install
build:
	@echo "🔨 Building AuthMesh..."
	@$(GOBUILD) -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o bin/$(BINARY_NAME) ./cmd/server

clean:
	@echo "🧹 Cleaning build artifacts..."
	@$(GOCLEAN)
	@rm -rf bin/ coverage.* *.out
	@echo "✅ Clean complete"

install:
	@echo "📦 Installing dependencies..."
	@$(GOMOD) download
	@$(GOMOD) tidy

## Documentation
docs:
	@echo "📚 Generating API documentation..."
	@godoc -http=:6060 &
	@echo "✅ Documentation server started at http://localhost:6060"

docs-serve:
	@echo "🌐 Serving documentation locally..."
	@godoc -http=:6060

## Release
release-dry:
	@echo "🧪 Dry run release process..."
	@goreleaser release --snapshot --clean --skip=publish

release:
	@echo "🚀 Creating release..."
	@goreleaser release --clean

## Utility Commands

## Utility Commands
# Coverage check for specific package
coverage-package:
	@if [ -z "$(PKG)" ]; then \
		echo "Usage: make coverage-package PKG=pkg/auth"; \
		exit 1; \
	fi
	@echo "📦 Coverage for $(PKG):"
	@$(GOTEST) -cover ./$(PKG)/...

# Clean coverage files
coverage-clean:
	@echo "🧹 Cleaning coverage files..."
	@rm -f $(COVERAGE_OUT) coverage.html coverage-func.txt
	@echo "✅ Coverage files cleaned"

# Coverage watch mode (requires entr: brew install entr)
coverage-watch:
	@echo "👀 Watching for changes and running coverage..."
	@echo "Press Ctrl+C to stop"
	@find ./pkg -name "*.go" | entr -s 'make coverage-summary'

# Generate coverage report for README
coverage-readme:
	@echo "📝 Generating coverage data for README..."
	@$(GOTEST) -coverprofile=$(COVERAGE_OUT) ./pkg/... >/dev/null 2>&1
	@TOTAL=$$(go tool cover -func=$(COVERAGE_OUT) | tail -1 | awk '{print $$3}' | sed 's/%//'); \
	AUTH=$$(go tool cover -func=$(COVERAGE_OUT) | grep "pkg/auth" | tail -1 | awk '{print $$3}' | sed 's/%//' | head -1 || echo "0"); \
	KEYCLOAK=$$(go tool cover -func=$(COVERAGE_OUT) | grep "pkg/keycloak" | tail -1 | awk '{print $$3}' | sed 's/%//' | head -1 || echo "0"); \
	MIDDLEWARE=$$(go tool cover -func=$(COVERAGE_OUT) | grep "pkg/middleware" | tail -1 | awk '{print $$3}' | sed 's/%//' | head -1 || echo "0"); \
	RATELIMIT=$$(go tool cover -func=$(COVERAGE_OUT) | grep "pkg/ratelimit" | tail -1 | awk '{print $$3}' | sed 's/%//' | head -1 || echo "0"); \
	TLSUTIL=$$(go tool cover -func=$(COVERAGE_OUT) | grep "pkg/tlsutil" | tail -1 | awk '{print $$3}' | sed 's/%//' | head -1 || echo "0"); \
	echo "📊 Coverage Data:"; \
	echo "Total: $$TOTAL%"; \
	echo "Auth: $$AUTH%"; \
	echo "Keycloak: $$KEYCLOAK%"; \
	echo "Middleware: $$MIDDLEWARE%"; \
	echo "RateLimit: $$RATELIMIT%"; \
	echo "TLSUtil: $$TLSUTIL%"

## Help
help:
	@echo "📚 AuthMesh Development Commands"
	@echo "=================================="
	@echo ""
	@echo "🚀 Development Environment:"
	@echo "  make dev-up          - Start development environment (Keycloak, Redis, etc.)"
	@echo "  make dev-down        - Stop development environment"
	@echo "  make dev-reset       - Reset development data"
	@echo ""
	@echo "🧪 Testing:"
	@echo "  make test            - Run unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make test-e2e        - Run E2E tests"
	@echo "  make test-all        - Run all tests"
	@echo "  make benchmark       - Run performance benchmarks"
	@echo ""
	@echo "🔍 Code Quality:"
	@echo "  make lint            - Run linters"
	@echo "  make fmt             - Format code"
	@echo "  make security        - Security scans"
	@echo "  make coverage        - Generate coverage report"
	@echo ""
	@echo "📊 Coverage Commands:"
	@echo "  make coverage-full   - Generate all coverage reports"
	@echo "  make coverage-html   - Generate and open HTML report"
	@echo "  make coverage-summary - Show coverage summary with status"
	@echo "  make coverage-ci     - Coverage check with 80% threshold"
	@echo ""
	@echo "🔨 Build & Install:"
	@echo "  make build           - Build AuthMesh binary"
	@echo "  make clean           - Clean build artifacts"
	@echo "  make install         - Install dependencies"
	@echo ""
	@echo "📚 Documentation:"
	@echo "  make docs            - Generate API documentation"
	@echo "  make docs-serve      - Serve docs locally"
	@echo ""
	@echo "🚀 Release:"
	@echo "  make release-dry     - Dry run release process"
	@echo "  make release         - Create release (maintainers only)"
	@echo ""
	@echo "❓ Help:"
	@echo "  make help            - Show this help message"

# Default target
.DEFAULT_GOAL := help
