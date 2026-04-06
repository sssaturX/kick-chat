# Pack release\SaturX into a ZIP without a full rebuild.
# From repo root: .\scripts\zip-saturx.ps1
# Or: powershell -ExecutionPolicy Bypass -File .\scripts\zip-saturx.ps1

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

$ROOT = (Get-Item $PSScriptRoot).Parent.FullName
$USER_PACKAGE = Join-Path $ROOT "release\SaturX"

if (-not (Test-Path $USER_PACKAGE)) {
  Write-Error "Missing folder: $USER_PACKAGE (run .\scripts\build-release.ps1 first)"
  exit 1
}

$zipName = "SaturX-$(Get-Date -Format 'yyyyMMdd-HHmmss').zip"
$zipPath = Join-Path (Join-Path $ROOT "release") $zipName

if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
Compress-Archive -LiteralPath $USER_PACKAGE -DestinationPath $zipPath

Write-Host "Created: $zipPath"
Get-Item $zipPath | Format-List FullName, Length, LastWriteTime
