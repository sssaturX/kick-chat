# Как всё запустить (License Server + Kick Chat)

## Где задаётся домен / URL сервера лицензий

Запросы к серверу лицензий идут **из приложения Kick Chat** (с ПК пользователя). **Пользователю не нужно прописывать URL и HMAC в .env** — ты задаёшь их при сборке бинарника (см. раздел 2). В собранном приложении они уже «зашиты».


| Где                         | Кто задаёт                 | Что                                                                                         |
| --------------------------- | -------------------------- | ------------------------------------------------------------------------------------------- |
| **Kick Chat (бинарник)**    | Ты при сборке (`-ldflags`) | URL сервера лицензий и HMAC-секрет для проверки подписи. Юзер их не видит и не редактирует. |
| **На VPS (License Server)** | Ты в .env / окружении      | `PORT`, `HMAC_SECRET`, `ADMIN_API_KEY` и т.д. Домен и HTTPS — в nginx/caddy на VPS.         |


---

## Схема

```
[ПК пользователя]                          [Твой VPS]
  Kick Chat  ─── URL/HMAC из бинарника ───►  License Server (Postgres, Redis)
       │              (activate, refresh)         │
       │                                          │
       └── дашборд localhost:8080                 └── API :8000 (или за nginx)
```

---

## 1. License Server (на VPS или локально)

### 1.1 Локально (для разработки / теста)

**Postgres и Redis через Docker:**

```bash
cd kick-chat/license-server
cp .env.example .env
# Отредактируй .env (см. ниже)
docker compose up -d postgres redis
```

**Заполни `license-server/.env`:**

```env
PORT=8000
DATABASE_URL=postgres://postgres:postgres@localhost:5434/licensedb?sslmode=disable
REDIS_URL=redis://localhost:6379/0
HMAC_SECRET=твой-hmac-секрет-32-байта
ADMIN_API_KEY=твой-админ-ключ
RATE_LIMIT_RPS=100
```

Для Docker Postgres хост с хоста — `localhost:5434` (порт 5434 с docker-compose).

**Запуск самого приложения (без Docker образа):**

```bash
cd kick-chat/license-server
go run ./cmd/server
```

Сервер будет на **[http://localhost:8000](http://localhost:8000)**.

---

### 1.2 На VPS (продакшен)

На VPS поднимаешь Postgres и Redis (Docker или системные пакеты), затем приложение.

**Пример .env на VPS:**

```env
PORT=8000
DATABASE_URL=postgres://user:password@localhost:5432/licensedb?sslmode=disable
REDIS_URL=redis://localhost:6379/0
HMAC_SECRET=твой-секрет-из-openssl-rand
ADMIN_API_KEY=твой-админ-ключ
RATE_LIMIT_RPS=100
```

Домен к этому серверу настраиваешь в **веб-сервере** (nginx/caddy): проксирование на `http://127.0.0.1:8000`. Например:

- Домен: `license.mysite.com`
- SSL: Let's Encrypt
- Проксирование `https://license.mysite.com` → `http://127.0.0.1:8000`

Тогда снаружи запросы идут на `https://license.mysite.com` — этот URL ты подставляешь при сборке Kick Chat в `-ldflags` (см. раздел 2).

**Полный подъём через Docker на VPS (все сервисы в контейнерах):**

```bash
cd kick-chat/license-server
# .env с реальными HMAC_SECRET, ADMIN_API_KEY, при необходимости DATABASE_URL для внешней БД
docker compose up -d
```

Приложение будет слушать порт 8000 внутри контейнера; снаружи порт 8000 проброшен. Домен и HTTPS — снова в nginx/caddy перед контейнером.

---

## 2. Kick Chat (у пользователя)

Запускается на ПК пользователя. URL сервера лицензий и HMAC **не задаются пользователем** — они задаются при сборке бинарника (см. ниже). В .env у пользователя только OAuth и опционально порт.

**Сборка бинарника для распространения (делаешь ты):**

- Подставь свой URL и HMAC-секрет (как на license server) — пользователь не пишет их в .env.
- Собери с тегом **`release`**: накрутка зрителей идёт через **скомпилированный** вьюербот (бинарник из твоего Python-кода), без установки Python и без отдачи .py — код не просмотреть.

**1) Сборка Kick Chat:**

