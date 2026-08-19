# SaturX — implemented features

What the project currently does.

---

## 1. OAuth and auth

- Kick **OAuth 2.0 (PKCE)** sign-in.
- Scopes: **channel:read**, **chat:write**.
- First launch without accounts shows the Kick authorize flow; after redirect to `http://localhost:<port>/oauth/callback` the token is stored.
- Config via `KICK_CLIENT_ID`, `KICK_CLIENT_SECRET`, `KICK_REDIRECT_URI` (or `.env`).

---

## 2. Multiple accounts

- **Storage:** `.kick_accounts.json` (id, name, token, proxy).
- **Import:** if the list is empty, one account can be imported from `KICK_ACCESS_TOKEN` or the legacy `.kick_access_token` file.
- **Switch:** dashboard current account is persisted (`last_used`).
- **Add:** OAuth add; the new token is appended and becomes current.
- **List:** dashboard and `/api/accounts`.

---

## 3. SOCKS5 proxies

- Each account can have its own proxy: **host:port:user:pass**.
- Optional **`.kick_proxies`**: one line per account (line 1 → account 1, …); empty `proxy` fields are filled on startup.
- Kick API calls use the account proxy; if none is set, the app uses a direct connection.
- **Fallback:** on proxy connect/timeout errors the request is retried without a proxy.
- **HTTP client cache:** one client per proxy string (TCP/Keep-Alive reuse).

---

## 4. 401 / invalid tokens

- Startup does not crash if the current account gets 401 on channel lookup.
- Switching accounts re-resolves the channel / broadcaster id.
- Chat send retries channel resolution; on 401 the UI/logs tell you to switch or re-add the account.

---

## 5. Web dashboard

- Starts with the app; default port **8080** (`DASHBOARD_PORT`).
- Single page, HTML/CSS/JS embedded via `go:embed` from `static/`.

Layout:

- **Header:** SaturX logo, channel slug, stream status, theme, add account, shutdown.
- **Chat:** live Kick chat.
- **Stream:** Kick player embed (`https://player.kick.com/<channel_slug>`).
- **Accounts:** linked accounts and runner/ready status.
- **Bottom bar:** account select, message input (500 chars), send.

Emote bar: official Kick emotes plus local GIFs under `/emotes/`. Clicking an emote sends it from the selected account.

Shutdown: `POST /api/shutdown` then the process exits.

---

## 6. Backend API (selected)

- `GET /api/accounts` — accounts (id, stable_id, name, current, proxy)
- `GET /api/status` — ready flag per account
- `POST /api/send` — `{ "account_id": N, "message": "text" }`
- `GET /api/channel` — `{ "slug": "..." }`
- `POST /api/shutdown` — stop the app
- `GET /emotes/<name>.gif` — emote files from `static/emotes/`

---

## 7. Config files

| File / variable | Purpose |
|-----------------|---------|
| `.env` | `KICK_CLIENT_ID`, `KICK_CLIENT_SECRET`, `KICK_REDIRECT_URI`, `CHANNEL_SLUG`, `DASHBOARD_PORT` |
| `.kick_accounts.json` | Accounts: id, name, token, proxy; last_used; display_order |
| `.kick_proxies` | Proxy lines (`host:port:user:pass`) |
| `static/emotes/*.gif` | Local emote icons |
| `messages.txt` | Dashboard presets |
| `auto-sender.txt` | Auto-sender lines |

---

## 8. Stack

- **Language:** Go 1.24+.
- **SDK:** [kick-go-sdk](https://github.com/henrikah/kick-go-sdk) v2.
- **Also:** godotenv, golang.org/x/net (SOCKS5 with user/password).
- Dashboard: one HTML file, no separate frontend framework.

---

## 9. Build and run

```bash
go mod tidy
go build -o saturx .
./saturx
# or
go run .
```

Dashboard: **http://localhost:8080** (or `DASHBOARD_PORT`).
