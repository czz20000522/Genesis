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

if ($null -eq (Get-Command makensis.exe -ErrorAction SilentlyContinue)) {
    foreach ($candidate in @('C:\Program Files (x86)\NSIS\makensis.exe', 'C:\Program Files\NSIS\makensis.exe')) {
        if (Test-Path -LiteralPath $candidate) {
            $env:Path = "$(Split-Path -Parent $candidate);$env:Path"
            break
        }
    }
}

New-Item -ItemType Directory -Force -Path $binRoot | Out-Null
$adapterTarget = Join-Path $binRoot "scripts\providers"
New-Item -ItemType Directory -Force -Path $adapterTarget | Out-Null
Copy-Item -LiteralPath (Join-Path $repoRoot "scripts\providers\llama_cpp_provider_command.py") -Destination (Join-Path $adapterTarget "llama_cpp_provider_command.py") -Force
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

    $installerPath = Join-Path $binRoot "genesis-desktop-amd64-installer.exe"
    $checksumPath = Join-Path $binRoot "genesis-desktop-amd64-installer.exe.sha256"
    $checksum = (Get-FileHash -Path $installerPath -Algorithm SHA256).Hash.ToLowerInvariant()
    Set-Content -Path $checksumPath -Value "$checksum *genesis-desktop-amd64-installer.exe" -NoNewline -Encoding ascii
} finally {
    Pop-Location
}
