[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$imageName = 'personalvpn-wireguard-lab:local'
$labPath = Join-Path $repoRoot 'tests/e2e/lab'
$resultPath = Join-Path $repoRoot 'tests/e2e/results'

New-Item -ItemType Directory -Force -Path $resultPath | Out-Null

docker build --tag $imageName $labPath
if ($LASTEXITCODE -ne 0) {
    throw "Docker build failed with exit code $LASTEXITCODE"
}

docker run --rm --privileged `
    --name personalvpn-wireguard-lab `
    --volume "${resultPath}:/results" `
    $imageName
if ($LASTEXITCODE -ne 0) {
    throw "WireGuard lab failed with exit code $LASTEXITCODE"
}

Get-Content -LiteralPath (Join-Path $resultPath 'wireguard-lab.txt')
