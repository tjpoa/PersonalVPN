[CmdletBinding()]
param(
    [string]$Device = 'emulator-5554'
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$output = Join-Path $repoRoot 'deploy/secrets/caddy-local-root.crt'
$adb = (Get-Command adb -ErrorAction SilentlyContinue).Source
if (-not $adb) {
    $candidate = Join-Path $env:LOCALAPPDATA 'Android\Sdk\platform-tools\adb.exe'
    if (Test-Path -LiteralPath $candidate) { $adb = $candidate }
}
if (-not $adb) { throw 'adb not found. Install Android SDK Platform-Tools or add it to PATH.' }
docker cp deploy-local-proxy-1:/data/caddy/pki/authorities/local/root.crt $output
if ($LASTEXITCODE -ne 0) { throw 'Could not extract the Caddy local CA.' }

& $adb -s $Device push $output /sdcard/Download/PersonalVPN-Local-CA.crt | Out-Host
if ($LASTEXITCODE -ne 0) { throw "Could not copy the CA to $Device." }
& $adb -s $Device shell am start -a android.intent.action.VIEW -t application/x-x509-ca-cert -d file:///sdcard/Download/PersonalVPN-Local-CA.crt | Out-Host
if ($LASTEXITCODE -ne 0) { throw 'Could not open the certificate installer on the emulator.' }
Write-Output 'Certificate installer opened on the emulator. Confirm the CA installation there.'
