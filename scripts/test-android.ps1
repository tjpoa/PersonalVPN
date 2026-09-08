[CmdletBinding()]
param(
    [string]$Device = ""
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$androidRoot = Join-Path $repoRoot 'clients/android'
$adb = Join-Path $env:LOCALAPPDATA 'Android/Sdk/platform-tools/adb.exe'
if (-not (Test-Path $adb)) { throw "adb not found at $adb" }

$devices = @(& $adb devices | Select-String '\sdevice$')
if ($devices.Count -eq 0) { throw "No Android device/emulator is connected" }
if ([string]::IsNullOrWhiteSpace($Device)) {
    $Device = (($devices[0].ToString() -split '\s+')[0])
}

Push-Location $androidRoot
try {
    & .\gradlew.bat testDebugUnitTest --console=plain
    if ($LASTEXITCODE -ne 0) { throw "Android unit tests failed" }
    & .\gradlew.bat assembleDebug assembleDebugAndroidTest --console=plain
    if ($LASTEXITCODE -ne 0) { throw "Android APK build failed" }
} finally { Pop-Location }

$appApk = Join-Path $androidRoot 'app/build/outputs/apk/debug/app-debug.apk'
$testApk = Join-Path $androidRoot 'app/build/outputs/apk/androidTest/debug/app-debug-androidTest.apk'
& $adb -s $Device install -r $appApk
& $adb -s $Device install -r $testApk
if ($LASTEXITCODE -ne 0) { throw "Could not install Android test APKs" }
& $adb -s $Device shell am instrument -w -r 'com.personalvpn.client.test/androidx.test.runner.AndroidJUnitRunner'
if ($LASTEXITCODE -ne 0) { throw "Android instrumentation failed" }
Write-Output "Android unit and instrumentation tests passed on $Device."
