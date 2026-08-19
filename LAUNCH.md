# How to run License Server + SaturX

## Where the license server URL is set

License checks are made **from the SaturX app** on the user’s PC. **Users do not put the URL or HMAC in `.env`** — you embed them at build time (section 2).

**In this repo, set them in the root `.env` (next to `main.go`):**

- `LICENSE_SERVER_URL=https://license.example.com` — no trailing slash
- `LICENSE_HMAC_SECRET=...` — same value as `HMAC_SECRET` on the license server

`scripts/build-release.ps1` / `scripts/build-release.sh` read that `.env` and pass the values into `go build -ldflags`.

| Where | Who sets it | What |
|-------|-------------|------|
| **SaturX binary** | You at build time (`-ldflags`) | License server URL and HMAC. Users never edit these. |
| **VPS (License Server)** | You in `.env` / the environment | `PORT`, `HMAC_SECRET`, `ADMIN_API_KEY`, etc. Domain and HTTPS via nginx/caddy. |

---

## Diagram

```
[User PC]                                   [Your VPS]
  SaturX  ─── URL/HMAC from binary ───►  License Server (Postgres, Redis)
       │              (activate, refresh)         │
       │                                          │
       └── dashboard localhost:8080               └── API :8000 (or behind nginx)
```

---

## 1. License Server (VPS or local)

### 1.1 Local (dev / test)

**Postgres and Redis via Docker:**

```bash
cd license-server
cp .env.example .env
# edit .env
docker compose up -d postgres redis
```

**`license-server/.env` example:**

```env
PORT=8000
DATABASE_URL=postgres://postgres:postgres@localhost:5434/licensedb?sslmode=disable
REDIS_URL=redis://localhost:6379/0
HMAC_SECRET=your-hmac-secret-32-bytes
ADMIN_API_KEY=your-admin-key
RATE_LIMIT_RPS=100
```

Host port for Docker Postgres is `localhost:5434`.

**Run the Go server (not the Docker image):**

```bash
cd license-server
go run ./cmd/server
```

Server: **http://localhost:8000**.

### 1.2 VPS (production)

Run Postgres and Redis, then the app. Put the domain and TLS on nginx/caddy in front of `http://127.0.0.1:8000`.

Full Docker on the VPS:

```bash
cd license-server
docker compose up -d
```

---

## 2. SaturX (on the user’s machine)

Users do **not** set license URL/HMAC. You bake them into the binary.

**Build SaturX:**

```bash
cd kick-chat
go build -tags release -ldflags "\
  -X main.defaultLicenseServerURL=https://license.example.com \
  -X main.defaultLicenseHMACSecret=SAME_HMAC_AS_LICENSE_SERVER" \
  -o saturx .
```

**Build viewerbot (one binary from kick.py, no sources shipped):**

```bash
cd test_view/kick-viewbot
./build-viewerbot.sh
```

**One-shot scripts (Go + Python, flags from `.env`):**

- macOS/Linux: `./scripts/build-release.sh`
- Cross-compile Windows from Mac: `./scripts/build-release.sh windows` (SaturX.exe only; build `viewerbot.exe` on Windows)
- Windows PowerShell: `.\scripts\build-release.ps1`

Admin build with license checks off (do not ship to customers): `.\scripts\build-release-admin.ps1` or `./scripts/build-release-admin.sh`.

**User `.env` only needs:**

```env
KICK_CLIENT_ID=...
KICK_CLIENT_SECRET=...
CHANNEL_SLUG=your_channel
# DASHBOARD_PORT=8080
```

**User start:**

```bash
./saturx
```

Dashboard: **http://localhost:8080**. First visit asks for a license key.

For local testing you can still set `LICENSE_SERVER_URL` and `LICENSE_HMAC_SECRET` in `.env` to override empty ldflags.

---

## 3. Admin: creating licenses

```bash
curl -X POST https://license.example.com/admin/licenses \
  -H "Content-Type: application/json" \
  -H "X-Admin-API-Key: YOUR_ADMIN_API_KEY" \
  -d '{"license_key":"XXXX-YYYY-ZZZZ","expires_at":"2026-12-31T23:59:59Z","max_activations":3}'
```

Web admin: **https://license.example.com** (or `http://IP:8000`) → login with the Admin API Key.

---

## 4. `release/` folder

`.\scripts\build-release.ps1` writes **`release\SaturX\`**:

- `SaturX.exe`
- `viewerbot.exe` (if built)
- `.env.example`
- `USER-GUIDE.md`
- `README.txt`

Zip that folder for users. Do not include `.git`, a working `.env` with secrets, or `.kick_accounts.json`.

Windows icon / file properties: `versioninfo.json` and optional `build\icon.ico`.
