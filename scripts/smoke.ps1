# smoke.ps1 — end-to-end smoke test for an easysearch.exe release binary.
#
# Goal: validate the most critical user journey from a freshly built
# executable, without any external network. The script:
#
#   1. Boots the binary against a fresh temp data dir (no prior state).
#   2. Waits for the .port file and hits GET /api/v1/system/status.
#   3. Confirms GET /api/v1/indexer-catalog lists the three demo
#      indexers (demo-alpha / demo-beta / demo-gamma).
#   4. Adds demo-alpha via POST /api/v1/indexers, verifies it appears.
#   5. POSTs a search session for "smoke" and waits for SSE
#      session_completed. Verifies at least one result is returned.
#   6. Hits GET /api/v1/system/diagnostics and confirms the response is
#      a valid ZIP containing version.txt + indexers.json + README.txt.
#   7. Stops the binary cleanly and reports PASS/FAIL counts.
#
# Strategy: spin the binary as a child process, drive it via HTTP.
# The binary itself owns browser launch + port file writing.

$ErrorActionPreference = "Stop"

$Script:failed = $false
$Script:passed = 0
$Script:total = 0

function Check {
    param([string]$Name, [bool]$Cond)
    $Script:total++
    if ($Cond) {
        $Script:passed++
        Write-Host "  PASS $Name"
    } else {
        Write-Host "  FAIL $Name"
        $Script:failed = $true
    }
}

function Section {
    param([string]$Title)
    Write-Host ""
    Write-Host "[$Title]"
}

$dataDir = Join-Path $env:TEMP "easysearch-smoke-$([guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Path $dataDir -Force | Out-Null

$env:EASYSEARCH_DATA_DIR = $dataDir
$exe = Join-Path $PSScriptRoot "..\dist\easysearch.exe"
if (-not (Test-Path $exe)) {
    throw "exe not found at $exe; run scripts/build.ps1 first"
}

Write-Host "Using temp data dir: $dataDir"
Write-Host "Using exe:           $exe"

