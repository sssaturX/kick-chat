#!/usr/bin/env bash
# ╨Я╨╛╨╗╨╜╤Л╨╣ ╨╖╨░╨┐╤Г╤Б╨║ ╨┤╨╗╤П ╤В╨╡╤Б╤В╨░: Postgres + Redis + License Server, ╤Б╨╛╨╖╨┤╨░╨╜╨╕╨╡ ╤В╨╡╤Б╤В╨╛╨▓╨╛╨╣ ╨╗╨╕╤Ж╨╡╨╜╨╖╨╕╨╕.
# Kick-chat ╨╜╤Г╨╢╨╜╨╛ ╨╖╨░╨┐╤Г╤Б╤В╨╕╤В╤М ╨╛╤В╨┤╨╡╨╗╤М╨╜╨╛ (╤Б╨╝. ╨║╨╛╨╜╨╡╤Ж ╤Б╨║╤А╨╕╨┐╤В╨░).

set -e
cd "$(dirname "$0")/.."
ROOT="$(pwd)"
LS_DIR="$ROOT/license-server"

echo "=== 1. ╨Я╨╛╨┤╨╜╨╕╨╝╨░╨╡╨╝ Postgres ╨╕ Redis (Docker) ==="
docker compose -f "$LS_DIR/docker-compose.yml" up -d postgres redis

echo "=== 2. ╨Ц╨┤╤С╨╝ ╨│╨╛╤В╨╛╨▓╨╜╨╛╤Б╤В╨╕ Postgres ==="
for i in 1 2 3 4 5 6 7 8 9 10; do
  if docker compose -f "$LS_DIR/docker-compose.yml" exec -T postgres pg_isready -U postgres 2>/dev/null; then
    break
  fi
  echo "  ╨╢╨┤╤С╨╝ Postgres... ($i/10)"
  sleep 2
done

echo "=== 3. ╨Ч╨░╨┐╤Г╤Б╨║╨░╨╡╨╝ License Server (╤Д╨╛╨╜╨╛╨▓╨╛) ==="
export PORT=8000
export DATABASE_URL="postgres://postgres:postgres@localhost:5434/licensedb?sslmode=disable"
export REDIS_URL="redis://localhost:6379/0"
export HMAC_SECRET="${HMAC_SECRET:-test-hmac-secret-32bytes-long}"
export ADMIN_API_KEY="${ADMIN_API_KEY:-santori}"

# ╨Ч╨░╨┐╤Г╤Б╨║ ╨▓ ╤Д╨╛╨╜╨╡; ╨╗╨╛╨│╨╕ ╨▓ ╤Д╨░╨╣╨╗
( cd "$LS_DIR" && go run ./cmd/server >> "$ROOT/license-server.log" 2>&1 ) &
LS_PID=$!
echo "  License Server PID: $LS_PID"

echo "=== 4. ╨Ц╨┤╤С╨╝ /health ==="
SERVER_OK=
for i in 1 2 3 4 5 6 7 8 9 10; do
  if curl -s -o /dev/null -w "%{http_code}" http://localhost:8000/health 2>/dev/null | grep -q 200; then
    echo "  License Server ╨│╨╛╤В╨╛╨▓."
    SERVER_OK=1
    break
  fi
  echo "  ╨╢╨┤╤С╨╝ ╤Б╨╡╤А╨▓╨╡╤А... ($i/10)"
  sleep 2
done
if [[ -z "$SERVER_OK" ]]; then
  echo "  License Server ╨╜╨╡ ╨╛╤В╨▓╨╡╤В╨╕╨╗. ╨Я╨╛╤Б╨╗╨╡╨┤╨╜╨╕╨╡ ╤Б╤В╤А╨╛╨║╨╕ ╨╗╨╛╨│╨░:"
  tail -20 "$ROOT/license-server.log" 2>/dev/null || echo "  (╨╗╨╛╨│ ╨┐╤Г╤Б╤В ╨╕╨╗╨╕ ╨╜╨╡ ╤Б╨╛╨╖╨┤╨░╨╜)"
  exit 1
fi

echo "=== 5. ╨б╨╛╨╖╨┤╨░╤С╨╝ ╤В╨╡╤Б╤В╨╛╨▓╤Г╤О ╨╗╨╕╤Ж╨╡╨╜╨╖╨╕╤О ==="
# expires_at ╤З╨╡╤А╨╡╨╖ 30 ╨┤╨╜╨╡╨╣ (macOS: -v+30d, Linux: -d "+30 days")
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
echo "=== ╨У╨╛╤В╨╛╨▓╨╛. ╨в╨╡╤Б╤В╨╛╨▓╨░╤П ╨╗╨╕╤Ж╨╡╨╜╨╖╨╕╤П: TEST-KEY-1234 ==="
echo ""
echo "╨Ч╨░╨┐╤Г╤Б╤В╨╕╤В╨╡ Kick-chat ╨▓ ╨┤╤А╤Г╨│╨╛╨╝ ╤В╨╡╤А╨╝╨╕╨╜╨░╨╗╨╡:"
echo ""
echo "  cd $ROOT"
echo "  export KICK_CLIENT_ID=\"╨▓╨░╤И_client_id\""
echo "  export KICK_CLIENT_SECRET=\"╨▓╨░╤И_client_secret\""
echo "  export LICENSE_SERVER_URL=\"http://localhost:8000\""
echo "  export LICENSE_HMAC_SECRET=\"$HMAC_SECRET\""
echo "  go run ."
echo ""
echo "╨Ю╤В╨║╤А╨╛╨╣╤В╨╡ http://localhost:8080 тАФ ╨┤╨╛╨╗╨╢╨╜╨░ ╨┐╨╛╤П╨▓╨╕╤В╤М╤Б╤П ╤Д╨╛╤А╨╝╨░ ╨╗╨╕╤Ж╨╡╨╜╨╖╨╕╨╕."
echo "╨Т╨▓╨╡╨┤╨╕╤В╨╡ ╨║╨╗╤О╤З: TEST-KEY-1234 ╨╕ ╨╜╨░╨╢╨╝╨╕╤В╨╡ Activate."
echo ""
echo "╨Ю╤Б╤В╨░╨╜╨╛╨▓╨╕╤В╤М License Server: kill $LS_PID"
echo "╨Ю╤Б╤В╨░╨╜╨╛╨▓╨╕╤В╤М Docker: cd $LS_DIR && docker compose down"
