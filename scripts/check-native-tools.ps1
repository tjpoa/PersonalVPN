$ErrorActionPreference = 'SilentlyContinue'

# Android Studio installs adb under the SDK even when it is not on PATH.
$androidPlatformTools = Join-Path $env:LOCALAPPDATA 'Android\Sdk\platform-tools'
if (Test-Path -LiteralPath (Join-Path $androidPlatformTools 'adb.exe')) {
    $env:Path = "$androidPlatformTools$([IO.Path]::PathSeparator)$env:Path"
}
$localDotnet = Join-Path $env:LOCALAPPDATA 'PersonalVPN\dotnet8'
if (Test-Path -LiteralPath (Join-Path $localDotnet 'dotnet.exe')) {
    $env:Path = "$localDotnet$([IO.Path]::PathSeparator)$env:Path"
}

function Test-Tool([string]$Name, [string]$Hint) {
    $command = Get-Command $Name
    if ($null -ne $command) {
        $version = ''
        switch ($Name) {
            'docker' { $version = (& $Name --version 2>$null | Select-Object -First 1) }
            'dotnet' { $version = (& $Name --version 2>$null | Select-Object -First 1) }
            'adb' { $version = (& $Name version 2>$null | Select-Object -First 1) }
            'gradle' { $version = (& $Name --version 2>$null | Select-String '^Gradle ' | Select-Object -First 1) }
            'xcodebuild' { $version = (& $Name -version 2>$null | Select-Object -First 1) }
        }
        if ([string]::IsNullOrWhiteSpace($version)) { $version = 'version unavailable' }
        Write-Host ("[ok]   {0} -> {1} ({2})" -f $Name, $command.Source, $version.ToString().Trim())
        return $true
    }
    Write-Host ("[miss] {0} ({1})" -f $Name, $Hint)
    return $false
}

$found = 0
if (Test-Tool 'docker' 'Docker Desktop for API/lab tests') { $found++ }
if (Test-Tool 'dotnet' '.NET 8 SDK for clients/windows') { $found++ }
if (Test-Tool 'adb' 'Android SDK Platform Tools for Android/TV') { $found++ }
$gradleWrapper = Join-Path (Split-Path -Parent $PSScriptRoot) 'clients/android/gradlew.bat'
if (Get-Command gradle) {
    if (Test-Tool 'gradle' 'Android Gradle tooling (CI installs Gradle 8.9)') { $found++ }
} elseif (Test-Path -LiteralPath $gradleWrapper) {
    Write-Host ("[ok]   gradle -> {0} (project wrapper)" -f $gradleWrapper)
    $found++
} else {
    Write-Host '[miss] gradle (Android project wrapper not found)'
}
if ($IsMacOS -and (Test-Tool 'xcodebuild' 'Xcode for clients/ios')) { $found++ }
elseif (-not $IsMacOS) { Write-Output '[miss] xcodebuild (requires a macOS runner for clients/ios)' }

Write-Output ("Detected {0} native toolchain command(s). This check is read-only." -f $found)
