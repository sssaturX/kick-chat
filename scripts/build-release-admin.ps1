# Admin release: same as build-release.ps1 (SaturX + viewerbot + zip) but license checks are OFF (embedded).
# Does not read LICENSE_SERVER_URL / LICENSE_HMAC_SECRET from .env.
# Output: release\SaturX-Admin\ and release\SaturX-Admin-yyyyMMdd-HHmmss.zip
# Run from repo root: .\scripts\build-release-admin.ps1

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

$ROOT = (Get-Item $PSScriptRoot).Parent.FullName
Set-Location $ROOT

$RELEASE_ROOT = Join-Path $ROOT "release"
$USER_PACKAGE = Join-Path $RELEASE_ROOT "SaturX-Admin"
if (Test-Path -LiteralPath $USER_PACKAGE) {
  $resolvedPackage = (Resolve-Path -LiteralPath $USER_PACKAGE).Path
  $resolvedRelease = (Resolve-Path -LiteralPath $RELEASE_ROOT).Path
  if (-not $resolvedPackage.StartsWith($resolvedRelease, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to remove outside release: $resolvedPackage"
  }
  Remove-Item -LiteralPath $resolvedPackage -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $USER_PACKAGE | Out-Null

function Get-GoExecutable {
  if ($env:GO_EXE -and (Test-Path -LiteralPath $env:GO_EXE)) {
    return (Resolve-Path -LiteralPath $env:GO_EXE).Path
  }
  $fromPath = Get-Command go -ErrorAction SilentlyContinue
  if ($fromPath -and $fromPath.Source -and (Test-Path -LiteralPath $fromPath.Source)) {
    return $fromPath.Source
  }
  $candidates = @()
  if ($env:GOROOT) {
    $candidates += (Join-Path $env:GOROOT "bin\go.exe")
  }
  if ($env:LOCALAPPDATA) {
    $candidates += (Join-Path $env:LOCALAPPDATA "Programs\Go\bin\go.exe")
  }
  if ($env:USERPROFILE) {
    $candidates += (Join-Path $env:USERPROFILE "scoop\apps\go\current\bin\go.exe")
    $candidates += (Join-Path $env:USERPROFILE "sdk\go\bin\go.exe")
  }
  $candidates += @(
    "C:\Program Files\Go\bin\go.exe"
    "${env:ProgramFiles(x86)}\Go\bin\go.exe"
  )
  foreach ($p in $candidates) {
    if ($p -and (Test-Path -LiteralPath $p)) { return (Resolve-Path -LiteralPath $p).Path }
  }
  return $null
}

$goExe = Get-GoExecutable
if (-not $goExe) {
  Write-Error @"
Go (go.exe) not found. Install from https://go.dev/dl/ (Windows MSI), then either:
  - open a new terminal so PATH includes Go (e.g. C:\Program Files\Go\bin), or
  - set GOROOT to your Go install (folder that contains bin\go.exe), or
  - set GO_EXE to the full path of go.exe, e.g. GO_EXE=$env:LOCALAPPDATA\Programs\Go\bin\go.exe
"@
  exit 1
}
Write-Host "Using Go: $goExe"
Write-Host "=== Admin release (no license) ==="

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
  $goversioninfoArgs = @("-64", "-o", $sysoOut, $versioninfoJson)
  if (Test-Path $iconPath) {
    $goversioninfoArgs = @("-64", "-icon", $iconPath, "-o", $sysoOut, $versioninfoJson)
    Write-Host "Using icon: build\icon.ico"
  } else {
    Write-Host "build\icon.ico not found, building without icon"
  }
  & $goExe run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest @goversioninfoArgs
  if ($LASTEXITCODE -ne 0) {
    Write-Warning "goversioninfo failed (exit $LASTEXITCODE). Exe will have no custom icon."
  } elseif (Test-Path $sysoOut) {
    Write-Host "resource.syso created (icon + version info)"
  }
}

Write-Host ""
Write-Host "=== 3. Building SaturX-Admin (Go, admin release ldflags) ==="
$ldflags = "-s -w -X main.defaultAdminRelease=1 -X main.defaultLicenseServerURL= -X main.defaultLicenseHMACSecret="
$exeName = "SaturX-Admin.exe"
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
  @{ Src = "kick-emotes.json"; Dest = "kick-emotes.json" }
  @{ Src = "USER-GUIDE.md"; Dest = "USER-GUIDE.md" }
  @{ Src = "release\README.txt"; Dest = "README.txt" }
  @{ Src = "release\README-USER.md"; Dest = "README.md" }
  @{ Src = "release\README-ADMIN.txt"; Dest = "README-ADMIN.txt" }
)) {
  $src = Join-Path $ROOT $pair.Src
  if (Test-Path $src) {
    Copy-Item $src (Join-Path $USER_PACKAGE $pair.Dest) -Force
    Write-Host "Copied $($pair.Dest)"
  }
}

Write-Host ""
Write-Host "=== 4. ZIP ==="
$zipName = "SaturX-Admin-$(Get-Date -Format 'yyyyMMdd-HHmmss').zip"
$zipPath = Join-Path (Join-Path $ROOT "release") $zipName
try {
  if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
  Compress-Archive -LiteralPath $USER_PACKAGE -DestinationPath $zipPath
  Write-Host "ZIP: $zipPath"
} catch {
  Write-Warning "Could not create ZIP: $_"
}

Write-Host ""
Write-Host "Done. Admin folder: $USER_PACKAGE"
Get-ChildItem $USER_PACKAGE
Write-Host ('Do not ship SaturX-Admin to customers. Kick: .env with KICK_CLIENT_ID, KICK_CLIENT_SECRET, CHANNEL_SLUG')
