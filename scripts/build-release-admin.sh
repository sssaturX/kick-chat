#!/usr/bin/env bash
# Admin release: same as build-release.sh but license checks OFF (embedded). No LICENSE_* in .env required.
# Output: release/SaturX-Admin/
# Run from repo root: ./scripts/build-release-admin.sh
# Cross-build Windows: ./scripts/build-release-admin.sh windows

set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

TARGET="${1:-}"
if [[ "$TARGET" = "windows" ]]; then
  export GOOS=windows
  export GOARCH=amd64
  BIN_NAME="SaturX-Admin.exe"
  BUILD_WINDOWS=1
else
  BIN_NAME="SaturX-Admin"
  BUILD_WINDOWS=0
fi

USER_PACKAGE="$ROOT/release/SaturX-Admin"
mkdir -p "$USER_PACKAGE"

echo "=== Admin release (no license) ==="

if [[ $BUILD_WINDOWS -eq 1 ]]; then
  echo "=== Target: Windows (cross-build). Viewerbot not built here — build viewerbot on Windows if needed. ==="
else
  echo "=== 1. Building viewerbot (PyInstaller) ==="
  VB_DIR="$ROOT/test_view/kick-viewbot"
  if [[ ! -f "$VB_DIR/build-viewerbot.sh" ]]; then
    echo "Error: not found $VB_DIR/build-viewerbot.sh" >&2
    exit 1
  fi
  (cd "$VB_DIR" && ./build-viewerbot.sh) || {
    echo "Warning: viewerbot build failed. Run manually: cd test_view/kick-viewbot && ./build-viewerbot.sh" >&2
  }
fi

echo ""
echo "=== 2. Building SaturX-Admin (Go) ==="
if [[ $BUILD_WINDOWS -eq 1 ]]; then
  echo "  GOOS=windows GOARCH=amd64 -> $BIN_NAME"
fi
go build -tags release \
  -ldflags "-s -w -X main.defaultAdminRelease=1 -X main.defaultLicenseServerURL= -X main.defaultLicenseHMACSecret=" \
  -o "$USER_PACKAGE/$BIN_NAME" .

echo "Output: $USER_PACKAGE/$BIN_NAME"

if [[ $BUILD_WINDOWS -eq 0 ]]; then
  VB_DIST="$ROOT/test_view/kick-viewbot/dist"
  if [[ -f "$VB_DIST/viewerbot" ]]; then
    cp "$VB_DIST/viewerbot" "$USER_PACKAGE/viewerbot"
    echo "Copied viewerbot"
  elif [[ -f "$VB_DIST/viewerbot.exe" ]]; then
    cp "$VB_DIST/viewerbot.exe" "$USER_PACKAGE/viewerbot.exe"
    echo "Copied viewerbot.exe"
  else
    echo "Warning: viewerbot not found" >&2
  fi

  cat > "$USER_PACKAGE/run-saturx.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"
if [[ ! -x "./SaturX-Admin" ]]; then
  chmod +x "./SaturX-Admin" 2>/dev/null || true
fi
exec "./SaturX-Admin"
EOF
  chmod +x "$USER_PACKAGE/run-saturx.sh"
  echo "Created run-saturx.sh (runs SaturX-Admin)"
fi

for pair in \
  ".env.example:.env.example" \
  "kick-emotes.json:kick-emotes.json" \
  "USER-GUIDE.md:USER-GUIDE.md" \
  "release/README.txt:README.txt" \
  "release/README-USER.md:README.md" \
  "release/README-ADMIN.txt:README-ADMIN.txt"
do
  src="${pair%%:*}"
  dest="${pair##*:}"
  if [[ -f "$ROOT/$src" ]]; then
    cp "$ROOT/$src" "$USER_PACKAGE/$dest"
    echo "Copied $dest"
  fi
done

echo ""
echo "Done. Admin folder: $USER_PACKAGE"
ls -la "$USER_PACKAGE"
echo ""
echo "Do not ship SaturX-Admin to customers."
