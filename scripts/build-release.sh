#!/usr/bin/env bash
# Build release for macOS/Linux: SaturX binary + viewerbot.
# Same layout as Windows (build-release.ps1): output in release/SaturX/.
# Run from repo root: ./scripts/build-release.sh
#   (first time: chmod +x scripts/build-release.sh)
# Cross-build for Windows from Mac/Linux: ./scripts/build-release.sh windows

set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Target: current OS by default; "windows" = cross-build Go for Windows (viewerbot not built)
TARGET="${1:-}"
if [[ "$TARGET" = "windows" ]]; then
  export GOOS=windows
  export GOARCH=amd64
  BIN_NAME="SaturX.exe"
  BUILD_WINDOWS=1
else
  BIN_NAME="SaturX"
  BUILD_WINDOWS=0
fi

USER_PACKAGE="$ROOT/release/SaturX"
mkdir -p "$USER_PACKAGE"

ENV_FILE="${ENV_FILE:-.env}"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "Ошибка: не найден $ENV_FILE (задай LICENSE_SERVER_URL и LICENSE_HMAC_SECRET)" >&2
  exit 1
fi

LICENSE_SERVER_URL=$(grep -E '^LICENSE_SERVER_URL=' "$ENV_FILE" | cut -d= -f2- | tr -d '\r')
LICENSE_HMAC_SECRET=$(grep -E '^LICENSE_HMAC_SECRET=' "$ENV_FILE" | cut -d= -f2- | tr -d '\r')

if [[ -z "$LICENSE_SERVER_URL" || -z "$LICENSE_HMAC_SECRET" ]]; then
  echo "Ошибка: в $ENV_FILE должны быть LICENSE_SERVER_URL и LICENSE_HMAC_SECRET" >&2
  exit 1
fi

if [[ $BUILD_WINDOWS -eq 1 ]]; then
  echo "=== Цель: Windows (кросс-сборка). Viewerbot не собираем — viewerbot.exe собери на Windows. ==="
else
  echo "=== 1. Building viewerbot (PyInstaller) ==="
  VB_DIR="$ROOT/test_view/kick-viewbot"
  if [[ ! -f "$VB_DIR/build-viewerbot.sh" ]]; then
    echo "Ошибка: не найден $VB_DIR/build-viewerbot.sh" >&2
    exit 1
  fi
  (cd "$VB_DIR" && ./build-viewerbot.sh) || {
    echo "Предупреждение: сборка viewerbot завершилась с ошибкой. Запусти вручную: cd test_view/kick-viewbot && ./build-viewerbot.sh" >&2
  }
fi

echo ""
echo "=== 2. Building SaturX (Go) ==="
if [[ $BUILD_WINDOWS -eq 1 ]]; then
  echo "  GOOS=windows GOARCH=amd64 -> $BIN_NAME"
fi
go build -tags release -ldflags "\
  -s -w \
  -X main.defaultLicenseServerURL=$LICENSE_SERVER_URL \
  -X main.defaultLicenseHMACSecret=$LICENSE_HMAC_SECRET" \
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
    echo "Предупреждение: viewerbot не найден — собери: cd test_view/kick-viewbot && ./build-viewerbot.sh" >&2
  fi

  cat > "$USER_PACKAGE/run-saturx.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

if [[ ! -x "./SaturX" ]]; then
  chmod +x "./SaturX" 2>/dev/null || true
fi

exec "./SaturX"
EOF
  chmod +x "$USER_PACKAGE/run-saturx.sh"
  echo "Created run-saturx.sh"
fi

# Same copy list as build-release.ps1
for pair in \
  ".env.example:.env.example" \
  "USER-GUIDE.md:USER-GUIDE.md" \
  "release/README.txt:README.txt" \
  "release/README-USER.md:README.md"
do
  src="${pair%%:*}"
  dest="${pair##*:}"
  if [[ -f "$ROOT/$src" ]]; then
    cp "$ROOT/$src" "$USER_PACKAGE/$dest"
    echo "Copied $dest"
  fi
done

echo ""
echo "Done. Folder for user: $USER_PACKAGE"
ls -la "$USER_PACKAGE"
echo ""
echo "Give user: zip folder 'SaturX' + .env (KICK_CLIENT_ID, KICK_CLIENT_SECRET, CHANNEL_SLUG)"
