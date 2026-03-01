#!/usr/bin/env bash
# Сборка для юзера: Go (kick-chat) + Python (viewerbot). Читает LICENSE_SERVER_URL и LICENSE_HMAC_SECRET из .env.
# Запуск: из корня kick-chat — ./scripts/build-release.sh
# Результат: папка release/ с kick-chat и viewerbot (или viewerbot.exe на Windows).
# Сборка для Windows с Mac: ./scripts/build-release.sh windows

set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Цель: по умолчанию текущая ОС; windows — кросс-сборка Go для Windows (с Mac/Linux)
TARGET="${1:-${TARGET}}"
if [[ "$TARGET" = "windows" ]]; then
  export GOOS=windows
  export GOARCH=amd64
  GO_OUTPUT="kick-chat.exe"
  BUILD_WINDOWS=1
else
  GO_OUTPUT="kick-chat"
  BUILD_WINDOWS=0
fi

ENV_FILE="${ENV_FILE:-.env}"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "Ошибка: не найден $ENV_FILE (задай LICENSE_SERVER_URL и LICENSE_HMAC_SECRET)" >&2
  exit 1
fi

# Читаем флаги для Go из .env (строки LICENSE_SERVER_URL=... и LICENSE_HMAC_SECRET=...)
LICENSE_SERVER_URL=$(grep -E '^LICENSE_SERVER_URL=' "$ENV_FILE" | cut -d= -f2- | tr -d '\r')
LICENSE_HMAC_SECRET=$(grep -E '^LICENSE_HMAC_SECRET=' "$ENV_FILE" | cut -d= -f2- | tr -d '\r')

if [[ -z "$LICENSE_SERVER_URL" || -z "$LICENSE_HMAC_SECRET" ]]; then
  echo "Ошибка: в $ENV_FILE должны быть LICENSE_SERVER_URL и LICENSE_HMAC_SECRET" >&2
  exit 1
fi

RELEASE_DIR="$ROOT/release"
mkdir -p "$RELEASE_DIR"

if [[ $BUILD_WINDOWS -eq 1 ]]; then
  echo "=== Цель: Windows (кросс-сборка Go с Mac). Viewerbot не собираем — viewerbot.exe собери на Windows: test_view\\kick-viewbot\\build-viewerbot.ps1 ==="
else
  echo "=== 1. Сборка viewerbot (Python -> один бинарник) ==="
  (cd "$ROOT/test_view/kick-viewbot" && ./build-viewerbot.sh)
fi

echo ""
echo "=== 2. Сборка kick-chat (Go, -tags release, ldflags из .env) ==="
if [[ $BUILD_WINDOWS -eq 1 ]]; then
  echo "  GOOS=windows GOARCH=amd64 -> $GO_OUTPUT"
fi
go build -tags release -ldflags "\
  -X main.defaultLicenseServerURL=$LICENSE_SERVER_URL \
  -X main.defaultLicenseHMACSecret=$LICENSE_HMAC_SECRET" \
  -o "$RELEASE_DIR/$GO_OUTPUT" .

if [[ $BUILD_WINDOWS -eq 0 ]]; then
  VB_DIST="$ROOT/test_view/kick-viewbot/dist"
  if [[ -f "$VB_DIST/viewerbot" ]]; then
    cp "$VB_DIST/viewerbot" "$RELEASE_DIR/viewerbot"
    echo "  скопирован viewerbot"
  elif [[ -f "$VB_DIST/viewerbot.exe" ]]; then
    cp "$VB_DIST/viewerbot.exe" "$RELEASE_DIR/viewerbot.exe"
    echo "  скопирован viewerbot.exe"
  else
    echo "  предупреждение: не найден dist/viewerbot или dist/viewerbot.exe — скопируй вручную в $RELEASE_DIR" >&2
  fi
else
  echo "  viewerbot.exe добавь в release/ после сборки на Windows (см. выше)."
fi

echo ""
echo "Готово: $RELEASE_DIR/"
ls -la "$RELEASE_DIR"
echo ""
if [[ $BUILD_WINDOWS -eq 1 ]]; then
  echo "В release/ лежит kick-chat.exe. viewerbot.exe собери на Windows (PowerShell: .\\scripts\\build-release.ps1 или только viewerbot: test_view\\kick-viewbot\\build-viewerbot.ps1) и положи рядом."
else
  echo "Отдай пользователю содержимое папки release/ + инструкцию про .env (KICK_CLIENT_ID, KICK_CLIENT_SECRET)."
fi
