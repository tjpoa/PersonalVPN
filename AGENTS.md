# Repository Guidelines

## Project Structure & Module Organization

Repository combines Go services, native clients, and deployment documentation:

- `services/api/` — Go control plane, PostgreSQL store/migrations, gateway agent/reconciler.
- `api/` — versioned OpenAPI contract for control-plane and gateway-agent endpoints.
- `clients/android/`, `clients/windows/`, `clients/ios/` — platform-specific VPN foundations; Android TV uses the Android client.
- `deploy/` — Docker Compose staging configuration, secrets templates, and systemd unit/runbook.
- `tests/` — fixtures and isolated lab assets; package tests stay beside Go code as `*_test.go`.
- `scripts/` — setup and maintenance commands.
- `docs/` — architecture and operating guides.

Do not commit generated output, environments, credentials, keys, or provider configuration; extend `.gitignore` for new secret formats.

## Build, Test, and Development Commands

- `powershell -File scripts/verify.ps1` — checks secret files, key fields, tables, and dependencies.
- `powershell -File scripts/test-api.ps1` — runs Go formatting, vet, race-enabled tests, and Docker builds for API/agent binaries.
- `powershell -File scripts/test-db.ps1` — starts a temporary PostgreSQL 17 container, applies the migration, and runs persistence/allocator integration tests.
- `powershell -File scripts/test-android.ps1` — builds Android APKs, installs them on the selected `adb` device/emulator, and runs instrumentation directly.
- `powershell -File scripts/check-lab-prereqs.ps1` — read-only check before the privileged WireGuard lab.
- `& "$env:LOCALAPPDATA\PersonalVPN\dotnet8\dotnet.exe" build clients/windows/PersonalVpn.Client.csproj --configuration Release` — builds the Windows client.
- `docker compose -f deploy/docker-compose.staging.yml config --no-interpolate` — checks deployment configuration.
- `powershell -File scripts/run-lab.ps1` — runs the WireGuard namespace lab; review first because it requires privileged Docker.

Never run the lab directly on the Windows host or silently modify host networking/firewall settings.

## Coding Style & Naming Conventions

Run `gofmt` on Go changes; use lowercase package names and `PascalCase` exported identifiers. Keep OS/WireGuard integrations behind interfaces. Use idiomatic Kotlin, C#, and Swift formatting, `PascalCase` types, and `camelCase` members. Name documentation in `kebab-case`.

## Testing Guidelines

Name Go tests `Test<Behavior>` and keep them beside the package. Cover validation, authorization, reconciliation, and cleanup. Use fixtures, mocks, or isolated containers—never credentials or host-network changes.

## Commit & Pull Request Guidelines

No commit history; use short, imperative subjects, optionally with a Conventional Commit prefix, such as `feat: add WireGuard profile validation`.

Pull requests explain the problem and security/networking impact, link issues, list verification commands, and redact tokens, keys, endpoints, and IPs.

## Security & Configuration

Use `.env.example` as a template and inject secrets through deployment secrets or environment files. Do not weaken HTTPS, mTLS, certificate validation, redirect rejection, or least privilege.
