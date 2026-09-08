[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $repoRoot 'deploy/docker-compose.staging.yml'
docker compose -f $composeFile --profile local-proxy up -d local-proxy
if ($LASTEXITCODE -ne 0) { throw 'Local HTTPS proxy failed to start.' }
Write-Output 'Local HTTPS proxy listening on https://10.0.2.2:8443'
Write-Output 'The Caddy development CA must be trusted by the Android emulator before API calls will succeed.'
