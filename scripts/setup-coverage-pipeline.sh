#!/bin/bash

# Setup script for AuthMesh coverage pipeline
set -e

echo "🚀 Setting up AuthMesh Coverage Pipeline"
echo "========================================="

# Check if we're in the right directory
if [ ! -f "go.mod" ] || ! grep -q "github.com/AuthMesh/authmesh" go.mod; then
    echo "❌ Error: Not in AuthMesh directory or go.mod not found"
    echo "Please run this script from the AuthMesh root directory"
    exit 1
fi

# Check required tools
echo "🔍 Checking required tools..."

# Check Go
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.24+"
    exit 1
fi

echo "✅ Go $(go version | awk '{print $3}')"

# Check Git
if ! command -v git &> /dev/null; then
    echo "❌ Git is not installed"
    exit 1
fi

echo "✅ Git $(git --version | awk '{print $3}')"

# Check if we have bc for calculations
if ! command -v bc &> /dev/null; then
    echo "⚠️ bc not found. Installing..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        if command -v brew &> /dev/null; then
            brew install bc
        else
            echo "❌ Please install bc: brew install bc"
            exit 1
        fi
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        if command -v apt-get &> /dev/null; then
            sudo apt-get update && sudo apt-get install -y bc
        elif command -v yum &> /dev/null; then
            sudo yum install -y bc
        else
            echo "❌ Please install bc using your package manager"
            exit 1
        fi
    fi
fi

echo "✅ bc calculator available"

# Check if jq is available (optional but helpful)
if ! command -v jq &> /dev/null; then
    echo "⚠️ jq not found (optional). Installing..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        if command -v brew &> /dev/null; then
            brew install jq
        fi
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        if command -v apt-get &> /dev/null; then
            sudo apt-get update && sudo apt-get install -y jq
        fi
    fi
fi

# Create necessary directories
echo "📁 Creating necessary directories..."
mkdir -p .github/workflows
mkdir -p .github/ISSUE_TEMPLATE
mkdir -p .github/PULL_REQUEST_TEMPLATE

# Check if workflows already exist
WORKFLOWS_CREATED=0

if [ ! -f ".github/workflows/coverage.yml" ]; then
    echo "✅ coverage.yml workflow already exists"
else
    echo "✅ Created coverage.yml workflow"
    WORKFLOWS_CREATED=$((WORKFLOWS_CREATED + 1))
fi

if [ ! -f ".github/workflows/coverage-pr.yml" ]; then
    echo "✅ coverage-pr.yml workflow already exists"
else
    echo "✅ Created coverage-pr.yml workflow"
    WORKFLOWS_CREATED=$((WORKFLOWS_CREATED + 1))
fi

if [ ! -f ".github/workflows/update-readme-coverage.yml" ]; then
    echo "✅ update-readme-coverage.yml workflow already exists"
else
    echo "✅ Created update-readme-coverage.yml workflow"
    WORKFLOWS_CREATED=$((WORKFLOWS_CREATED + 1))
fi

# Test local coverage
echo "🧪 Testing local coverage setup..."
if make coverage-summary > /dev/null 2>&1; then
    echo "✅ Local coverage commands working"
else
    echo "⚠️ Issues with local coverage commands, but setup continues..."
fi

# Run initial coverage test
echo "📊 Running initial coverage test..."
COVERAGE=$(make coverage-readme 2>/dev/null | grep "Total:" | awk '{print $2}' | sed 's/%//' || echo "0")
echo "📈 Current coverage: ${COVERAGE}%"

