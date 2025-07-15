# Coverage testing targets for AuthMesh library
.PHONY: coverage coverage-html coverage-func coverage-summary coverage-ci test-all coverage-check coverage-badge

# Quick coverage check
coverage:
	@echo "🧪 Running tests with coverage..."
	@go test -cover ./pkg/...

# Generate comprehensive coverage reports
coverage-full:
	@echo "📊 Generating comprehensive coverage reports..."
	@go test -coverprofile=coverage.out -covermode=atomic ./pkg/...
	@go tool cover -func=coverage.out > coverage-func.txt
	@go tool cover -html=coverage.out -o coverage.html
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
	@go test -coverprofile=coverage.out ./pkg/... >/dev/null 2>&1
	@go tool cover -func=coverage.out

# Coverage summary with percentage
coverage-summary:
	@echo "📊 Coverage Summary"
	@echo "==================="
	@go test -coverprofile=coverage.out ./pkg/... >/dev/null 2>&1
	@TOTAL_COVERAGE=$$(go tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | sed 's/%//'); \
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
	@go tool cover -func=coverage.out | grep "pkg/" | awk '{print $$1 ": " $$3}' | sed 's/github.com\/AuthMesh\/authmesh\///'

# Coverage check with threshold (for CI)
coverage-ci:
	@echo "🎯 Coverage CI Check"
	@go test -coverprofile=coverage.out ./pkg/...
	@COVERAGE=$$(go tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | sed 's/%//'); \
	THRESHOLD=15; \
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
	@go test -coverprofile=coverage.out ./pkg/... >/dev/null 2>&1
	@COVERAGE=$$(go tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | sed 's/%//'); \
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

# Run all tests (unit + integration if available)
test-all:
	@echo "🧪 Running all tests with verbose output..."
	@go test -v ./pkg/...

# Coverage check for specific package
coverage-package:
	@if [ -z "$(PKG)" ]; then \
		echo "Usage: make coverage-package PKG=pkg/auth"; \
		exit 1; \
	fi
	@echo "📦 Coverage for $(PKG):"
	@go test -cover ./$(PKG)/...

# Clean coverage files
coverage-clean:
	@echo "🧹 Cleaning coverage files..."
	@rm -f coverage.out coverage.html coverage-func.txt
	@echo "✅ Coverage files cleaned"

# Coverage watch mode (requires entr: brew install entr)
coverage-watch:
	@echo "👀 Watching for changes and running coverage..."
	@echo "Press Ctrl+C to stop"
	@find ./pkg -name "*.go" | entr -s 'make coverage-summary'

# Generate coverage report for README
coverage-readme:
	@echo "📝 Generating coverage data for README..."
	@go test -coverprofile=coverage.out ./pkg/... >/dev/null 2>&1
	@TOTAL=$$(go tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | sed 's/%//'); \
	AUTH=$$(go tool cover -func=coverage.out | grep "pkg/auth" | tail -1 | awk '{print $$3}' | sed 's/%//' | head -1 || echo "0"); \
	KEYCLOAK=$$(go tool cover -func=coverage.out | grep "pkg/keycloak" | tail -1 | awk '{print $$3}' | sed 's/%//' | head -1 || echo "0"); \
	MIDDLEWARE=$$(go tool cover -func=coverage.out | grep "pkg/middleware" | tail -1 | awk '{print $$3}' | sed 's/%//' | head -1 || echo "0"); \
	RATELIMIT=$$(go tool cover -func=coverage.out | grep "pkg/ratelimit" | tail -1 | awk '{print $$3}' | sed 's/%//' | head -1 || echo "0"); \
	TLSUTIL=$$(go tool cover -func=coverage.out | grep "pkg/tlsutil" | tail -1 | awk '{print $$3}' | sed 's/%//' | head -1 || echo "0"); \
	echo "📊 Coverage Data:"; \
	echo "Total: $$TOTAL%"; \
	echo "Auth: $$AUTH%"; \
	echo "Keycloak: $$KEYCLOAK%"; \
	echo "Middleware: $$MIDDLEWARE%"; \
	echo "RateLimit: $$RATELIMIT%"; \
	echo "TLSUtil: $$TLSUTIL%"

# Help target
coverage-help:
	@echo "📚 AuthMesh Coverage Commands:"
	@echo ""
	@echo "Basic Commands:"
	@echo "  make coverage           - Quick coverage check"
	@echo "  make coverage-full      - Generate all coverage reports"
	@echo "  make coverage-html      - Generate and open HTML report"
	@echo "  make coverage-summary   - Show coverage summary with status"
	@echo ""
	@echo "Development Commands:"
	@echo "  make coverage-watch     - Watch files and show coverage (requires entr)"
	@echo "  make coverage-package PKG=pkg/auth - Coverage for specific package"
	@echo ""
	@echo "CI/CD Commands:"
	@echo "  make coverage-ci        - Coverage check with threshold (15%)"
	@echo "  make coverage-badge     - Generate badge URL"
	@echo "  make coverage-readme    - Generate data for README"
	@echo ""
	@echo "Utility Commands:"
	@echo "  make coverage-clean     - Clean coverage files"
	@echo "  make test-all           - Run all tests"
	@echo "  make coverage-help      - Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make coverage-package PKG=pkg/auth"
	@echo "  make coverage-full && open coverage.html"
