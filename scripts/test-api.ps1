[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$apiRoot = Join-Path $repoRoot 'services/api'
$imageName = 'personalvpn-api:test'

docker build --tag $imageName $apiRoot
if ($LASTEXITCODE -ne 0) {
    throw "API test/build failed with exit code $LASTEXITCODE"
}

Write-Output "API tests and container build passed: $imageName"
