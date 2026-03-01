# Как полностью запустить и протестировать

## Вариант 1: Автоматический скрипт

```bash
cd /Users/santori/Desktop/TEST_KICK/kick-chat
chmod +x scripts/run-full-test.sh
./scripts/run-full-test.sh
```

Скрипт поднимет Postgres и Redis (Docker), запустит License Server и создаст лицензию `TEST-KEY-1234`. В конце выведет команды для запуска Kick-chat — выполните их в **другом терминале**.

---

## Вариант 2: Вручную по шагам

### 1. Postgres и Redis

```bash
cd /Users/santori/Desktop/TEST_KICK/kick-chat/license-server
docker compose up -d postgres redis
```

Postgres будет на порту **5434**, Redis — **6379**.

### 2. License Server

В том же каталоге или из корня kick-chat:

```bash
cd /Users/santori/Desktop/TEST_KICK/kick-chat/license-server

export PORT=8000
export DATABASE_URL="postgres://postgres:postgres@localhost:5434/licensedb?sslmode=disable"
export REDIS_URL="redis://localhost:6379/0"
export HMAC_SECRET="test-hmac-secret-32bytes-long"
export ADMIN_API_KEY="admin-secret"

go run ./cmd/server
```

Оставьте этот терминал открытым. Сервер: http://localhost:8000

### 3. Создать тестовую лицензию

В **новом** терминале:

```bash
curl -X POST http://localhost:8000/admin/licenses \
  -H "Content-Type: application/json" \
  -H "X-Admin-API-Key: admin-secret" \
  -d '{"license_key":"TEST-KEY-1234","expires_at":"2026-12-31T23:59:59Z","max_activations":3}'
```

Должен вернуться JSON с `"status":"ok"`.

### 4. Запустить Kick-chat (с проверкой лицензии)

В **третьем** терминале:

```bash
cd /Users/santori/Desktop/TEST_KICK/kick-chat

export KICK_CLIENT_ID="ваш_client_id_с_developers.kick.com"
export KICK_CLIENT_SECRET="ваш_client_secret"
export LICENSE_SERVER_URL="http://localhost:8000"
export LICENSE_HMAC_SECRET="test-hmac-secret-32bytes-long"

go run .
```

`LICENSE_HMAC_SECRET` должен совпадать с `HMAC_SECRET` на License Server.

### 5. Проверка в браузере

1. Откройте **http://localhost:8080**
2. Должна открыться страница «License Required» с полем для ключа
3. Введите **TEST-KEY-1234** и нажмите **Activate**
4. После успеха откроется основной дашборд (аккаунты, чат, стрим)

---

## Без лицензии (как раньше)

Если не задавать `LICENSE_SERVER_URL`, дашборд и API работают без проверки лицензии:

```bash
export KICK_CLIENT_ID="..."
export KICK_CLIENT_SECRET="..."
go run .
```

---

## Проверка накрутки зрителей

1. Запусти лицензию и Kick Chat по шагам выше (вариант 1 или 2).
2. В браузере открой **http://localhost:8080**, активируй ключ **TEST-KEY-1234**.
3. Перейди на вкладку **«Накрутка зрителей»**.
4. Укажи **канал** (slug, например `mlaffonxd`) и **число зрителей** (1–5000, для теста хватит 5–20).
5. Нажми **Старт**. В блоке «Онлайн» появятся счётчики: подключено, пинги и т.д.
6. Остановка — кнопка **Стоп**.

Локально Kick Chat собирается **без** `-tags release`. Вьюербот ищет в таком порядке: бинарник **viewerbot** (или viewerbot.exe) рядом с kick-chat → затем **kick.py** (если есть) → иначе Go-реализация. Для накрутки через твой Python-код либо положи рядом с kick-chat собранный `viewerbot` (из `test_view/kick-viewbot/build-viewerbot.sh`), либо запускай из папки kick-chat, где есть `test_view/kick-viewbot/kick.py`.

---

## Остановка

- License Server: `Ctrl+C` в терминале, где он запущен
- Docker: `cd license-server && docker compose down`
