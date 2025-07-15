# 🚀 AuthMesh Coverage Pipeline Setup Guide

## 📋 Overview

This guide sets up a comprehensive CI/CD pipeline for code coverage analysis, automated reporting, and README updates for the AuthMesh library.

## 🎯 What You Get

### 1. **Automated Coverage Analysis**
- ✅ Coverage reports generated on every push and PR
- ✅ Function-level coverage analysis
- ✅ HTML reports with visual coverage maps
- ✅ Coverage trend tracking over time

### 2. **GitHub Integration**
- ✅ Automatic PR comments with coverage details
- ✅ Clean README with coverage badge only
- ✅ Detailed coverage docs in `docs/COVERAGE.md`
- ✅ GitHub Pages deployment for HTML reports
- ✅ Coverage threshold enforcement (15% minimum)

### 3. **Local Development Tools**
- ✅ Makefile targets for quick coverage checks
- ✅ Watch mode for real-time coverage feedback
- ✅ Package-specific coverage analysis
- ✅ Badge generation for documentation

## 🚀 Quick Setup

### Option 1: Automated Setup
```bash
# Run the setup script
./scripts/setup-coverage-pipeline.sh
```

### Option 2: Manual Setup
1. Copy the workflow files to `.github/workflows/`
2. Copy the Makefile to the root directory
3. Configure GitHub repository settings
4. Push changes to trigger workflows

## 📁 Files Created

### GitHub Actions Workflows
```
.github/workflows/
├── coverage.yml                    # Comprehensive coverage analysis
├── coverage-pr.yml                 # PR-specific coverage comments
├── update-readme-coverage.yml      # Automated README updates
└── pages.yml                       # GitHub Pages deployment
```

### Local Development Tools
```
├── Makefile                              # Coverage commands and targets
├── scripts/setup-coverage-pipeline.sh   # Automated setup script
└── docs/COVERAGE_PIPELINE_GUIDE.md      # This documentation
```

## 🛠️ Local Commands

### Basic Coverage Commands
```bash
# Quick coverage check
make coverage

# Comprehensive coverage with all reports
make coverage-full

# Open HTML coverage report in browser
make coverage-html

# Show coverage summary with status indicators
make coverage-summary
```

### Development Commands
```bash
# Watch files and show coverage on changes (requires entr)
make coverage-watch

# Coverage for specific package
make coverage-package PKG=pkg/auth

# Clean coverage files
make coverage-clean
```

### CI/CD Commands
```bash
# Coverage check with threshold (15%)
make coverage-ci

# Generate coverage badge URL
make coverage-badge

# Generate coverage data for README updates
make coverage-readme
```

## 🔧 GitHub Repository Configuration

### 1. **Actions Permissions**
Go to `Settings > Actions > General`:
- ✅ Enable "Read and write permissions" for GITHUB_TOKEN
- ✅ Enable "Allow GitHub Actions to create and approve pull requests"

### 2. **GitHub Pages (Optional)**
Go to `Settings > Pages`:
- ✅ Source: "GitHub Actions"
- ✅ This will host HTML coverage reports at: `https://yourusername.github.io/authmesh/coverage.html`

### 3. **Branch Protection (Recommended)**
Go to `Settings > Branches`:
- ✅ Add rule for `main` branch
- ✅ Require status checks (coverage workflow)
- ✅ Require pull request reviews

## 📊 Workflow Triggers

### Coverage Analysis (`coverage.yml`)
```yaml
Triggers:
  - Push to main/init/develop branches
  - Pull requests to main/init
  - Daily cron schedule (2 AM UTC)
```

### PR Coverage (`coverage-pr.yml`)
```yaml
Triggers:
  - Pull request opened/updated
  - Synchronize events
```

### README Updates (`update-readme-coverage.yml`)
```yaml
Triggers:
  - Push to main/init branches
  - Daily cron schedule (6 AM UTC)
```

## 🎯 Coverage Thresholds and Goals

### Current Thresholds
- **Minimum:** 15% (CI fails below this)
- **Target:** 25% (next milestone)
- **Goal:** 70% (long-term target)

### Package-Specific Goals
| Package | Current | Target | Priority |
|---------|---------|--------|----------|
| pkg/auth | 3.6% | 60% | 🔴 Critical |
| pkg/keycloak | 42.9% | 70% | 🟢 Good |
| pkg/middleware | 3.3% | 50% | 🔴 Critical |
| pkg/ratelimit | 5.4% | 40% | 🟡 Medium |
| pkg/tlsutil | 53.8% | 80% | 🟢 Good |

