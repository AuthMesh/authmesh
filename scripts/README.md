# 📜 AuthMesh Scripts

This directory contains automation scripts for AuthMesh development and deployment.

## 📊 Coverage & Testing Scripts

### `setup-coverage-pipeline.sh`
**Automated Coverage Pipeline Setup**

Sets up the complete code coverage pipeline including:
- GitHub Actions workflows for coverage analysis
- Makefile with coverage commands
- Documentation generation
- Badge configuration

```bash
# Run the setup
./scripts/setup-coverage-pipeline.sh

# What it creates:
# ├── .github/workflows/update-readme-coverage.yml
# ├── .github/workflows/coverage-pr.yml  
# ├── Makefile (coverage targets)
# └── Documentation updates
```

**Features:**
- ✅ Automated coverage badge updates
- ✅ PR coverage comments
- ✅ Detailed coverage documentation
- ✅ CI/CD integration
- ✅ Local development commands

## 🔧 Usage

All scripts should be run from the project root:

```bash
# From /path/to/authmesh/
./scripts/setup-coverage-pipeline.sh
```

## 📝 Adding New Scripts

When adding new scripts:

1. **Make them executable:** `chmod +x scripts/your-script.sh`
2. **Document them:** Add description in this README
3. **Follow conventions:** Use kebab-case naming
4. **Add error handling:** Include proper error checking
5. **Test thoroughly:** Verify on clean environment

## 🗂️ Script Categories

- **Coverage & Testing:** `setup-coverage-pipeline.sh`
- **Deployment:** *(future scripts)*
- **Development:** *(future scripts)*
- **Utilities:** *(future scripts)*

---

*Keep scripts organized and well-documented for the development team.*
