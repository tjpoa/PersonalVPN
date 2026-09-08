[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$composeFile = Join-Path (Split-Path -Parent $PSScriptRoot) 'deploy/docker-compose.staging.yml'
if (-not (Test-Path -LiteralPath $composeFile)) { throw "Compose file not found: $composeFile" }

docker compose -f $composeFile run --rm -T migrate sh -c 'read -r PGPASSWORD < /run/secrets/postgres_password; export PGPASSWORD; psql -v ON_ERROR_STOP=1 -f /seed/seed-local.sql'
if ($LASTEXITCODE -ne 0) { throw 'Local seed failed.' }

$checkSql = "SELECT CASE WHEN EXISTS (SELECT 1 FROM regions WHERE id = 'pt-lis' AND available) AND EXISTS (SELECT 1 FROM gateways WHERE name = 'local-gateway' AND state = 'ready') THEN 1 ELSE 0 END;"
$checkResult = $checkSql | docker compose -f $composeFile run --rm -T migrate sh -c 'read -r PGPASSWORD < /run/secrets/postgres_password; export PGPASSWORD; psql -Atq'
if ($LASTEXITCODE -ne 0 -or $checkResult.Trim() -ne '1') { throw 'Local seed verification failed.' }
Write-Output 'Local region and gateway seed applied.'
