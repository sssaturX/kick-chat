#!/usr/bin/env bash
# Полный запуск для теста: Postgres + Redis + License Server, создание тестовой лицензии.
# Kick-chat нужно запустить отдельно (см. конец скрипта).

set -e
cd "$(dirname "$0")/.."
ROOT="$(pwd)"
LS_DIR="$ROOT/license-server"

echo "=== 1. Поднимаем Postgres и Redis (Docker) ==="
docker compose -f "$LS_DIR/docker-compose.yml" up -d postgres redis

echo "=== 2. Ждём готовности Postgres ==="
for i in 1 2 3 4 5 6 7 8 9 10; do
  if docker compose -f "$LS_DIR/docker-compose.yml" exec -T postgres pg_isready -U postgres 2>/dev/null; then
    break
  fi
  echo "  ждём Postgres... ($i/10)"
  sleep 2
done

echo "=== 3. Запускаем License Server (фоново) ==="
export PORT=8000
export DATABASE_URL="postgres://postgres:postgres@localhost:5434/licensedb?sslmode=disable"
export REDIS_URL="redis://localhost:6379/0"
export HMAC_SECRET="${HMAC_SECRET:-test-hmac-secret-32bytes-long}"
export ADMIN_API_KEY="${ADMIN_API_KEY:-santori}"

# Запуск в фоне; логи в файл
( cd "$LS_DIR" && go run ./cmd/server >> "$ROOT/license-server.log" 2>&1 ) &
LS_PID=$!
echo "  License Server PID: $LS_PID"

echo "=== 4. Ждём /health ==="
SERVER_OK=
for i in 1 2 3 4 5 6 7 8 9 10; do
  if curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/health 2>/dev/null | grep -q 200; then
    echo "  License Server готов."
    SERVER_OK=1
    break
  fi
  echo "  ждём сервер... ($i/10)"
  sleep 2
done
if [[ -z "$SERVER_OK" ]]; then
  echo "  License Server не ответил. Последние строки лога:"
  tail -20 "$ROOT/license-server.log" 2>/dev/null || echo "  (лог пуст или не создан)"
  exit 1
fi

echo "=== 5. Создаём тестовую лицензию ==="
# expires_at через 30 дней (macOS: -v+30d, Linux: -d "+30 days")
if date -u -v+30d &>/dev/null; then
  EXPIRES_ISO=$(date -u -v+30d +"%Y-%m-%dT%H:%M:%SZ")
else
  EXPIRES_ISO=$(date -u -d "+30 days" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "2026-12-31T23:59:59Z")
fi

curl -s -X POST http://localhost:8000/admin/licenses \
  -H "Content-Type: application/json" \
  -H "X-Admin-API-Key: $ADMIN_API_KEY" \
  -d "{\"license_key\":\"TEST-KEY-1234\",\"expires_at\":\"$EXPIRES_ISO\",\"max_activations\":3}" \
  | head -5

echo ""
echo "=== Готово. Тестовая лицензия: TEST-KEY-1234 ==="
echo ""
echo "Запустите Kick-chat в другом терминале:"
echo ""
echo "  cd $ROOT"
echo "  export KICK_CLIENT_ID=\"ваш_client_id\""
echo "  export KICK_CLIENT_SECRET=\"ваш_client_secret\""
echo "  export LICENSE_SERVER_URL=\"http://localhost:8000\""
echo "  export LICENSE_HMAC_SECRET=\"$HMAC_SECRET\""
echo "  go run ."
echo ""
echo "Откройте http://localhost:8080 — должна появиться форма лицензии."
echo "Введите ключ: TEST-KEY-1234 и нажмите Activate."
echo ""
echo "Остановить License Server: kill $LS_PID"
echo "Остановить Docker: cd $LS_DIR && docker compose down"
