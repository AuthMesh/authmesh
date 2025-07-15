---
name: 🐛 Bug Report
about: Create a report to help us improve AuthMesh
title: '[BUG] '
labels: ['bug', 'needs-triage']
assignees: ''
---

## 🐛 Bug Description

**Summary:**
A clear and concise description of what the bug is.

**Expected Behavior:**
What you expected to happen.

**Actual Behavior:**
What actually happened.

## 🔄 Reproduction Steps

1. Go to '...'
2. Configure '...'
3. Execute '...'
4. See error

**Minimal Reproduction:**
```go
// Please provide minimal code that reproduces the issue
package main

import "github.com/AuthMesh/authmesh/pkg/platform"

func main() {
    // Your reproduction code here
}
```

## 🌍 Environment

**AuthMesh Version:** `v0.x.x` (or commit hash)
**Go Version:** `go version go1.21.x`
**Operating System:** `Linux/macOS/Windows`
**Architecture:** `amd64/arm64`

**Dependencies:**
- Keycloak version: `xx.x.x`
- Redis version: `x.x.x`
- Framework: `Gin/Echo/Fiber/other`

**Configuration:**
```yaml
# Relevant configuration (remove sensitive data)
keycloak:
  url: "https://keycloak.example.com"
  realm: "my-realm"
```

## 📋 Additional Context

**Logs:**
```
# Relevant log output (remove sensitive data)
```

**Screenshots:**
If applicable, add screenshots to help explain your problem.

**Related Issues:**
- #123
- #456

## ✅ Checklist

- [ ] I have searched existing issues and this is not a duplicate
- [ ] I have provided a minimal reproduction case
- [ ] I have included relevant logs and configuration
- [ ] I have tested this with the latest version of AuthMesh
- [ ] I have removed any sensitive information from this report

## 🔍 Impact Assessment

**Severity:** `Critical/High/Medium/Low`
**Frequency:** `Always/Often/Sometimes/Rarely`
**Workaround Available:** `Yes/No`

**Impact Description:**
How does this bug affect your application or users?

---

**Note:** For security-related issues, please use our [Security Report Template](.github/ISSUE_TEMPLATE/security_report.md) instead.