## 📈 Badge Colors and Meanings

| Coverage | Color | Status | Action |
|----------|-------|--------|--------|
| 80%+ | ![Green](https://img.shields.io/badge/coverage-80%25-brightgreen) | Excellent | Maintain |
| 60-79% | ![Yellow-Green](https://img.shields.io/badge/coverage-70%25-green) | Good | Improve |
| 40-59% | ![Yellow](https://img.shields.io/badge/coverage-50%25-yellow) | Moderate | Focus |
| 20-39% | ![Orange](https://img.shields.io/badge/coverage-30%25-orange) | Low | Urgent |
| <20% | ![Red](https://img.shields.io/badge/coverage-10%25-red) | Critical | Immediate |

## 🔄 Automated PR Creation

The pipeline automatically creates PRs for:

### Coverage Updates
- **Title:** "📊 Automated Coverage Update: X.X%"
- **Labels:** `📊 coverage`, `🤖 automated`, `📈 improvement`
- **Content:** Coverage summary, package breakdown, trend analysis

### README Synchronization
- **Frequency:** After coverage changes on main/init branch
- **Auto-merge:** Can be configured for coverage improvements
- **Review:** Optional reviewer assignment available

## 🚨 Troubleshooting

### Common Issues

#### 1. **Workflows Not Running**
```bash
# Check workflow files syntax
yamllint .github/workflows/*.yml

# Verify branch names in workflow triggers
git branch -a
```

#### 2. **Permission Errors**
- Ensure GITHUB_TOKEN has write permissions
- Check repository settings for Actions permissions

#### 3. **Coverage Command Failures**
```bash
# Test local coverage
make coverage-summary

# Check Go module issues
go mod tidy
go mod verify
```

#### 4. **Badge Not Updating**
- Check if README has correct badge URLs
- Verify coverage-data.json is being generated
- Clear browser cache for badge images

### Debug Commands
```bash
# Check coverage file generation
ls -la coverage*

# Verify Go test output
go test -v ./pkg/...

# Check Makefile targets
make coverage-help
```

## 📚 Advanced Configuration

### Custom Coverage Thresholds
Edit `.github/workflows/coverage.yml`:
```yaml
env:
  COVERAGE_THRESHOLD: 20.0  # Change this value
```

### Package-Specific Thresholds
Add to Makefile:
```makefile
coverage-strict:
	@go test -coverprofile=coverage.out ./pkg/auth/...
	@COVERAGE=$$(go tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | sed 's/%//'); \
	if [ $$(echo "$$COVERAGE < 60" | bc -l) -eq 1 ]; then \
		echo "❌ Auth package coverage below 60%"; exit 1; \
	fi
```

### Custom Badge Styles
Modify badge URLs in workflows:
```bash
# Flat style
https://img.shields.io/badge/coverage-${COVERAGE}%25-${COLOR}?style=flat

# Flat-square style  
https://img.shields.io/badge/coverage-${COVERAGE}%25-${COLOR}?style=flat-square

# Plastic style
https://img.shields.io/badge/coverage-${COVERAGE}%25-${COLOR}?style=plastic
```

## 🎯 Next Steps After Setup

### 1. **Immediate (Week 1)**
- ✅ Verify all workflows are running
- ✅ Create a test PR to see coverage comments
- ✅ Check that README updates are working
- ✅ Add tests for critical auth functions

### 2. **Short-term (Week 2-4)**
- 🎯 Reach 25% overall coverage
- 🎯 Focus on pkg/auth and pkg/middleware
- 🎯 Add integration tests
- 🎯 Set up coverage trending

### 3. **Long-term (Month 1-3)**
- 🎯 Reach 70% overall coverage
- 🎯 Add performance benchmarks
- 🎯 Set up coverage regression prevention
- 🎯 Add mutation testing

## 🤝 Contributing

When contributing to AuthMesh:

1. **Before submitting PR:**
   ```bash
   make coverage-summary  # Check current coverage
   make test-all          # Run all tests
   ```

2. **During PR review:**
   - Check coverage comments on PR
   - Ensure coverage doesn't decrease significantly
   - Add tests for new functionality

3. **After merge:**
   - Monitor automated README updates
   - Check coverage trending
   - Review HTML reports for gaps

## 📞 Support

- **Issues:** Check GitHub Actions logs
- **Local problems:** Use `make coverage-help`
- **Documentation:** Review workflow files in `.github/workflows/`
- **Coverage reports:** Check `coverage.html` for detailed analysis

---

**🎉 Happy testing with automated coverage tracking!** 🧪✨
