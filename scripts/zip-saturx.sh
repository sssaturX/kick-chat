#!/usr/bin/env bash
# Pack release/SaturX into a ZIP without a full rebuild.
# From repo root: ./scripts/zip-saturx.sh
# (first time: chmod +x scripts/zip-saturx.sh)

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
USER_PACKAGE="$ROOT/release/SaturX"

if [[ ! -d "$USER_PACKAGE" ]]; then
  echo "Missing folder: $USER_PACKAGE (run ./scripts/build-release.sh first)" >&2
  exit 1
fi

ZIP_NAME="SaturX-$(date +%Y%m%d-%H%M%S).zip"
ZIP_PATH="$ROOT/release/$ZIP_NAME"

if command -v zip >/dev/null 2>&1; then
  (
    cd "$ROOT/release"
    zip -r "$ZIP_NAME" "SaturX" >/dev/null
  )
elif command -v 7z >/dev/null 2>&1; then
  (
    cd "$ROOT/release"
    7z a -tzip "$ZIP_NAME" "SaturX" >/dev/null
  )
else
  echo "Error: neither 'zip' nor '7z' found. Install one of them and retry." >&2
  exit 1
fi

echo "Created: $ZIP_PATH"
ls -lh "$ZIP_PATH"
