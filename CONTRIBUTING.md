# Contributing to AuthMesh

Thank you for your interest in contributing to AuthMesh! This document provides guidelines and information for contributors.

## 🤝 Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## 🚀 Getting Started

### Prerequisites

- **Go 1.21+** - Latest stable version recommended
- **Docker & Docker Compose** - For running dependencies
- **Make** - For development commands
- **Git** - Version control

### Development Setup

1. **Fork and Clone**
   ```bash
   git clone https://github.com/your-username/authmesh.git
   cd authmesh
   ```

2. **Install Dependencies**
   ```bash
   go mod download
   ```

3. **Start Development Environment**
   ```bash
   make dev-up
   ```

4. **Run Tests**
   ```bash
   make test
   ```

5. **Run Example Application**
   ```bash
   cd examples/basic-app
   docker-compose up
   ```

## 📋 How to Contribute

### Reporting Bugs

Use our [Bug Report Template](.github/ISSUE_TEMPLATE/bug_report.md) and include:

- **Clear Description**: What happened vs. what you expected
- **Environment**: Go version, OS, AuthMesh version
- **Reproduction Steps**: Minimal code to reproduce the issue
- **Logs/Output**: Error messages and relevant logs
- **Impact**: How this affects your use case

### Requesting Features

Use our [Feature Request Template](.github/ISSUE_TEMPLATE/feature_request.md) and include:

- **Use Case**: What problem does this solve?
- **Proposed Solution**: Your ideal implementation
- **Alternatives**: Other solutions you've considered
- **Examples**: Code examples or similar features in other libraries

### Security Issues

**DO NOT** open public issues for security vulnerabilities. Instead:

1. Use our [Security Report Template](.github/ISSUE_TEMPLATE/security_report.md)
2. Email: security@authmesh.dev
3. See [SECURITY.md](SECURITY.md) for our full security policy

## 🔧 Development Workflow

### Branch Strategy

- **main**: Production-ready code
- **develop**: Integration branch for features
- **feature/***: Individual feature development
- **hotfix/***: Critical bug fixes

### Pull Request Process

1. **Create Feature Branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make Changes**
   - Follow our coding standards
   - Add tests for new functionality
   - Update documentation as needed

3. **Test Your Changes**
   ```bash
   make test-all      # Run all tests
   make lint          # Run linters
   make security      # Security checks
   ```

4. **Submit Pull Request**
   - Use our [PR Template](.github/PULL_REQUEST_TEMPLATE.md)
   - Link related issues
   - Provide clear description of changes

### Code Review Criteria

✅ **Required for Approval:**
- All tests pass (unit, integration, E2E)
- Code coverage >80% for new code
- No linting errors or security issues
- Documentation updated
- Breaking changes clearly documented

## 📝 Coding Standards

### Go Style Guide

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use [golangci-lint](https://golangci-lint.run/) configuration
- Format code with `gofmt`
- Use meaningful variable and function names

### Documentation Standards

- **Go Doc Comments**: All exported functions, types, and constants
- **README Updates**: For significant feature additions
- **API Documentation**: Update docs/ for public API changes
- **Examples**: Include working code examples

### Testing Requirements

- **Unit Tests**: Co-located with source code (`*_test.go`)
- **Integration Tests**: In `tests/integration/`
- **E2E Tests**: In `tests/e2e/`
- **Coverage**: >80% for new code, >90% for critical paths
- **Benchmarks**: For performance-critical code

### Commit Message Format

```
type(scope): brief description

Detailed explanation of the change, including:
- What was changed and why
- Any breaking changes
- Issue references

Closes #123
```

**Types**: feat, fix, docs, style, refactor, perf, test, chore

## 🎯 Areas for Contribution

### High Priority
- **Performance Optimization**: JWT validation, rate limiting
- **Documentation**: API docs, tutorials, migration guides
- **Examples**: Real-world usage patterns
- **Testing**: Edge cases, stress tests, compatibility

### Medium Priority
- **Observability**: Enhanced metrics and tracing
- **Security**: Additional security middleware
- **Configuration**: Simplified configuration options
- **Integrations**: Support for additional frameworks

### Low Priority
- **Developer Experience**: Better error messages, tooling
- **CI/CD**: Build optimization, additional platforms
- **Maintenance**: Code cleanup, dependency updates

## 🛠️ Development Commands

```bash
# Development
make dev-up         # Start development environment
make dev-down       # Stop development environment
make dev-reset      # Reset development data

# Testing
make test           # Run unit tests
make test-integration # Run integration tests
make test-e2e       # Run E2E tests
make test-all       # Run all tests
make benchmark      # Run performance benchmarks

# Code Quality
make lint           # Run linters
make fmt            # Format code
make security       # Security scans
make coverage       # Generate coverage report

# Documentation
make docs           # Generate API documentation
make docs-serve     # Serve docs locally

# Release
make release-dry    # Dry run release process
make release        # Create release (maintainers only)
```

## 📚 Resources

### Documentation
- [Getting Started Guide](docs/getting-started.md)
- [API Reference](docs/api/)
- [Configuration Guide](docs/configuration.md)
- [Security Best Practices](docs/security.md)

### Community
- **GitHub Discussions**: For questions and ideas
- **Issues**: Bug reports and feature requests
- **Pull Requests**: Code contributions

### Learning
- [Go Documentation](https://golang.org/doc/)
- [JWT Best Practices](https://auth0.com/blog/a-look-at-the-latest-draft-for-jwt-bcp/)
- [Multi-Tenant Architecture](https://docs.microsoft.com/en-us/azure/architecture/guide/multitenant/)

## 🏆 Recognition

Contributors are recognized through:
- GitHub contributor graphs
- Release notes attribution
- Annual contributor highlights

## ❓ Questions?

- **Documentation Issues**: Open an issue with the `documentation` label
- **Development Questions**: Use GitHub Discussions
- **Security Concerns**: Follow our [Security Policy](SECURITY.md)

Thank you for helping make AuthMesh better! 🚀