```bash
cd kick-chat
go build -tags release -ldflags "\
  -X main.defaultLicenseServerURL=https://license.mysite.com \
  -X main.defaultLicenseHMACSecret=ТОТ_ЖЕ_HMAC_SECRET_ЧТО_НА_LICENSE_SERVER" \
  -o kick-chat .
```

**2) Сборка вьюербота (один бинарник из kick.py, без исходников):**

```bash
cd kick-chat/test_view/kick-viewbot
./build-viewerbot.sh
```

Скрипт ставит зависимости и PyInstaller, собирает один файл **`dist/viewerbot`** (на Windows — `viewerbot.exe`). Исходный код в бинарник не попадает, юзер Python не ставит.

**Всё одной командой (Go + Python, флаги из .env):**

- **macOS/Linux:** `cd kick-chat` → `./scripts/build-release.sh`
- **Сборка под Windows с Mac:** `./scripts/build-release.sh windows` — соберётся только **kick-chat.exe** (Go кросс-компилируется). **viewerbot.exe** нужно собрать на самой Windows: `.\scripts\build-release.ps1` или только `test_view\kick-viewbot\build-viewerbot.ps1`, затем положить в `release/` рядом с kick-chat.exe.
- **Windows (PowerShell):** `cd kick-chat` → `.\scripts\build-release.ps1`

Скрипты читают `LICENSE_SERVER_URL` и `LICENSE_HMAC_SECRET` из `.env`, собирают kick-chat и viewerbot, кладут оба в папку **`release/`** (на Windows: kick-chat.exe и viewerbot.exe).

Секрет в `-ldflags` не коммить в репозиторий (используй скрипт/CI или переменные окружения при сборке).

**Что отдавать пользователю:** два файла в одной папке — **`kick-chat`** и **`viewerbot`** (или `viewerbot.exe`), плюс инструкция про .env с `KICK_CLIENT_ID` и `KICK_CLIENT_SECRET`. Никаких .py и папок с кодом; накрутка работает через бинарник viewerbot, код не просмотреть, Python ставить не нужно.

**У пользователя в `.env` только:**

```env
KICK_CLIENT_ID=...
KICK_CLIENT_SECRET=...
# DASHBOARD_PORT=8080  — опционально
```

`LICENSE_SERVER_URL` и `LICENSE_HMAC_SECRET` пользователь не прописывает — они уже в бинарнике.

**Запуск у пользователя:**

```bash
./kick-chat
```

- Дашборд: **[http://localhost:8080](http://localhost:8080)**
- При первом открытии — экран «Введите ключ лицензии». После активации — полный доступ (дашборд + консоль).

**Для локальной разработки/теста** можно по-прежнему задать `LICENSE_SERVER_URL` и `LICENSE_HMAC_SECRET` в .env или окружении — тогда они переопределяют значения из ldflags (если ты собрал с пустыми ldflags).

---

## 3. Админ: создание лицензий

На сервере лицензий (локально или на VPS):

```bash
curl -X POST https://license.mysite.com/admin/licenses \
  -H "Content-Type: application/json" \
  -H "X-Admin-API-Key: твой-ADMIN_API_KEY" \
  -d '{"license_key":"XXXX-YYYY-ZZZZ","expires_at":"2026-12-31T23:59:59Z","max_activations":3}'
```

Ключ `XXXX-YYYY-ZZZZ` выдаёшь пользователю; он вводит его в форме на [http://localhost:8080](http://localhost:8080).

Админ-дашборд (логи, создание/отзыв ключей): открой в браузере **[https://license.mysite.com](https://license.mysite.com)** (или [http://IP:8000](http://IP:8000)), введи Admin API Key и используй формы/кнопки.

---

## 4. Краткая таблица


| Компонент          | Где запускается    | Что задаёт «домен» / URL                                                          |
| ------------------ | ------------------ | --------------------------------------------------------------------------------- |
| **License Server** | VPS (или локально) | На VPS: nginx/caddy дают домен и HTTPS. В приложении только `PORT=8000`.          |
| **Kick Chat**      | ПК пользователя    | URL и HMAC при сборке (`-ldflags`). Накрутка — бинарник `viewerbot` (PyInstaller из kick.py), без .py и без Python. |


Итого: при сборке задаёшь URL и HMAC; пользователю отдаёшь **kick-chat** + **viewerbot** в одной папке, в .env только `KICK_CLIENT_ID` и `KICK_CLIENT_SECRET`. На VPS домен — в nginx/caddy перед `PORT` (8000).