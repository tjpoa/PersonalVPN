# Security Policy

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability. Use a private GitHub Security Advisory for this repository (or the project maintainer's private security channel once it is published) and include:

- affected commit, component, and platform;
- concise reproduction steps and required privileges;
- security impact and a suggested severity;
- a minimal proof of concept that contains no real user data.

Never include private keys, access tokens, production endpoints, client identifiers, packet captures, or personal data in a report. Replace them with clearly marked fixtures.

Reports are triaged confidentially. The maintainer should acknowledge receipt within 7 days, coordinate a fix and disclosure date with the reporter, and credit the reporter only with permission. Emergency issues affecting an active deployment take priority over feature work.

## Supported releases

Only unreleased development builds and the latest signed release receive security fixes. Do not use staging images, lab profiles, or unsigned native artifacts as production VPN clients.

## Security expectations for contributors

Changes must preserve HTTPS/TLS 1.3, gateway mTLS, local private-key storage, strict input validation, redirect rejection, least-privilege services, and redacted logging. Run `scripts/verify.ps1` and the relevant test script before submitting a security-sensitive change. Do not run the privileged WireGuard lab on a host system.
