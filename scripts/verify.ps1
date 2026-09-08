[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$failures = [System.Collections.Generic.List[string]]::new()

$forbiddenExtensions = @('.key', '.pem', '.p12', '.pfx', '.mobileprovision', '.tfstate')
$trackedCandidates = Get-ChildItem -LiteralPath $repoRoot -Recurse -File -Force |
    Where-Object {
        $_.FullName -notlike "*\.git\*" -and
        $_.FullName -notlike "*\tests\e2e\results\*" -and
        $_.FullName -notlike "*\deploy\secrets\*"
    }

foreach ($file in $trackedCandidates) {
    if ($forbiddenExtensions -contains $file.Extension.ToLowerInvariant()) {
        $failures.Add("Forbidden secret/state extension: $($file.FullName)")
    }
}

$openApiPath = Join-Path $repoRoot 'api/openapi.yaml'
$openApi = Get-Content -LiteralPath $openApiPath -Raw
if ($openApi -match '(?im)^\s*(privateKey|private_key):') {
    $failures.Add('OpenAPI must not expose a client private-key field.')
}
foreach ($requiredPlatform in @('windows', 'android', 'ios', 'android_tv')) {
    if ($openApi -notmatch [regex]::Escape($requiredPlatform)) {
        $failures.Add("OpenAPI is missing platform: $requiredPlatform")
    }
}
foreach ($requiredPath in @('/auth/register:', '/auth/login:', '/devices:', '/configuration:', '/agent/gateways/{gatewayId}/snapshots/next:', '/agent/gateways/{gatewayId}/snapshots/{deliveryId}/ack:')) {
    if ($openApi -notmatch [regex]::Escape($requiredPath)) {
        $failures.Add("OpenAPI is missing path: $requiredPath")
    }
}
if ($openApi -notmatch '(?im)^\s*- url: https://') {
    $failures.Add('OpenAPI must define an HTTPS server URL.')
}
if ($openApi -notmatch '(?im)^\s*gatewayMTLS:\s*$') {
    $failures.Add('OpenAPI is missing the gateway mTLS security scheme.')
}
if ($openApi -notmatch '(?im)^\s*type:\s*mutualTLS\s*$') {
    $failures.Add('Gateway security scheme must use mutualTLS.')
}

$androidEnginePath = Join-Path $repoRoot 'clients/android/app/src/main/java/com/personalvpn/client/WireGuardEngine.kt'
if (-not (Test-Path -LiteralPath $androidEnginePath)) {
    $failures.Add('Android WireGuard engine adapter is missing.')
} else {
    $androidEngine = Get-Content -LiteralPath $androidEnginePath -Raw
    if ($androidEngine -notmatch 'fun start\(configuration: TunnelConfiguration, service: VpnService\)' -or
        $androidEngine -notmatch 'override fun start\(configuration: TunnelConfiguration, service: VpnService\)') {
        $failures.Add('Android WireGuard engine implementations must match the VpnService adapter signature.')
    }
}
$androidManifestPath = Join-Path $repoRoot 'clients/android/app/src/main/AndroidManifest.xml'
$androidManifest = Get-Content -LiteralPath $androidManifestPath -Raw
if ($androidManifest -notmatch 'android:usesCleartextTraffic="false"') {
    $failures.Add('Android manifest must disable cleartext traffic.')
}
if ($androidManifest -notmatch 'android.software.leanback' -or $androidManifest -notmatch 'android.hardware.touchscreen" android:required="false"') {
    $failures.Add('Android manifest must declare optional Leanback and touchscreen features for TV support.')
}
$androidAPIPath = Join-Path $repoRoot 'clients/android/app/src/main/java/com/personalvpn/client/VpnApiClient.kt'
$androidAPI = Get-Content -LiteralPath $androidAPIPath -Raw
if ($androidAPI -notmatch 'instanceFollowRedirects\s*=\s*false') {
    $failures.Add('Android API client must reject redirects to protect bearer tokens.')
}
$androidActivityPath = Join-Path $repoRoot 'clients/android/app/src/main/java/com/personalvpn/client/MainActivity.kt'
$androidActivity = Get-Content -LiteralPath $androidActivityPath -Raw
if ($androidActivity -notmatch 'POST_NOTIFICATIONS' -or $androidActivity -notmatch 'requestPermissions') {
    $failures.Add('Android activity must request notification permission on supported API levels.')
}
$androidBuildPath = Join-Path $repoRoot 'clients/android/app/build.gradle.kts'
$androidBuild = Get-Content -LiteralPath $androidBuildPath -Raw
if ($androidBuild -notmatch 'coreLibraryDesugaring' -or $androidBuild -notmatch 'VERSION_17') {
    $failures.Add('Android build must configure Java 17 core-library desugaring.')
}
$unitPath = Join-Path $repoRoot 'deploy/systemd/personalvpn-gateway-agent.service'
if (-not (Test-Path -LiteralPath $unitPath)) {
    $failures.Add('Gateway-agent systemd unit is missing.')
} else {
    $unit = Get-Content -LiteralPath $unitPath -Raw
    foreach ($requiredUnitSetting in @('User=personalvpn-agent', 'NoNewPrivileges=true', 'CapabilityBoundingSet=CAP_NET_ADMIN', 'ProtectSystem=strict', 'Environment=PATH=/usr/bin:/bin')) {
        if ($unit -notmatch [regex]::Escape($requiredUnitSetting)) {
            $failures.Add("Gateway-agent systemd unit is missing: $requiredUnitSetting")
        }
    }
}

$migrationPath = Join-Path $repoRoot 'services/api/migrations/0001_initial.sql'
$migration = Get-Content -LiteralPath $migrationPath -Raw
if ($migration -match '(?im)\b(private_key|privatekey)\b') {
    $failures.Add('Database migration must not contain a private-key column.')
}
foreach ($requiredTable in @('users', 'devices', 'regions', 'gateways', 'ip_leases', 'peer_assignments', 'audit_events', 'gateway_snapshots')) {
    if ($migration -notmatch "(?im)^CREATE TABLE $requiredTable \(") {
        $failures.Add("Database migration is missing table: $requiredTable")
    }
}

$goModulePath = Join-Path $repoRoot 'services/api/go.mod'
$goModule = Get-Content -LiteralPath $goModulePath -Raw
if ($goModule -notmatch 'github\.com/jackc/pgx/v5\s+v5\.10\.0') {
    $failures.Add('Go module must pin the reviewed pgx v5.10.0 release.')
}
if (-not (Test-Path -LiteralPath (Join-Path $repoRoot 'services/api/go.sum'))) {
    $failures.Add('Go dependency checksums are missing.')
}

$dockerfilePath = Join-Path $repoRoot 'services/api/Dockerfile'
$dockerfile = Get-Content -LiteralPath $dockerfilePath -Raw
if ($dockerfile -notmatch '(?im)^FROM\s+golang:1\.26-alpine@sha256:[0-9a-f]{64}\s+AS\s+build\s*$') {
    $failures.Add('Go builder image must be pinned by digest.')
}
$composePath = Join-Path $repoRoot 'deploy/docker-compose.staging.yml'
$compose = Get-Content -LiteralPath $composePath -Raw
if ($compose -notmatch '(?im)^\s*image:\s*postgres:17-alpine@sha256:[0-9a-f]{64}\s*$') {
    $failures.Add('PostgreSQL staging image must be pinned by digest.')
}

if ($failures.Count -gt 0) {
    $failures | ForEach-Object { Write-Error $_ }
    exit 1
}

Write-Output 'Repository static checks passed.'
