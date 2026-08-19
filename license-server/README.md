# License Server

Production-ready license server for the desktop app (Go 1.22+, Gin, PostgreSQL, GORM, Redis).

---

## How it works

### What you host

- **You** run **one** License Server on the internet (VPS, cloud, etc.) with the app, PostgreSQL, and Redis. Users do not open it by hand — **SaturX** calls it over HTTPS.
- **The user** installs the desktop app. On start (and periodically) it calls **your** License Server (for example `https://license.example.com`).

You only need to host the License Server. SaturX and other services can live elsewhere.

### Checks on the user machine

1. **First launch (activate)**  
   The user pastes a key in the app. The app sends **POST /activate** with `license_key` and optional `hwid`.  
   The server checks the key exists, status is `active`, it is not expired, and activation limits / HWID match.  
   The app stores the key, `expires_at`, and HMAC signature locally.

2. **Later launches**  
   Local expiry can allow a start, then **POST /validate** runs in the background. Status is `active` | `expired` | `revoked` | `invalid`. Expired/revoked keys lock the app.

3. **Offline**  
   A short grace window based on the last successful check is reasonable; then require a successful **POST /validate**.

4. **Admin**  
   Create keys (`POST /admin/licenses`), extend (`POST /admin/activate`), revoke (`POST /admin/revoke`).

---

## Requirements

- Go 1.22+
- PostgreSQL
- Redis

## Configuration

Copy `.env.example` to `.env` and set:

- `DATABASE_URL` — Postgres
- `REDIS_URL` — Redis (default `redis://localhost:6379/0`)
- `HMAC_SECRET` — response signing secret (at least 32 characters)
- `ADMIN_API_KEY` — admin API and `/admin/login`
- optional `ADMIN_SESSION_TTL` (e.g. `24h`), `ADMIN_COOKIE_SECURE` (`true` behind HTTPS)

## Run locally

```bash
# Postgres and Redis must be running
go run ./cmd/server
```

Port comes from `PORT` (default 8000). `/` redirects to `/admin/dashboard`. Manual API test page: `/dev`.

## Admin UI

- `GET /admin/login` — Admin API Key form
- HttpOnly cookie `admin_session` in Redis (`ADMIN_SESSION_TTL`, default 24h)
- `GET /admin/dashboard` — list, create, extend, revoke, delete
- `POST /admin/api/session/login` — JSON `{"api_key":"..."}`
- `POST /admin/api/session/logout`
- `GET /admin/api/licenses`
- `POST /admin/delete` — JSON `{"license_key":"..."}`

All `/admin/*` except login accept a **session cookie** or **`X-Admin-API-Key`**. Query-string keys are not supported. Set **`ADMIN_COOKIE_SECURE=true`** in production HTTPS.

## Docker

VPS checklist (DB backup, build, HTTPS, download portal): **[DEPLOY-VPS.md](./DEPLOY-VPS.md)**.

```bash
docker-compose up -d
```

App on 8000, Postgres on host 5434, Redis on 6379.

If http://localhost:8000 does not open, check `docker-compose logs app`. Typical causes: DB connection, missing `HMAC_SECRET` / `ADMIN_API_KEY`. Keep a `.env` next to `docker-compose.yml`.

## Download portal (optional)

If `DOWNLOAD_FILE_PATH` and `DOWNLOAD_FILE_NAME` are set:

- `GET /download` — user enters a license key, gets a one-time file link
- `POST /download/verify` — JSON `{"license_key":"..."}`
- `GET /download/file?token=...` — serves the archive; Redis token is one-time

Key check uses the same logic as **POST /validate** (empty `hwid`). Point `SOFTWARE_DOWNLOAD_URL` at `https://software.example.com/download`. Put files in **`releases/`** next to compose.

## API

- **POST /activate** — activate (`license_key`, `hwid`)
- **POST /validate** — status: active | expired | revoked | invalid
- **POST /admin/revoke** — body: `license_key`
- **POST /admin/activate** — enable / extend (`license_key`, `expires_at`)
- **POST /admin/licenses** — create (`license_key`, `expires_at`, optional `max_activations`)
- **GET /admin/api/licenses** — list
- **POST /admin/delete** — delete key and activations
- **GET /health**

## Create a key (SQL example)

```sql
INSERT INTO licenses (id, license_key, status, expires_at, max_activations)
VALUES (
  gen_random_uuid(),
  'ABCD-1234-EFGH',
  'active',
  NOW() + INTERVAL '30 days',
  1
);
```

`licenses` is migrated on start (GORM AutoMigrate).
