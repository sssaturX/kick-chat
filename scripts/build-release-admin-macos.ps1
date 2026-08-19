# Cross-build the license-free admin release for macOS from Windows.
# Produces separate archives for Apple Silicon and Intel Macs.

[CmdletBinding()]
param(
    [ValidateSet("all", "arm64", "amd64")]
    [string]$Arch = "all"
)

$ErrorActionPreference = "Stop"
$ROOT = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$RELEASE_ROOT = Join-Path $ROOT "release"
$architectures = if ($Arch -eq "all") { @("arm64", "amd64") } else { @($Arch) }
$ldflags = "-s -w -X main.defaultAdminRelease=1 -X main.defaultLicenseServerURL= -X main.defaultLicenseHMACSecret="
$copyFiles = @(
    @{ Src = ".env.example"; Dest = ".env.example" },
    @{ Src = "kick-emotes.json"; Dest = "kick-emotes.json" },
    @{ Src = "USER-GUIDE.md"; Dest = "USER-GUIDE.md" },
    @{ Src = "release\README.txt"; Dest = "README.txt" },
    @{ Src = "release\README-USER.md"; Dest = "README.md" },
    @{ Src = "release\README-ADMIN.txt"; Dest = "README-ADMIN.txt" }
)

Push-Location $ROOT
$oldGoos = $env:GOOS
$oldGoarch = $env:GOARCH
$oldCgo = $env:CGO_ENABLED
$oldGocache = $env:GOCACHE
try {
    $env:GOOS = "darwin"
    $env:CGO_ENABLED = "0"
    $env:GOCACHE = Join-Path $ROOT ".cache\go-build"

    foreach ($targetArch in $architectures) {
        $env:GOARCH = $targetArch
        $packageName = "SaturX-Admin-macOS-$targetArch"
        $packageDir = Join-Path $RELEASE_ROOT $packageName
        New-Item -ItemType Directory -Force -Path $packageDir | Out-Null

        Write-Host "Building macOS/$targetArch admin release..."
        & go build -tags release -ldflags $ldflags -o (Join-Path $packageDir "SaturX-Admin") .
        if ($LASTEXITCODE -ne 0) { throw "go build failed for macOS/$targetArch" }

        foreach ($file in $copyFiles) {
            $source = Join-Path $ROOT $file.Src
            if (Test-Path $source) {
                Copy-Item -LiteralPath $source -Destination (Join-Path $packageDir $file.Dest) -Force
            }
        }

        Copy-Item -LiteralPath (Join-Path $PSScriptRoot "run-saturx-macos.sh") -Destination (Join-Path $packageDir "run-saturx.sh") -Force

        $zipPath = Join-Path $RELEASE_ROOT "$packageName.zip"
        Compress-Archive -Path $packageDir -DestinationPath $zipPath -Force
        Write-Host "Created: $zipPath"
    }
}
finally {
    $env:GOOS = $oldGoos
    $env:GOARCH = $oldGoarch
    $env:CGO_ENABLED = $oldCgo
    $env:GOCACHE = $oldGocache
    Pop-Location
}

Write-Host "Admin build has license checks disabled. Do not distribute it to customers."
