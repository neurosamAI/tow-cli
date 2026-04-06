# Security Policy

> **Tow** is created by [Murry Jeong (comchangs)](https://github.com/comchangs) and supported by [neurosam.AI](https://neurosam.ai). We take security seriously.

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| 0.4.x   | :white_check_mark: |
| 0.3.x   | :white_check_mark: |
| 0.2.x   | :x:                |
| 0.1.x   | :x:                |

## Reporting a Vulnerability

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, please report them via email to:

**oss@neurosam.ai**

Please include:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

## Response Timeline

- **Acknowledgment**: Within 48 hours
- **Assessment**: Within 1 week
- **Fix & Disclosure**: Coordinated with reporter

## Security Best Practices

When using Tow, please follow the security guidelines in our [documentation](https://tow-cli.neurosam.ai/security):

- Never commit SSH keys or passwords to `tow.yaml`
- Use environment variables (`${VAR}`) for sensitive values
- Add `tow.local.yaml` to `.gitignore`
- Enable branch policies for production environments
- Maintain `~/.ssh/known_hosts` for host key verification

---

<p align="center">
  <sub><a href="https://neurosam.ai">neurosam.AI</a></sub>
</p>
