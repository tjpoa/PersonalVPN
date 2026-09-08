[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$checks = @(
    @{ Name = 'Docker'; Test = { (docker info --format '{{.OSType}}' 2>$null) -eq 'linux' } },
    @{ Name = 'Lab harness'; Test = { Test-Path (Join-Path $repoRoot 'tests/e2e/lab/run.sh') } },
    @{ Name = 'Lab wrapper'; Test = { Test-Path (Join-Path $repoRoot 'scripts/run-lab.ps1') } },
    @{ Name = 'WireGuard lab docs'; Test = { Test-Path (Join-Path $repoRoot 'docs/lab.md') } }
)

$failed = $false
foreach ($check in $checks) {
    if (& $check.Test) { Write-Output "[ok]   $($check.Name)" }
    else { Write-Output "[miss] $($check.Name)"; $failed = $true }
}
if ($failed) { throw 'WireGuard lab prerequisites are incomplete' }
Write-Output 'Lab preflight passed (read-only; no namespaces or firewall changes).'
