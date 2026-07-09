# Phase 4 smoke test: end-to-end verification that the indexer
# management flow actually works against a fresh database.
#
# 1. Boot the server with EASYSEARCH_DATA_DIR pointing at a temp dir.
# 2. Read the port from .port file.
# 3. GET /api/v1/indexer-catalog should list 3 builtins.
# 4. POST /api/v1/indexers with demo-alpha; expect 201.
# 5. GET /api/v1/indexers should list 1.
# 6. POST /api/v1/indexers/{id}/test should return ok=true.
# 7. PATCH enabled=false should toggle.
# 8. DELETE should remove it.
#
# Exits non-zero on any failure.

$ErrorActionPreference = "Stop"

$dataDir = Join-Path $env:TEMP "easysearch-phase4-smoke-$([guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Path $dataDir -Force | Out-Null

$env:EASYSEARCH_DATA_DIR = $dataDir
$exe = Join-Path $PSScriptRoot "..\dist\easysearch.exe"
if (-not (Test-Path $exe)) {
    throw "exe not found at $exe; build first"
}

$proc = Start-Process -FilePath $exe -ArgumentList @("--port", "0", "--no-browser") -PassThru -RedirectStandardOutput "$dataDir\stdout.txt" -RedirectStandardError "$dataDir\stderr.txt"
try {
    # Wait for the .port file to appear.
    $portFile = Join-Path $dataDir ".port"
    $deadline = (Get-Date).AddSeconds(15)
    while (-not (Test-Path $portFile) -and (Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 100
    }
    if (-not (Test-Path $portFile)) {
        throw "server did not write $portFile within 15s"
    }
    $port = Get-Content $portFile -Raw
    $port = $port.Trim()
    $base = "http://127.0.0.1:$port"

    function Check($name, $cond) {
        if ($cond) {
            Write-Host "  PASS $name"
        } else {
            Write-Host "  FAIL $name"
            $script:failed = $true
        }
    }
    $failed = $false

    Write-Host "[1/6] catalog"
    $cat = Invoke-RestMethod -Method Get -Uri "$base/api/v1/indexer-catalog" -Headers @{ Accept = "application/json" }
    Check "catalog has 3 items" ($cat.items.Count -eq 3)

    Write-Host "[2/6] create"
    $created = Invoke-RestMethod -Method Post -Uri "$base/api/v1/indexers" -Headers @{ "Content-Type" = "application/json" } -Body (@{ definitionId = "demo-alpha"; baseUrl = "https://example.com" } | ConvertTo-Json)
    Check "create returned 201-ish id" ($created.id.Length -gt 0)

    Write-Host "[3/6] list"
    $list = Invoke-RestMethod -Method Get -Uri "$base/api/v1/indexers" -Headers @{ Accept = "application/json" }
    Check "list shows 1" ($list.items.Count -eq 1)

    Write-Host "[4/6] test"
    $test = Invoke-RestMethod -Method Post -Uri "$base/api/v1/indexers/$($created.id)/test"
    Check "test ok=true" ($test.ok -eq $true)

    Write-Host "[5/6] toggle"
    $patched = Invoke-RestMethod -Method Patch -Uri "$base/api/v1/indexers/$($created.id)" -Headers @{ "Content-Type" = "application/json" } -Body (@{ enabled = $false } | ConvertTo-Json)
    Check "toggle disabled" ($patched.enabled -eq $false)

    Write-Host "[6/6] delete + unsafe URL"
    Invoke-RestMethod -Method Delete -Uri "$base/api/v1/indexers/$($created.id)" | Out-Null
    $list2 = Invoke-RestMethod -Method Get -Uri "$base/api/v1/indexers"
    Check "list empty after delete" ($list2.items.Count -eq 0)

    # SSRF block
    try {
        Invoke-RestMethod -Method Post -Uri "$base/api/v1/indexers" -Headers @{ "Content-Type" = "application/json" } -Body (@{ definitionId = "demo-alpha"; baseUrl = "http://127.0.0.1/x" } | ConvertTo-Json)
        Check "unsafe URL rejected" $false
    } catch {
        Check "unsafe URL rejected" ($_.Exception.Response.StatusCode.value__ -eq 400)
    }

    if ($failed) {
        Write-Host "PHASE 4 SMOKE FAILED"
        exit 1
    }
    Write-Host "PHASE 4 SMOKE OK"
    exit 0
} finally {
    if ($proc -and -not $proc.HasExited) {
        Stop-Process -Id $proc.Id -Force
    }
    Remove-Item -Recurse -Force $dataDir -ErrorAction SilentlyContinue
}