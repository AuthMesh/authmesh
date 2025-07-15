# Security Policy

## Supported Versions

We release security updates for the latest major and minor versions of AuthMesh. Please ensure you are using a supported version before reporting vulnerabilities.

| Version | Supported          |
| ------- | ----------------- |
| 1.x     | :white_check_mark: |
| <1.0    | :x:               |

## Reporting a Vulnerability

If you discover a security vulnerability, **do not open a public issue**. Instead, please follow these steps:

1. **Email us at:** security@authmesh.dev
2. **Use the Security Report Template:** `.github/ISSUE_TEMPLATE/security_report.md` (for private, responsible disclosure)
3. **Include:**
   - A clear description of the vulnerability
   - Steps to reproduce
   - Impact assessment
   - Any relevant logs or screenshots

We will respond within 3 business days and coordinate a fix with you. All reports are handled confidentially.

## Security Process

- All code changes are scanned with [gosec](https://github.com/securego/gosec), [CodeQL](https://github.com/github/codeql), [Trivy](https://github.com/aquasecurity/trivy), and [Gitleaks](https://github.com/gitleaks/gitleaks).
- Dependencies are checked for vulnerabilities using [nancy](https://github.com/sonatype-nexus-community/nancy).
- Docker images are scanned for vulnerabilities and secrets.
- Security advisories are published for all confirmed vulnerabilities.

## Responsible Disclosure

We follow [Coordinated Vulnerability Disclosure](https://www.first.org/global/sigs/vulnerability-coordination) best practices. Please give us a reasonable time to address the issue before public disclosure.

## Security Best Practices

- Always use the latest release
- Run AuthMesh behind a secure reverse proxy
- Use strong secrets and rotate them regularly
- Enable TLS for all network traffic
- Monitor logs and metrics for suspicious activity
- Restrict access to sensitive endpoints

## Hall of Fame

We recognize and thank all security researchers who responsibly disclose vulnerabilities. With your help, we make AuthMesh safer for everyone.

## Questions?

For any security-related questions, contact: security@authmesh.dev
