# Build a green, no-install ZIP. End users only need to extract and
# double-click 启动 EasySearch.vbs; Node, Go, and npm are not required.
param(
    [string]$Version = "0.1.0"
)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$releaseRoot = Join-Path $root 'dist\portable\EasySearch'
$zipPath = Join-Path $root ("dist\EasySearch-v{0}-windows-x64-portable.zip" -f $Version)

& (Join-Path $PSScriptRoot 'build.ps1')
if (Test-Path $releaseRoot) { Remove-Item -LiteralPath $releaseRoot -Recurse -Force }
New-Item -ItemType Directory -Path $releaseRoot -Force | Out-Null
Copy-Item (Join-Path $root 'dist\easysearch.exe') (Join-Path $releaseRoot 'easysearch.exe')
Copy-Item (Join-Path $PSScriptRoot 'portable-start.vbs') (Join-Path $releaseRoot '启动 EasySearch.vbs')
@"
EasySearch 绿色免安装版

使用方法：
1. 解压整个文件夹。
2. 双击“启动 EasySearch.vbs”。
3. 程序会自动打开浏览器；无需安装 Node、Go、Python 或其他依赖。

数据保存在 %APPDATA%\EasySearch\data，不会写入本文件夹。
"@ | Set-Content -LiteralPath (Join-Path $releaseRoot '使用说明.txt') -Encoding utf8
if (Test-Path $zipPath) { Remove-Item -LiteralPath $zipPath -Force }
Compress-Archive -Path $releaseRoot -DestinationPath $zipPath -CompressionLevel Optimal
Write-Host "[portable] created $zipPath" -ForegroundColor Green
