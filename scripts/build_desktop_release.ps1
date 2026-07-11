[CmdletBinding()]
param(
    [string]$WailsPath = ""
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$desktopRoot = Join-Path $repoRoot "desktop"
$binRoot = Join-Path $desktopRoot "build\bin"

if ([string]::IsNullOrWhiteSpace($WailsPath)) {
    $wails = Get-Command wails.exe -ErrorAction SilentlyContinue
    if ($null -eq $wails) {
        throw "wails.exe is required on PATH"
    }
    $WailsPath = $wails.Source
}

New-Item -ItemType Directory -Force -Path $binRoot | Out-Null
Push-Location $repoRoot
try {
    go build -o (Join-Path $binRoot "genesisd.exe") ./cmd/genesisd
} finally {
    Pop-Location
}

Push-Location $desktopRoot
try {
    & $WailsPath build -nsis
    if ($LASTEXITCODE -ne 0) {
        throw "Wails NSIS build failed with exit code $LASTEXITCODE"
    }
} finally {
    Pop-Location
}
