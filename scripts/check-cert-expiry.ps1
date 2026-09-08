param(
    [Parameter(Mandatory = $true)][string[]]$Path,
    [int]$WarnWithinDays = 30
)

$ErrorActionPreference = 'Stop'
if ($WarnWithinDays -lt 1 -or $WarnWithinDays -gt 365) { throw 'WarnWithinDays must be between 1 and 365' }
$now = [DateTime]::UtcNow
$warning = $now.AddDays($WarnWithinDays)
$failed = $false

foreach ($certificatePath in $Path) {
    if (-not (Test-Path -LiteralPath $certificatePath -PathType Leaf)) {
        Write-Output "[missing] $certificatePath"
        $failed = $true
        continue
    }
    $certificate = [System.Security.Cryptography.X509Certificates.X509Certificate2]::new((Resolve-Path -LiteralPath $certificatePath).Path)
    try {
        $remaining = $certificate.NotAfter.ToUniversalTime() - $now
        if ($certificate.NotAfter.ToUniversalTime() -le $now) {
            Write-Output "[expired] $certificatePath"
            $failed = $true
        } elseif ($certificate.NotAfter.ToUniversalTime() -le $warning) {
            Write-Output ("[warning] {0} expires in {1:N0} days" -f $certificatePath, $remaining.TotalDays)
        } else {
            Write-Output ("[ok] {0} expires in {1:N0} days" -f $certificatePath, $remaining.TotalDays)
        }
    } finally { $certificate.Dispose() }
}

if ($failed) { exit 1 }
