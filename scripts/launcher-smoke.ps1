# launcher-smoke.ps1 - verify the user-facing double-click launcher on a fresh profile.

$ErrorActionPreference = 'Stop'

$root = Split-Path -Parent $PSScriptRoot
$exe = Join-Path $root 'dist\easysearch.exe'
$launcher = Join-Path $root '启动 EasySearch.vbs'
if (-not (Test-Path $exe)) { throw "exe not found at $exe; run scripts\build.ps1 first" }
if (-not (Test-Path $launcher)) { throw "launcher not found at $launcher" }

$originalAppData = $env:APPDATA
$testAppData = Join-Path $env:TEMP "easysearch-launcher-$([guid]::NewGuid().ToString('N'))"
$env:APPDATA = $testAppData
$dataDir = Join-Path $testAppData 'EasySearch\data'
$portFile = Join-Path $dataDir '.port'
$serverPid = $null

try {
    # cscript is used only to invoke the same root VBS that Explorer runs.
    & cscript.exe //nologo //B $launcher
    if ($LASTEXITCODE -ne 0) { throw "launcher returned exit code $LASTEXITCODE" }

    $deadline = (Get-Date).AddSeconds(15)
    while (-not (Test-Path $portFile) -and (Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 100
    }
    if (-not (Test-Path $portFile)) { throw "launcher did not create $portFile within 15 seconds" }

    $port = [int](Get-Content $portFile -Raw).Trim()
    $status = Invoke-RestMethod -Method Get -Uri "http://127.0.0.1:$port/api/v1/system/status"
    if ([string]::IsNullOrWhiteSpace($status.version)) { throw 'system status did not contain a version' }

    $connection = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction Stop | Select-Object -First 1
    $serverPid = $connection.OwningProcess
    Write-Host "LAUNCHER OK: EasySearch $($status.version) is reachable on port $port (pid=$serverPid)" -ForegroundColor Green
}
finally {
    if ($serverPid) {
        Stop-Process -Id $serverPid -Force -ErrorAction SilentlyContinue
    }
    $env:APPDATA = $originalAppData
    Remove-Item -LiteralPath $testAppData -Recurse -Force -ErrorAction SilentlyContinue
}
