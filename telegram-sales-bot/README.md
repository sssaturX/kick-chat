# Telegram sales bot (SaturX)

Telegram bot for **Crypto Pay** checkout, **automatic license keys** via your existing **license-server** admin API, **“My license”** with days left, and **renewal reminders**.

## What it does

1. **`/start`** — sends an optional promo photo + HTML text (features, two plans) and inline buttons:
   - **Standard — $29/mo** (charged as **USDT** via Crypto Pay; amount configurable)
   - **Pro — $129/yr** (same; copy mentions AI bots in a future update)
   - **My license** — shows key, expiry (UTC), days remaining
2. **Payment** — creates a **Crypto Pay** invoice; user pays in Telegram. The bot **polls** invoice status (no HTTPS webhook required).
3. **After payment** — calls `POST /admin/licenses` (new user) or `POST /admin/activate` (renewal) on your license server, then DM’s the key.
4. **Reminders** — once per day at `REMINDER_HOUR_UTC`, users in the **last 7 days** of a period get up to three nudges (7-day window, 3-day window, last day).
5. **`/support`** — manager contact [@ssaturx](https://t.me/ssaturx).

## Requirements

- Running **[license-server](../license-server/)** with `ADMIN_API_KEY`
- Telegram bot token (**@BotFather**)
- **[@CryptoBot](https://t.me/CryptoBot) → Crypto Pay → Create App** → API token ([Crypto Pay API](https://help.crypt.bot/crypto-pay-api))

> **USDT vs USD:** Crypto Pay invoices use **crypto** (here **USDT**). Amounts are configured in USDT strings (`29` monthly Standard, `129` yearly Pro); exact fiat value moves with rates. Period lengths: `SUBSCRIPTION_PERIOD_DAYS` (Standard) and `SUBSCRIPTION_PERIOD_DAYS_PRO` (Pro, default 365).

## Setup (quick)

```bash
cd telegram-sales-bot
cp .env.example .env
# edit .env

go mod tidy
go run ./cmd/bot
```

---

## Как запустить бота (локально и на VPS)

### Что нужно заранее

1. **Токен бота** — [@BotFather](https://t.me/BotFather) → `/newbot` или существующий бот → API token.  
2. **Crypto Pay** — [@CryptoBot](https://t.me/CryptoBot) → **Crypto Pay** → **Create App** → скопировать **API Token** (для тестов можно включить testnet в `.env`).  
3. **License-server** уже поднят и отвечает (хотя бы `GET /health`), известны **`LICENSE_SERVER_URL`** и **`ADMIN_API_KEY`** (тот же кладёшь в `LICENSE_ADMIN_API_KEY` у бота).  
4. **Свой Telegram user id** для админ-команд — [@userinfobot](https://t.me/userinfobot) или из клиента → в `TELEGRAM_ADMIN_IDS`.

---

### Локальный запуск (Windows / Linux / macOS)

1. Установи **[Go 1.22+](https://go.dev/dl/)** и проверь: `go version`.  
2. Открой папку репозитория:  
   `kick-chat/telegram-sales-bot`  
3. Скопируй конфиг:  
   `cp .env.example .env`  
   (на Windows: `copy .env.example .env`)  
4. Заполни **`.env`** минимум:
   - `TELEGRAM_BOT_TOKEN`
   - `TELEGRAM_ADMIN_IDS` (например `941135938`)
   - `CRYPTOPAY_API_TOKEN`
   - `LICENSE_SERVER_URL` — если license-server на этой же машине: `http://127.0.0.1:8000` (порт как у твоего сервера)
   - `LICENSE_ADMIN_API_KEY` — как `ADMIN_API_KEY` на license-server  
5. Убедись, что **license-server запущен** (Docker или `go run`), если используешь `127.0.0.1`.  
6. В терминале из папки `telegram-sales-bot`:
   ```bash
   go mod tidy
   go run ./cmd/bot
   ```
7. В логах должно быть что-то вроде `Authorized as @YourBotName`. В Telegram напиши боту `/start`.  
8. Остановка: **Ctrl+C**.

База бота по умолчанию: файл **`telegram-sales-bot.db`** в текущей папке (меняется через `BOT_SQLITE_PATH`).

---

### Запуск на VPS (Linux)

Бот ходит **наружу по HTTPS** (Telegram, Crypto Pay, твой license-server). **Входящий порт для бота не нужен** — используется long polling.

#### Вариант A: собрать на сервере

1. Поставь Go **или** используй Docker (ниже отдельно не расписан — при желании оберни бинарь в контейнер).  
2. Склонируй репозиторий (или скопируй только каталог `telegram-sales-bot`).  
3. На сервере:
   ```bash
   cd /path/to/kick-chat/telegram-sales-bot
   cp .env.example .env
   nano .env   # заполни переменные
   chmod 600 .env
   go mod tidy
   go build -o tg-sales-bot ./cmd/bot
   ./tg-sales-bot
   ```
4. Если **license-server на том же VPS** и слушает `8000` на localhost:
   - `LICENSE_SERVER_URL=http://127.0.0.1:8000`  
   Если license только по домену с HTTPS:
   - `LICENSE_SERVER_URL=https://license.твой-домен.com`  

#### Вариант B: собрать у себя и залить бинарь

На своей машине (с Go), для типичного **Linux amd64** сервера:

```bash
cd telegram-sales-bot
go mod tidy
set GOOS=linux
set GOARCH=amd64
go build -o tg-sales-bot ./cmd/bot
```

На Linux/macOS вместо `set` используй:  
`GOOS=linux GOARCH=amd64 go build -o tg-sales-bot ./cmd/bot`

Затем скопируй на VPS **`tg-sales-bot`**, **`.env`** (и при необходимости создай каталог для SQLite). Запуск:

```bash
chmod +x tg-sales-bot
./tg-sales-bot
```

#### systemd (чтобы бот жил после выхода из SSH)

Создай файл `/etc/systemd/system/tg-sales-bot.service` (пути подставь свои):

```ini
[Unit]
Description=SaturX Telegram sales bot
After=network-online.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/opt/tg-sales-bot
EnvironmentFile=/opt/tg-sales-bot/.env
ExecStart=/opt/tg-sales-bot/tg-sales-bot
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

**Замечание:** в unit удобнее не полагаться на `godotenv` из текущей папки — в коде уже есть `godotenv.Load()`; либо оставь `.env` в `WorkingDirectory`, либо пропиши переменные через `Environment=` / `EnvironmentFile`. Если используешь только `EnvironmentFile`, убедись, что формат совместим (строки `KEY=value`).

Команды:

```bash
sudo systemctl daemon-reload
sudo systemctl enable tg-sales-bot
sudo systemctl start tg-sales-bot
sudo systemctl status tg-sales-bot
journalctl -u tg-sales-bot -f
```

#### Фаервол

Нужен **исходящий** доступ к `api.telegram.org`, `pay.crypt.bot` (или testnet), и к хосту **license-server**. Входящие правила для бота открывать не обязательно.

---

### Частые проблемы

| Симптом | Что проверить |
|--------|----------------|
| `TLS handshake` / `tls: first record` / нет связи с Telegram | Исходящий **HTTPS** до `api.telegram.org` блокируется или подменяется. Проверь `curl https://api.telegram.org` с VPS, при необходимости другой регион/VPN. |
| `timeout awaiting response headers` на **getUpdates** | Нормальный long poll ждёт до **60 с** без ответа; HTTP-клиент не должен обрывать ожидание заголовков раньше. В коде для Telegram-клиента **`ResponseHeaderTimeout` отключён**, общий таймаут запроса **~125 с**. |
| Бот не отвечает | Токен в `.env`, бот запущен, нет второго процесса с тем же токеном |
| Ошибка при оплате / инвойс | `CRYPTOPAY_API_TOKEN`, testnet/mainnet, баланс/лимиты в Crypto Pay |
| Лицензия не выдаётся | `LICENSE_SERVER_URL` с VPS доходит до сервера (`curl`), `LICENSE_ADMIN_API_KEY` совпадает с `ADMIN_API_KEY` |
| Админ-команды не работают | В `.env` ровно твой числовой id в `TELEGRAM_ADMIN_IDS`, перезапуск после правки `.env` |

## Admin commands

Set `TELEGRAM_ADMIN_IDS` to your numeric Telegram user id (comma-separated for several admins). Then:

- `/ahelp` or `/admin` — list commands  
- `/admin stats` — how many subscriptions + pending invoices  
- `/admin user <telegram_user_id>` — key, tier, expiry for that buyer  
- `/admin pending` — recent unpaid Crypto Pay invoices tracked by the bot  
- `/admin create <telegram_user_id> <standard|pro>` — create license on license server + bot DB; tries to DM the user  
- `/admin createkey <standard|pro>` — create license on server only (no bot row; hand off key yourself)  
- `/admin droplocal <telegram_user_id>` — remove bot DB row only (license server unchanged)  
- `/admin revoke <LICENSE-KEY>` — `POST /admin/revoke` on license server + drop bot row if this key is mapped  

Non-admins who send `/admin` get the same hint as for unknown commands (no “access denied” wording).

## Environment

| Variable | Meaning |
|----------|---------|
| `TELEGRAM_BOT_TOKEN` | From @BotFather |
| `TELEGRAM_ADMIN_IDS` | e.g. `941135938` or `111,222` |
| `CRYPTOPAY_API_TOKEN` | From Crypto Pay app |
| `CRYPTOPAY_TESTNET` | `true` for testnet API |
| `LICENSE_SERVER_URL` | e.g. `https://license.example.com` |
| `LICENSE_ADMIN_API_KEY` | Same as license server `ADMIN_API_KEY` |
| `WELCOME_PHOTO_URL` | Optional HTTPS image for `/start` (overrides embedded logo) |
| `WELCOME_PHOTO_PATH` | Optional local image if you skip the embedded `internal/bot/welcome_photo.jpg` |
| `SOFTWARE_DOWNLOAD_URL` | Optional `https://…` download portal — **inline button** after payment / **My license** / extend (not on `/start`) |
| `SOFTWARE_DOWNLOAD_LINK_TEXT` | Optional button label (default `Download SaturX`) |
| `PRICE_STANDARD_USDT` / `PRICE_PRO_USDT` | Invoice amounts (e.g. `29` / `129`) |
| `SUBSCRIPTION_PERIOD_DAYS` | Standard license length after payment (default `30`) |
| `SUBSCRIPTION_PERIOD_DAYS_PRO` | Pro license length (default `365` = 1 year) |
| `INVOICE_POLL_SECONDS` / `INVOICE_POLL_TIMEOUT_MIN` | How often the bot checks Crypto Pay and how long it keeps polling (default **1440** min ≈ 24h, matching invoice `expires_in`) |
| `BOT_SQLITE_PATH` | SQLite path (default `telegram-sales-bot.db`) |

## Security

- Keep **`LICENSE_ADMIN_API_KEY`** and **`CRYPTOPAY_API_TOKEN`** secret.
- The bot is equivalent to an admin: anyone with it can create/extend keys. Run only on a trusted VPS.

## Direct crypto (no Crypto Pay)

This build is wired to **Crypto Pay** only. For raw on-chain payments you’d add address + manual confirmation or a chain indexer; that’s not implemented here.

## Shipping the desktop app to buyers

- **Prefer one file:** pack the build as **`.zip`** or ship a **single installer** (`.exe` / `.msi`). Telegram is awkward for folders; bots can only send files up to ~**50 MB** as documents.
- **Gated portal (recommended):** on **license-server** set **`DOWNLOAD_FILE_PATH`** / **`DOWNLOAD_FILE_NAME`** — public page **`GET /download`** checks the key via **`POST /validate`**, then issues a **one-time** download (`Redis`). Point **`SOFTWARE_DOWNLOAD_URL`** at that page (e.g. `https://software.saturx.store/download`). See **license-server README**.
- **Direct zip URL:** only if you accept that anyone with the link can download; set **`SOFTWARE_DOWNLOAD_URL`** to the raw file.
- **Sending the file from the bot** is possible only for small builds; every version update requires a new upload. A stable URL is easier to maintain.

## Bot profile in Telegram (description + logo)

Copy-paste texts for **@BotFather** (`/setdescription`, `/setabouttext`) and notes on **local logo** (no image host): see **[BOT_PROFILE.md](./BOT_PROFILE.md)**.

## Customize copy

Edit `internal/bot/bot.go` → `welcomeCaption()` for product text and button labels.

## License key format

Keys look like `XXXX-XXXX-XXXX-XXXX` (alphanumeric, no ambiguous `0`/`O`/`I`). Users enter them in SaturX as today.
