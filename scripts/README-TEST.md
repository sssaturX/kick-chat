# Full local test run

## Option 1: Script

```bash
cd /path/to/kick-chat
chmod +x scripts/run-full-test.sh
./scripts/run-full-test.sh
```

The script starts Postgres and Redis (Docker), runs the License Server, and creates license `TEST-KEY-1234`. It prints commands to start SaturX in **another terminal**.

---

## Option 2: Manual steps

### 1. Postgres and Redis

```bash
cd /path/to/kick-chat/license-server
docker compose up -d postgres redis
```

Postgres is on **5434**, Redis on **6379**.

### 2. License Server

```bash
cd /path/to/kick-chat/license-server

export PORT=8000
export DATABASE_URL="postgres://postgres:postgres@localhost:5434/licensedb?sslmode=disable"
export REDIS_URL="redis://localhost:6379/0"
export HMAC_SECRET="test-hmac-secret-32bytes-long"
export ADMIN_API_KEY="admin-secret"

go run ./cmd/server
```

Leave this terminal open. Server: http://localhost:8000

### 3. Create a test license

In a **new** terminal:

```bash
curl -X POST http://localhost:8000/admin/licenses \
  -H "Content-Type: application/json" \
  -H "X-Admin-API-Key: admin-secret" \
  -d '{"license_key":"TEST-KEY-1234","expires_at":"2026-12-31T23:59:59Z","max_activations":3}'
```

Expect JSON with `"status":"ok"`.

### 4. Run SaturX (with license checks)

In a **third** terminal:

```bash
cd /path/to/kick-chat

export KICK_CLIENT_ID="your_client_id_from_developers.kick.com"
export KICK_CLIENT_SECRET="your_client_secret"
export LICENSE_SERVER_URL="http://localhost:8000"
export LICENSE_HMAC_SECRET="test-hmac-secret-32bytes-long"

go run .
```

`LICENSE_HMAC_SECRET` must match `HMAC_SECRET` on the License Server.

### 5. Browser check

1. Open **http://localhost:8080**
2. You should see **License Required**
3. Enter **TEST-KEY-1234** and **Activate**
4. The main dashboard opens (accounts, chat, stream)

---

## Without a license (dev)

If `LICENSE_SERVER_URL` is unset, the dashboard and API skip license checks:

```bash
export KICK_CLIENT_ID="..."
export KICK_CLIENT_SECRET="..."
go run .
```

Or set `SKIP_LICENSE=1`.

---

## Viewer boost check

1. Start license server and SaturX as above.
2. Open **http://localhost:8080**, activate **TEST-KEY-1234**.
3. Open the **Viewer boost** tab.
4. Set the channel slug (`kick.com/<your_channel>`) and viewer target (1–5000; 5–20 is enough for a test).
5. Click **Start**. Online counters should move.
6. **Stop** to end the run.

Local SaturX is built **without** `-tags release`. Viewerbot lookup order: `viewerbot` / `viewerbot.exe` next to the app → `kick.py` if present → Go implementation.

---

## Stop

- License Server: `Ctrl+C` in its terminal
- Docker: `cd license-server && docker compose down`
