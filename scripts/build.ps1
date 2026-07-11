# Build EasySearch.exe on Windows.
# Steps:
#   1. Install npm deps (cached on subsequent runs)
#   2. Build the React frontend into the Go embed dir
#   3. Compile the Go binary with -H windowsgui so the process is a
#      windowless subsystem; the launcher still prints to stdout when
#      invoked from a console.
# Output: dist/easysearch.exe

param(
    [string]$OutputPath = ''
)

$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$frontend = Join-Path $root 'frontend'
$backend = Join-Path $root 'backend'
$distDir = Join-Path $root 'dist'
$exePath = if ($OutputPath) { [IO.Path]::GetFullPath($OutputPath) } else { Join-Path $distDir 'easysearch.exe' }
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $exePath) | Out-Null

New-Item -ItemType Directory -Force -Path $distDir | Out-Null

# 1. Frontend deps + build
Write-Host '[build] installing npm dependencies ...' -ForegroundColor Cyan
Push-Location $frontend
try {
    if (Test-Path (Join-Path $frontend 'package-lock.json')) {
        npm ci --no-audit --no-fund --registry=https://registry.npmmirror.com
    } else {
        npm install --no-audit --no-fund --registry=https://registry.npmmirror.com
    }
} finally {
    Pop-Location
}

Write-Host '[build] building React frontend ...' -ForegroundColor Cyan
Push-Location $frontend
try {
    npm run build
} finally {
    Pop-Location
}

# 2. Go build
$ldflags = '-s -w -H windowsgui'
$version = '0.1.0'
$ldflagsFull = "$ldflags -X main.version=$version"

Write-Host '[build] compiling Go binary ...' -ForegroundColor Cyan
$env:CGO_ENABLED = '0'
go build -trimpath -ldflags $ldflagsFull -o $exePath ./backend/cmd/easysearch
if ($LASTEXITCODE -ne 0) { throw 'go build failed' }

$size = (Get-Item $exePath).Length
Write-Host "[build] done: $exePath ($([math]::Round($size / 1MB, 2)) MB)" -ForegroundColor Green
Write-Host ''
Write-Host 'To run:'
Write-Host "  & '$exePath'"
Write-Host ''
Write-Host 'To run a smoke test on a fixed port:'
Write-Host "  & '$exePath' --port 18765 --no-browser"
Write-Host '  curl http://127.0.0.1:18765/api/v1/system/status'
