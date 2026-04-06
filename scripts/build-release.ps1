# Build release for Windows: SaturX.exe (icon + version) + viewerbot.exe.
# Output: release\SaturX\ and release\SaturX-yyyyMMdd-HHmmss.zip
# Run from repo root: .\scripts\build-release.ps1  OR  .\scripts\build-release.bat
# If script is blocked: powershell -ExecutionPolicy Bypass -File .\scripts\build-release.ps1

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

$ROOT = (Get-Item $PSScriptRoot).Parent.FullName
Set-Location $ROOT

$envFile = if ($env:ENV_FILE) { $env:ENV_FILE } else { ".env" }
if (-not (Test-Path $envFile)) {
  Write-Error "Missing $envFile (set LICENSE_SERVER_URL and LICENSE_HMAC_SECRET)"
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
  Write-Error "Set LICENSE_SERVER_URL and LICENSE_HMAC_SECRET in $envFile"
  exit 1
}

$USER_PACKAGE = Join-Path $ROOT "release\SaturX"
New-Item -ItemType Directory -Force -Path $USER_PACKAGE | Out-Null

function Get-GoExecutable {
  $fromPath = Get-Command go -ErrorAction SilentlyContinue
  if ($fromPath -and $fromPath.Source -and (Test-Path $fromPath.Source)) {
    return $fromPath.Source
  }
  $candidates = @()
  if ($env:GOROOT) {
    $candidates += (Join-Path $env:GOROOT "bin\go.exe")
  }
  $candidates += @(
    "C:\Program Files\Go\bin\go.exe"
    "${env:ProgramFiles(x86)}\Go\bin\go.exe"
  )
  foreach ($p in $candidates) {
    if ($p -and (Test-Path $p)) { return (Resolve-Path $p).Path }
  }
  return $null
}

$goExe = Get-GoExecutable
if (-not $goExe) {
  Write-Error @"
Go (go.exe) not found. Install from https://go.dev/dl/ (Windows MSI), then either:
  - open a new terminal so PATH includes Go (e.g. C:\Program Files\Go\bin), or
  - set GOROOT to your Go install and ensure GOROOT\bin is on PATH.
"@
  exit 1
}
Write-Host "Using Go: $goExe"

Write-Host "=== 1. Building viewerbot (PyInstaller) ==="
$vbDir = Join-Path $ROOT "test_view\kick-viewbot"
$vbScript = Join-Path $vbDir "build-viewerbot.ps1"
if (-not (Test-Path $vbScript)) {
  Write-Error "Not found: $vbScript"
  exit 1
}
Push-Location $vbDir
try {
  & powershell -ExecutionPolicy Bypass -NoProfile -File $vbScript
  if ($LASTEXITCODE -ne 0) {
    Write-Warning "viewerbot build failed (exit $LASTEXITCODE). Run manually: cd test_view\kick-viewbot; .\build-viewerbot.ps1"
  }
} finally {
  Pop-Location
}

Write-Host ""
Write-Host "=== 2. Windows icon + version (goversioninfo) ==="
$iconPath = Join-Path $ROOT "build\icon.ico"
$sysoOut = Join-Path $ROOT "resource.syso"
$versioninfoJson = Join-Path $ROOT "versioninfo.json"
if (-not (Test-Path $versioninfoJson)) {
  Write-Host "versioninfo.json not found, skipping icon/version"
} else {
  # go run goversioninfo; -64 for 64-bit exe (default would be 386)
  $goversioninfoArgs = @("-64", "-o", $sysoOut, $versioninfoJson)
  if (Test-Path $iconPath) {
    $goversioninfoArgs = @("-64", "-icon", $iconPath, "-o", $sysoOut, $versioninfoJson)
    Write-Host "Using icon: build\icon.ico"
  } else {
    Write-Host "build\icon.ico not found, building without icon"
  }
  & $goExe run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest @goversioninfoArgs
  if ($LASTEXITCODE -ne 0) {
    Write-Warning "goversioninfo failed (exit $LASTEXITCODE). Exe will have no custom icon. Check that build\icon.ico is valid .ico (e.g. 256x256 or multi-size)."
  } elseif (Test-Path $sysoOut) {
    Write-Host "resource.syso created (icon + version info)"
  }
}

Write-Host ""
Write-Host "=== 3. Building SaturX (Go) ==="
$ldflags = "-s -w -X main.defaultLicenseServerURL=$LICENSE_SERVER_URL -X main.defaultLicenseHMACSecret=$LICENSE_HMAC_SECRET"
$exeName = "SaturX.exe"
& $goExe build -tags release -ldflags $ldflags -o (Join-Path $USER_PACKAGE $exeName) .
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Output: $USER_PACKAGE\$exeName"
if (Test-Path $sysoOut) { Remove-Item $sysoOut -Force }

$vbDistExe = Join-Path $ROOT "test_view\kick-viewbot\dist\viewerbot.exe"
$vbRootExe = Join-Path $ROOT "test_view\kick-viewbot\viewerbot.exe"
if (Test-Path $vbDistExe) {
  Copy-Item $vbDistExe (Join-Path $USER_PACKAGE "viewerbot.exe") -Force
  Write-Host "Copied viewerbot.exe"
} elseif (Test-Path $vbRootExe) {
  Copy-Item $vbRootExe (Join-Path $USER_PACKAGE "viewerbot.exe") -Force
  Write-Host "Copied viewerbot.exe"
} else {
  Write-Warning "viewerbot.exe not found - run: cd test_view\kick-viewbot; .\build-viewerbot.ps1"
}

foreach ($pair in @(
  @{ Src = ".env.example"; Dest = ".env.example" }
  @{ Src = "USER-GUIDE.md"; Dest = "USER-GUIDE.md" }
  @{ Src = "release\README.txt"; Dest = "README.txt" }
  @{ Src = "release\README-USER.md"; Dest = "README.md" }
)) {
  $src = Join-Path $ROOT $pair.Src
  if (Test-Path $src) {
    Copy-Item $src (Join-Path $USER_PACKAGE $pair.Dest) -Force
    Write-Host "Copied $($pair.Dest)"
  }
}

Write-Host ""
Write-Host "=== 4. ZIP (server / distribution) ==="
$zipName = "SaturX-$(Get-Date -Format 'yyyyMMdd-HHmmss').zip"
$zipPath = Join-Path (Join-Path $ROOT "release") $zipName
try {
  if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
  Compress-Archive -LiteralPath $USER_PACKAGE -DestinationPath $zipPath
  Write-Host "ZIP: $zipPath"
} catch {
  Write-Warning "Could not create ZIP: $_"
}

Write-Host ""
Write-Host "Done. Folder for user: $USER_PACKAGE"
Get-ChildItem $USER_PACKAGE
# Use single-quoted tail: parentheses after .env break parsing in double-quoted strings
Write-Host ('For server: upload ' + $zipName + ' or folder release\SaturX. User needs .env with KICK_CLIENT_ID, KICK_CLIENT_SECRET, CHANNEL_SLUG')
