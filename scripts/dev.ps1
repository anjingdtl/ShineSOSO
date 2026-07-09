# Start the Go backend and Vite dev server side-by-side on Windows.
# Usage: .\scripts\dev.ps1

$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$dataDir = if ($env:EASYSEARCH_DATA_DIR) { $env:EASYSEARCH_DATA_DIR } else { Join-Path $root 'backend/.devdata' }
New-Item -ItemType Directory -Force -Path (Join-Path $dataDir 'logs') | Out-Null

$backend = Start-Process -FilePath 'go' `
    -ArgumentList 'run', './backend/cmd/easysearch', '--no-browser' `
    -WorkingDirectory $root `
    -RedirectStandardOutput (Join-Path $dataDir 'logs/backend.out') `
    -RedirectStandardError (Join-Path $dataDir 'logs/backend.err') `
    -PassThru -NoNewWindow

Write-Host "[dev] starting backend (pid=$($backend.Id)) ..."

$portFile = Join-Path $dataDir '.port'
for ($i = 0; $i -lt 30; $i++) {
    if (Test-Path $portFile) { break }
    Start-Sleep -Milliseconds 200
}

if (-not (Test-Path $portFile)) {
    Write-Host '[dev] backend did not write .port in time' -ForegroundColor Red
    Get-Content (Join-Path $dataDir 'logs/backend.err')
    $backend | Stop-Process -Force
    exit 1
}

$port = Get-Content $portFile -Raw
Write-Host "[dev] backend listening on port $port"

Push-Location (Join-Path $root 'frontend')
try {
    npm run dev
} finally {
    Pop-Location
    if (-not $backend.HasExited) {
        Write-Host '[dev] stopping backend'
        $backend | Stop-Process
    }
}
