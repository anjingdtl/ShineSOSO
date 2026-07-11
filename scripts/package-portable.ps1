# Build a green, no-install ZIP. End users only need to extract and
# double-click 启动 EasySearch.vbs; Node, Go, and npm are not required.
param(
    [string]$Version = "0.1.0",
    [string]$ProwlarrVersion = ""
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$releaseRoot = Join-Path $root 'dist\portable\EasySearch'
$zipPath = Join-Path $root ("dist\EasySearch-v{0}-windows-x64-portable.zip" -f $Version)
$cacheRoot = Join-Path $root 'dist\cache'
$packageBuildExe = Join-Path $root 'dist\package-build\easysearch.exe'

& (Join-Path $PSScriptRoot 'build.ps1') -OutputPath $packageBuildExe
if (Test-Path $releaseRoot) { Remove-Item -LiteralPath $releaseRoot -Recurse -Force }
New-Item -ItemType Directory -Path $releaseRoot -Force | Out-Null
Copy-Item $packageBuildExe (Join-Path $releaseRoot 'easysearch.exe')
Copy-Item (Join-Path $PSScriptRoot 'portable-start.vbs') (Join-Path $releaseRoot '启动 EasySearch.vbs')
Copy-Item (Join-Path $root 'THIRD_PARTY_NOTICES.md') (Join-Path $releaseRoot 'THIRD_PARTY_NOTICES.md')

# Prowlarr is distributed as the official Windows x64 portable ZIP. We use the
# GitHub release API rather than a hard-coded version so a release build can be
# reproduced with -ProwlarrVersion when required, while the default remains
# current and verifies GitHub's published SHA-256 asset digest.
New-Item -ItemType Directory -Path $cacheRoot -Force | Out-Null
$headers = @{ 'User-Agent' = 'EasySearch-portable-packager' }
if ($ProwlarrVersion) {
    $release = Invoke-RestMethod -Headers $headers -Uri ("https://api.github.com/repos/Prowlarr/Prowlarr/releases/tags/v{0}" -f $ProwlarrVersion)
} else {
    $release = Invoke-RestMethod -Headers $headers -Uri 'https://api.github.com/repos/Prowlarr/Prowlarr/releases/latest'
}
$asset = @($release.assets | Where-Object { $_.name -match '\.windows-core-x64\.zip$' })[0]
if (-not $asset) { throw 'Unable to locate the official Prowlarr Windows x64 portable ZIP.' }
$assetPath = Join-Path $cacheRoot $asset.name
if (-not (Test-Path $assetPath) -or (Get-Item $assetPath).Length -eq 0) {
    Write-Host "[portable] downloading $($asset.name) ..." -ForegroundColor Cyan
    Invoke-WebRequest -Headers $headers -Uri $asset.browser_download_url -OutFile $assetPath
}
if ($asset.digest) {
    $expected = ($asset.digest -replace '^sha256:', '').ToLowerInvariant()
    $actual = (Get-FileHash -LiteralPath $assetPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected) { throw "Prowlarr SHA-256 mismatch. Expected $expected, got $actual" }
}
$extractRoot = Join-Path $cacheRoot ([IO.Path]::GetFileNameWithoutExtension($asset.name))
if (Test-Path $extractRoot) { Remove-Item -LiteralPath $extractRoot -Recurse -Force }
Expand-Archive -LiteralPath $assetPath -DestinationPath $extractRoot -Force
$prowlarrExe = Get-ChildItem -LiteralPath $extractRoot -Filter 'Prowlarr.exe' -Recurse | Select-Object -First 1
if (-not $prowlarrExe) { throw 'Official Prowlarr package does not contain Prowlarr.exe.' }
$runtimeRoot = Join-Path $releaseRoot 'runtime\prowlarr'
New-Item -ItemType Directory -Path $runtimeRoot -Force | Out-Null
Copy-Item -Path (Join-Path $prowlarrExe.Directory.FullName '*') -Destination $runtimeRoot -Recurse -Force
$license = Get-ChildItem -LiteralPath $prowlarrExe.Directory.FullName -File | Where-Object { $_.Name -match '^(LICENSE|COPYING)(\.md|\.txt)?$' } | Select-Object -First 1
if (-not $license) { throw 'Official Prowlarr package does not contain a GPL license file.' }
$licenseDir = Join-Path $releaseRoot 'third-party'
New-Item -ItemType Directory -Path $licenseDir -Force | Out-Null
Copy-Item -LiteralPath $license.FullName -Destination (Join-Path $licenseDir 'prowlarr-GPL-3.0.txt') -Force
@"
EasySearch 绿色免安装版

使用方法：
1. 解压整个文件夹。
2. 双击“启动 EasySearch.vbs”。
3. 程序会自动打开浏览器；无需安装 Node、Go、Python、Prowlarr 或其他依赖。

数据保存在 %APPDATA%\EasySearch\data，不会写入本文件夹。

索引器引擎：首次启动时 EasySearch 会在后台自动启动本包内的 Prowlarr，
其运行数据和 API Key 仅保存在 %APPDATA%\EasySearch\data\prowlarr。
Prowlarr 的 GPL-3.0 许可证与来源说明见 third-party 和 THIRD_PARTY_NOTICES.md。
"@ | Set-Content -LiteralPath (Join-Path $releaseRoot '使用说明.txt') -Encoding utf8
if (Test-Path $zipPath) { Remove-Item -LiteralPath $zipPath -Force }
Compress-Archive -Path $releaseRoot -DestinationPath $zipPath -CompressionLevel Optimal
Write-Host "[portable] created $zipPath" -ForegroundColor Green
