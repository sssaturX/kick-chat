# Сборка для юзера на Windows: Go (kick-chat) + Python (viewerbot). Читает LICENSE_SERVER_URL и LICENSE_HMAC_SECRET из .env.
# Запуск: из корня kick-chat в PowerShell: .\scripts\build-release.ps1
# Результат: папка release\ с kick-chat.exe и viewerbot.exe

$ErrorActionPreference = "Stop"

$ROOT = (Get-Item $PSScriptRoot).Parent.FullName
Set-Location $ROOT

$envFile = if ($env:ENV_FILE) { $env:ENV_FILE } else { ".env" }
if (-not (Test-Path $envFile)) {
  Write-Error "Не найден $envFile (задай LICENSE_SERVER_URL и LICENSE_HMAC_SECRET)"
  exit 1
}

$LICENSE_SERVER_URL = ""
$LICENSE_HMAC_SECRET = ""
Get-Content $envFile -Encoding UTF8 | ForEach-Object {
  $line = $_ -replace "`r", ""
  if ($line -match '^LICENSE_SERVER_URL=(.+)$') { $LICENSE_SERVER_URL = $Matches[1].Trim() }
  if ($line -match '^LICENSE_HMAC_SECRET=(.+)$') { $LICENSE_HMAC_SECRET = $Matches[1].Trim() }
}

if (-not $LICENSE_SERVER_URL -or -not $LICENSE_HMAC_SECRET) {
  Write-Error "В $envFile должны быть LICENSE_SERVER_URL и LICENSE_HMAC_SECRET"
  exit 1
}

$RELEASE_DIR = Join-Path $ROOT "release"
New-Item -ItemType Directory -Force -Path $RELEASE_DIR | Out-Null

Write-Host "=== 1. Сборка viewerbot (Python -> один бинарник) ==="
Push-Location (Join-Path $ROOT "test_view\kick-viewbot")
try {
  & .\build-viewerbot.ps1
} finally {
  Pop-Location
}

Write-Host ""
Write-Host "=== 2. Сборка kick-chat (Go, -tags release, ldflags из .env) ==="
go build -tags release -ldflags "-X main.defaultLicenseServerURL=$LICENSE_SERVER_URL -X main.defaultLicenseHMACSecret=$LICENSE_HMAC_SECRET" -o (Join-Path $RELEASE_DIR "kick-chat.exe") .

$vbDist = Join-Path $ROOT "test_view\kick-viewbot\dist"
$vbExe = Join-Path $vbDist "viewerbot.exe"
if (Test-Path $vbExe) {
  Copy-Item $vbExe (Join-Path $RELEASE_DIR "viewerbot.exe")
  Write-Host "  скопирован viewerbot.exe"
} else {
  Write-Warning "Не найден dist\viewerbot.exe — скопируй вручную в $RELEASE_DIR"
}

Write-Host ""
Write-Host "Готово: $RELEASE_DIR"
Get-ChildItem $RELEASE_DIR
Write-Host ""
Write-Host "Отдай пользователю содержимое папки release\ + инструкцию про .env (KICK_CLIENT_ID, KICK_CLIENT_SECRET)."
