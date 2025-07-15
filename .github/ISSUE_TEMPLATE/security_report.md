---
name: 🔒 Security Report
about: Report a security vulnerability (CONFIDENTIAL)
title: '[SECURITY] '
labels: ['security', 'critical', 'confidential']
assignees: ''
---

## 🔒 Security Vulnerability Report

> **⚠️ IMPORTANT:** This template is for reporting security vulnerabilities. If this is not a security issue, please use the [Bug Report template](bug_report.md) instead.

**Vulnerability Type:**
- [ ] Authentication Bypass
- [ ] Authorization Bypass  
- [ ] JWT Token Manipulation
- [ ] Injection Attack (SQL, LDAP, etc.)
- [ ] Cross-Site Scripting (XSS)
- [ ] Cross-Site Request Forgery (CSRF)
- [ ] Information Disclosure
- [ ] Denial of Service (DoS)
- [ ] Rate Limiting Bypass
- [ ] Tenant Isolation Breach
- [ ] Cryptographic Weakness
- [ ] Other: _______________

## 🎯 Affected Components

**AuthMesh Version:** `v0.x.x` (or commit hash)
**Affected Packages:**
- [ ] `pkg/auth`
- [ ] `pkg/middleware`
- [ ] `pkg/ratelimit`
- [ ] `pkg/keycloak`
- [ ] `pkg/observability`
- [ ] `pkg/platform`
- [ ] Other: _______________

**Affected Endpoints/Functions:**
- Function: `functionName()`
- Endpoint: `/api/path`
- Middleware: `MiddlewareName`

## 🔍 Vulnerability Details

**Summary:**
Brief description of the vulnerability (avoid sensitive details here).

**Attack Vector:**
How can this vulnerability be exploited?

**Impact Assessment:**
- **Confidentiality:** `None/Low/Medium/High`
- **Integrity:** `None/Low/Medium/High`
- **Availability:** `None/Low/Medium/High`
- **Scope:** `Single Tenant/Multiple Tenants/System-wide`

**CVSS Score (if known):** `X.X` (use https://nvd.nist.gov/vuln-metrics/cvss/v3-calculator)

## 🔄 Proof of Concept

**Steps to Reproduce:**
1. Configure AuthMesh with...
2. Send request to...
3. Observe behavior...

**Minimal Example:**
```bash
# Remove any sensitive data, but provide enough detail for reproduction
curl -X POST "https://example.com/api/endpoint" \
  -H "Authorization: Bearer [REDACTED]" \
  -d '{"malicious": "payload"}'
```

**Expected vs Actual Behavior:**
- Expected: Secure behavior
- Actual: Vulnerability manifested

## 🌍 Environment

**Deployment Environment:**
- [ ] Development
- [ ] Staging  
- [ ] Production
- [ ] Local Testing

**Infrastructure:**
- Kubernetes version: `x.x.x`
- Container runtime: `Docker/Podman`
- Load balancer: `Nginx/HAProxy/other`
- TLS termination: `Application/Load Balancer`

**Dependencies:**
- Keycloak version: `xx.x.x`
- Redis version: `x.x.x`
- Go version: `go1.21.x`

## 💊 Suggested Mitigation

**Immediate Workaround:**
Steps users can take to mitigate the issue immediately.

**Proposed Fix:**
Your suggestion for how this could be fixed (if any).

**Security Controls:**
Additional security measures that could prevent this class of vulnerability.

## 📋 Additional Information

**Discovery Method:**
- [ ] Security Testing
- [ ] Code Review
- [ ] Penetration Testing
- [ ] Bug Bounty
- [ ] User Report
- [ ] Automated Scanning
- [ ] Other: _______________

**Public Disclosure:**
- [ ] This vulnerability has NOT been disclosed publicly
- [ ] This vulnerability has been disclosed elsewhere: _______________

**Related CVEs:**
Any known CVEs related to this issue.

## ✅ Reporter Information

**Contact Information:**
- Email: your.email@example.com (for follow-up questions)
- Security researcher: Yes/No
- Organization: Your Company/Individual

**Disclosure Preference:**
- [ ] Coordinated disclosure (preferred)
- [ ] Public disclosure after fix
- [ ] Credit in security advisory
- [ ] No credit needed

## 🔐 Security Team Use Only

**Triage Status:**
- [ ] Confirmed
- [ ] Invalid
- [ ] Duplicate
- [ ] Needs more information

**Priority Level:**
- [ ] Critical (P0) - Immediate attention required
- [ ] High (P1) - Fix within 7 days
- [ ] Medium (P2) - Fix within 30 days
- [ ] Low (P3) - Fix within 90 days

**CVE Requested:** Yes/No
**CVE Number:** CVE-YYYY-NNNNN

---

## 🚨 Security Policy

By submitting this report, you acknowledge that you have read and agree to our [Security Policy](../SECURITY.md).

**Responsible Disclosure:** We ask that you give us a reasonable amount of time to address the issue before making any public disclosure.

**Response Time:** We will acknowledge receipt within 24 hours and provide an initial assessment within 72 hours.

---

**Thank you for helping keep AuthMesh secure! 🙏**
