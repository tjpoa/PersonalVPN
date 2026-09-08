[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    Write-Output 'WireGuard Windows preflight skipped: this check requires Windows.'
    exit 0
}

$candidates = @(
    (Join-Path ${env:ProgramFiles} 'WireGuard\wireguard.exe'),
    (Join-Path ${env:ProgramFiles} 'WireGuard\wireguard.dll'),
    (Join-Path ${env:ProgramFiles} 'WireGuard\wireguard.exe.manifest')
)
$found = $candidates | Where-Object { Test-Path -LiteralPath $_ }
if ($found.Count -eq 0) {
    Write-Output 'WireGuard Windows preflight: official installation not found.'
    Write-Output 'Install the signed WireGuard distribution before enabling the Windows backend.'
    exit 1
}

$found | ForEach-Object { Write-Output ("[ok] {0}" -f $_) }
Write-Output 'WireGuard Windows preflight passed (read-only).'
