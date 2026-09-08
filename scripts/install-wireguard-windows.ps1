#Requires -RunAsAdministrator
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
winget install --id WireGuard.WireGuard --exact --source winget `
    --accept-package-agreements --accept-source-agreements
if ($LASTEXITCODE -ne 0) { throw "WireGuard installation failed with exit code $LASTEXITCODE" }

& (Join-Path $PSScriptRoot 'check-wireguard-windows.ps1')
if ($LASTEXITCODE -ne 0) { throw 'WireGuard installation could not be verified.' }
