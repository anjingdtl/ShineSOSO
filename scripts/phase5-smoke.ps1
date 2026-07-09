# Phase 5 smoke test: end-to-end verification of the YAML engine.
#
# 1. Boot the server against a fresh temp data dir.
# 2. POST /api/v1/indexer-catalog/import with a valid HTML YAML
#    (using a TEST-NET-2 IPv4 so the validator doesn't reject it as
#    private). Expect valid=true, persisted=true, definition registered.
# 3. POST same endpoint with a private-IP YAML → expect valid=false,
#    errors[0].code == "LINK_UNSAFE".
# 4. POST with oversize YAML → expect HTTP 400 DEFINITION_TOO_LARGE.
# 5. POST with broken YAML → expect HTTP 400 INVALID_YAML.
# 6. GET /api/v1/indexer-catalog/imported → should list 1 entry.
# 7. Follow-up: install the imported indexer via POST /api/v1/indexers
#    and verify GET /api/v1/indexers shows it.

$ErrorActionPreference = "Stop"

$dataDir = Join-Path $env:TEMP "easysearch-phase5-smoke-$([guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Path $dataDir -Force | Out-Null

$env:EASYSEARCH_DATA_DIR = $dataDir
$exe = Join-Path $PSScriptRoot "..\dist\easysearch.exe"
if (-not (Test-Path $exe)) {
    throw "exe not found at $exe; build first"
}

$proc = Start-Process -FilePath $exe -ArgumentList @("--port", "0", "--no-browser") -PassThru -RedirectStandardOutput "$dataDir\stdout.txt" -RedirectStandardError "$dataDir\stderr.txt"
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

    function Check($name, $cond) {
        if ($cond) {
            Write-Host "  PASS $name"
        } else {
            Write-Host "  FAIL $name"
            $script:failed = $true
        }
    }
    $failed = $false

    $goodYAML = @"
schema: 1
id: smoke-public
name: Smoke Public
version: 1.0.0
type: public
protocol: declarative
links:
  - https://198.51.100.1/
search:
  method: GET
  path: /search
  query:
    q: "{{ .Query.Keyword }}"
  timeoutSeconds: 5
response:
  format: html
  rows:
    selector: "li"
  fields:
    title:
      selector: "a.t"
      value: text
      required: true
"@

    $privateYAML = @"
schema: 1
id: smoke-private
name: Smoke Private
version: 1.0.0
type: public
protocol: declarative
links:
  - https://127.0.0.1/
search:
  method: GET
  path: /search
  query:
    q: "{{ .Query.Keyword }}"
response:
  format: html
  rows:
    selector: "li"
  fields:
    title: { selector: "a", value: text, required: true }
"@

    Write-Host "[1/7] valid import"
    $imported = Invoke-RestMethod -Method Post -Uri "$base/api/v1/indexer-catalog/import" -Headers @{ "Content-Type" = "application/json" } -Body (@{ yaml = $goodYAML; test = $true } | ConvertTo-Json)
    Check "valid=true" ($imported.valid -eq $true)
    Check "persisted=true" ($imported.persisted -eq $true)
    Check "definition id populated" ($imported.definition.id -eq "smoke-public")

    Write-Host "[2/7] private-IP rejection"
    $private = Invoke-RestMethod -Method Post -Uri "$base/api/v1/indexer-catalog/import" -Headers @{ "Content-Type" = "application/json" } -Body (@{ yaml = $privateYAML; test = $true } | ConvertTo-Json)
    Check "valid=false" ($private.valid -eq $false)
    Check "errors[0].code=LINK_UNSAFE" ($private.errors[0].code -eq "LINK_UNSAFE")

    Write-Host "[3/7] oversize rejection"
    $big = "x" * 600000
    try {
        Invoke-RestMethod -Method Post -Uri "$base/api/v1/indexer-catalog/import" -Headers @{ "Content-Type" = "application/json" } -Body (@{ yaml = $big } | ConvertTo-Json) | Out-Null
        Check "oversize rejected" $false
    } catch {
        Check "oversize rejected (400)" ($_.Exception.Response.StatusCode.value__ -eq 400)
        $body = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
        Check "error code DEFINITION_TOO_LARGE" ($body.error.code -eq "DEFINITION_TOO_LARGE")
    }

    Write-Host "[4/7] broken YAML rejection"
    try {
        Invoke-RestMethod -Method Post -Uri "$base/api/v1/indexer-catalog/import" -Headers @{ "Content-Type" = "application/json" } -Body (@{ yaml = "this : is : bad" } | ConvertTo-Json) | Out-Null
        Check "broken rejected" $false
    } catch {
        Check "broken rejected (400)" ($_.Exception.Response.StatusCode.value__ -eq 400)
    }

    Write-Host "[5/7] selector injection safe"
    $injectionYAML = @"
schema: 1
id: smoke-injection
name: Smoke Injection
version: 1.0.0
type: public
protocol: declarative
links:
  - https://198.51.100.1/
search:
  method: GET
  path: /search
  query:
    q: "{{ .Query.Keyword }}"
response:
  format: html
  rows:
    selector: "li"
  fields:
    title:
      selector: 'a <script>alert(1)</script>'
      value: text
      required: true
"@
    $inj = Invoke-RestMethod -Method Post -Uri "$base/api/v1/indexer-catalog/import" -Headers @{ "Content-Type" = "application/json" } -Body (@{ yaml = $injectionYAML; test = $false } | ConvertTo-Json)
    Check "selector with <script> rejected" ($inj.valid -eq $false)
    Check "errors[0].code=SELECTOR_FORBIDDEN" ($inj.errors[0].code -eq "SELECTOR_FORBIDDEN")

    Write-Host "[6/7] imported list"
    $list = Invoke-RestMethod -Method Get -Uri "$base/api/v1/indexer-catalog/imported" -Headers @{ Accept = "application/json" }
    Check "imported list has 1" ($list.items.Count -eq 1)

    Write-Host "[7/7] install imported"
    $install = Invoke-RestMethod -Method Post -Uri "$base/api/v1/indexers" -Headers @{ "Content-Type" = "application/json" } -Body (@{ definitionId = "smoke-public"; baseUrl = "https://198.51.100.1" } | ConvertTo-Json)
    Check "installed id matches" ($install.definitionId -eq "smoke-public")
    $installed = Invoke-RestMethod -Method Get -Uri "$base/api/v1/indexers" -Headers @{ Accept = "application/json" }
    Check "list shows 1" ($installed.items.Count -eq 1)

    if ($failed) {
        Write-Host "PHASE 5 SMOKE FAILED"
        exit 1
    }
    Write-Host "PHASE 5 SMOKE OK"
    exit 0
} finally {
    if ($proc -and -not $proc.HasExited) {
        Stop-Process -Id $proc.Id -Force
    }
    Remove-Item -Recurse -Force $dataDir -ErrorAction SilentlyContinue
}