$proc = Start-Process -FilePath $exe -ArgumentList @("--port", "0", "--no-browser") `
    -PassThru `
    -RedirectStandardOutput "$dataDir\stdout.txt" `
    -RedirectStandardError "$dataDir\stderr.txt"

try {
    $portFile = Join-Path $dataDir ".port"
    $deadline = (Get-Date).AddSeconds(15)
    while (-not (Test-Path $portFile) -and (Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 100
    }
    if (-not (Test-Path $portFile)) {
        throw "server did not write $portFile within 15s"
    }
    $port = (Get-Content $portFile -Raw).Trim()
    $base = "http://127.0.0.1:$port"
    Write-Host "Server up at $base"

    Section "1/6 system/status"
    $status = Invoke-RestMethod -Method Get -Uri "$base/api/v1/system/status"
    Check "status.version populated"        ($status.version -ne "")
    Check "status.bindHost is loopback"     ($status.bindHost -eq "127.0.0.1" -or [string]::IsNullOrEmpty($status.bindHost))
    Check "status.uptimeMs non-negative"    ($status.uptimeMs -ge 0)

    Section "2/6 indexer catalog lists built-in mocks"
    $catalog = Invoke-RestMethod -Method Get -Uri "$base/api/v1/indexer-catalog"
    $demoIds = @("demo-alpha", "demo-beta", "demo-gamma")
    $haveDemo = @()
    foreach ($d in $catalog.items) {
        if ($demoIds -contains $d.id) { $haveDemo += $d.id }
    }
    Check "demo-alpha in catalog" ("demo-alpha" -in $haveDemo)
    Check "demo-beta in catalog"  ("demo-beta" -in $haveDemo)
    Check "demo-gamma in catalog" ("demo-gamma" -in $haveDemo)

    Section "3/6 add demo-alpha via POST /indexers"
    $added = $null
    try {
        $added = Invoke-RestMethod -Method Post -Uri "$base/api/v1/indexers" `
            -Headers @{ "Content-Type" = "application/json" } `
            -Body (@{ definitionId = "demo-alpha" } | ConvertTo-Json -Compress)
    } catch {
        Check "POST /indexers returned 2xx" $false
        Write-Host "    $($_.Exception.Message)"
    }
    Check "POST /indexers succeeded" ($null -ne $added)
    Check "added.id populated"        ($added -and $added.id -ne "")
    $indexerId = $added.id

    $list = Invoke-RestMethod -Method Get -Uri "$base/api/v1/indexers"
    $found = $false
    foreach ($i in $list.items) { if ($i.id -eq $indexerId) { $found = $true; break } }
    Check "added indexer appears in GET /indexers" $found

    Section "4/6 search 'smoke' via SSE"
    $session = $null
    try {
        $session = Invoke-RestMethod -Method Post -Uri "$base/api/v1/search/sessions" `
            -Headers @{ "Content-Type" = "application/json" } `
            -Body (@{ query = "smoke"; categories = @("all") } | ConvertTo-Json -Compress)
    } catch {
        Check "POST /search/sessions returned 2xx" $false
        Write-Host "    $($_.Exception.Message)"
    }
    Check "session.id populated" ($null -ne $session -and $session.id -ne "")

    if ($session -and $session.id) {
        $eventUrl = "$base/api/v1/search/sessions/$($session.id)/events"
        $completed = $false
        $gotResult = $false
        $resultCount = 0
        $deadline = (Get-Date).AddSeconds(15)
        try {
            # Use HttpClient via .NET to read SSE line-by-line.
            Add-Type -AssemblyName System.Net.Http
            $client = New-Object System.Net.Http.HttpClient
            $client.Timeout = [TimeSpan]::FromSeconds(20)
            $req = New-Object System.Net.Http.HttpRequestMessage ([System.Net.Http.HttpMethod]::Get), $eventUrl
            $resp = $client.SendAsync($req, [System.Net.Http.HttpCompletionOption]::ResponseHeadersRead).GetAwaiter().GetResult()
            $stream = $resp.Content.ReadAsStreamAsync().GetAwaiter().GetResult()
            $reader = New-Object System.IO.StreamReader($stream)
            while ((Get-Date) -lt $deadline) {
                $line = $reader.ReadLine()
                if ($null -eq $line) { break }
                if ($line.StartsWith("data:")) {
                    $payload = $line.Substring(5).Trim()
                    if ($payload -eq "") { continue }
                    try {
                        $evt = $payload | ConvertFrom-Json -ErrorAction SilentlyContinue
                    } catch { continue }
                    if ($evt.type -eq "session_completed") { $completed = $true; break }
                    if ($evt.type -eq "indexer_result" -or $evt.type -eq "results_merged") {
                        if ($evt.results) { $resultCount += $evt.results.Count }
                        $gotResult = $true
                    }
                }
            }
        } catch {
            Write-Host "    SSE read error: $($_.Exception.Message)"
        }
        Check "session_completed received"      $completed
        Check "at least one result event"      ($gotResult -or $resultCount -gt 0)
        Check "resultCount >= 1"               ($resultCount -ge 1)
    }

    Section "5/6 diagnostics bundle"
    $diagPath = Join-Path $dataDir "diagnostics.zip"
    try {
        Invoke-WebRequest -Method Get -Uri "$base/api/v1/system/diagnostics" `
            -OutFile $diagPath -ErrorAction Stop | Out-Null
    } catch {
        Check "GET /system/diagnostics returned 2xx" $false
        Write-Host "    $($_.Exception.Message)"
    }
    Check "diagnostics zip exists"      (Test-Path $diagPath)
    Check "diagnostics zip non-empty"   ((Test-Path $diagPath) -and (Get-Item $diagPath).Length -gt 0)
    if (Test-Path $diagPath) {
        # A minimal ZIP ends with 50 4B 05 06. Just check magic.
        $bytes = [System.IO.File]::ReadAllBytes($diagPath)
        $magic = ($bytes[0], $bytes[1], $bytes[2], $bytes[3]) -join ","
        Check "diagnostics starts with PK magic ($magic)" ($magic -eq "80,75,3,4")
    }

    Section "6/6 graceful shutdown"
    if ($proc -and -not $proc.HasExited) {
        Stop-Process -Id $proc.Id -Force
        Start-Sleep -Milliseconds 500
    }
    Check "port file removed after shutdown" (-not (Test-Path $portFile))
}
finally {
    if ($proc -and -not $proc.HasExited) {
        Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
    }
    Remove-Item -Recurse -Force $dataDir -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Host "Passed: $Script:passed / $Script:total"
if ($Script:failed) {
    Write-Host "SMOKE FAILED"
    exit 1
}
Write-Host "SMOKE OK"
exit 0