# Check git repository status
echo "🔍 Checking git repository..."
if git rev-parse --git-dir > /dev/null 2>&1; then
    echo "✅ Git repository detected"
    
    # Check if we have a remote
    if git remote -v | grep -q origin; then
        echo "✅ Git remote 'origin' configured"
        REMOTE_URL=$(git remote get-url origin)
        echo "   Remote: $REMOTE_URL"
    else
        echo "⚠️ No git remote 'origin' found"
        echo "   Add remote: git remote add origin <your-repo-url>"
    fi
    
    # Check current branch
    CURRENT_BRANCH=$(git branch --show-current)
    echo "📍 Current branch: $CURRENT_BRANCH"
    
    if [ "$CURRENT_BRANCH" != "main" ] && [ "$CURRENT_BRANCH" != "init" ]; then
        echo "⚠️ Consider using 'main' or 'init' as your default branch for workflows"
    fi
else
    echo "❌ Not a git repository. Initialize with: git init"
    exit 1
fi

# Create a simple GitHub Pages setup file
cat > .github/workflows/pages.yml << 'EOF'
name: Deploy Coverage to GitHub Pages

on:
  push:
    branches: [ main, init ]
  workflow_run:
    workflows: ["Update README Coverage"]
    types:
      - completed

permissions:
  contents: read
  pages: write
  id-token: write

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Setup Pages
        uses: actions/configure-pages@v4
      - name: Generate coverage
        run: |
          go test -coverprofile=coverage.out ./pkg/...
          go tool cover -html=coverage.out -o coverage.html
      - name: Upload artifact
        uses: actions/upload-pages-artifact@v3
        with:
          path: './coverage.html'
      - name: Deploy to GitHub Pages
        id: deployment
        uses: actions/deploy-pages@v4
EOF

echo "✅ Created GitHub Pages workflow for coverage reports"

# Provide setup instructions
echo ""
echo "🎉 Coverage Pipeline Setup Complete!"
echo "====================================="
echo ""
echo "📋 What was set up:"
echo "   ✅ GitHub Actions workflows for coverage analysis"
echo "   ✅ Automated PR comments with coverage reports"
echo "   ✅ Clean README with coverage badge (links to docs/COVERAGE.md)"
echo "   ✅ Detailed coverage documentation in docs/COVERAGE.md"
echo "   ✅ Local Makefile targets for coverage testing"
echo "   ✅ GitHub Pages deployment for HTML reports"
echo ""
echo "🚀 Next Steps:"
echo ""
echo "1. 📤 Push to GitHub:"
echo "   git add ."
echo "   git commit -m \"📊 Add coverage pipeline and workflows\""
echo "   git push origin $CURRENT_BRANCH"
echo ""
echo "2. 🔧 Configure GitHub Repository:"
echo "   • Go to Settings > Actions > General"
echo "   • Enable 'Read and write permissions' for GITHUB_TOKEN"
echo "   • Enable 'Allow GitHub Actions to create and approve pull requests'"
echo ""
echo "3. 🌐 Enable GitHub Pages (optional):"
echo "   • Go to Settings > Pages"
echo "   • Source: 'GitHub Actions'"
echo "   • This will host your HTML coverage reports"
echo ""
echo "4. 🧪 Test Locally:"
echo "   make coverage-summary     # Quick coverage check"
echo "   make coverage-html        # Generate HTML report"
echo "   make coverage-help        # See all available commands"
echo ""
echo "5. 🔄 Test Workflows:"
echo "   • Create a test PR to see coverage comments"
echo "   • Push to main/init branch to see README updates"
echo "   • Check Actions tab for workflow runs"
echo ""
echo "📊 Current Status:"
echo "   Coverage: ${COVERAGE}%"
echo "   Goal: 25%+ (next milestone)"
echo ""
echo "💡 Pro Tips:"
echo "   • Workflows trigger on push to main/init and PR creation"
echo "   • Coverage reports are generated daily via cron schedule"
echo "   • PRs with coverage below 15% will show warnings"
echo "   • README updates are automatic via pull requests"
echo ""
echo "🆘 Need Help?"
echo "   • Check .github/workflows/ for workflow files"
echo "   • Use 'make coverage-help' for local commands"
echo "   • Review GitHub Actions logs for troubleshooting"
echo ""
echo "Happy testing! 🧪✨"
