#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

case "$(uname -m)" in
  arm64) expected="arm64" ;;
  x86_64) expected="x86_64" ;;
  *) expected="unknown" ;;
esac

# Archives created on Windows do not preserve Unix executable bits. Gatekeeper
# may also quarantine an unsigned local build after it is downloaded.
chmod +x ./SaturX-Admin
xattr -d com.apple.quarantine ./SaturX-Admin 2>/dev/null || true

binary_arch="$(file ./SaturX-Admin 2>/dev/null || true)"
if [[ "$expected" != "unknown" && "$binary_arch" != *"$expected"* ]]; then
  echo "Wrong build for this Mac ($expected): $binary_arch" >&2
  exit 1
fi

exec ./SaturX-Admin
