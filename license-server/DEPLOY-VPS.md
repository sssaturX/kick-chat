# Полный передеплой License Server на VPS

Документ описывает **полный цикл**: подготовка сервера, первый запуск, обновление кода и откат. Стек: **Docker Compose** (рекомендуется), **PostgreSQL**, **Redis**, опционально **портал скачивания** (`/download`).

---

## 1. Что должно быть на VPS

- **ОС**: Ubuntu 22.04/24.04 LTS (или аналог).
- **Docker** и **Docker Compose v2** (`docker compose`, не устаревший `docker-compose` как отдельный бинарь — на новых системах уже встроен).
- Открыты **порты**: `22` (SSH), `80`/`443` (если HTTPS на этой же машине). Порт **8000** наружу **не обязателен**, если перед приложением стоит reverse proxy только на `127.0.0.1:8000`.
- **Домен** (A-запись на IP VPS), если нужен HTTPS и ссылки для бота/клиента.

---

## 2. Каталог на сервере (один раз)

Рекомендуемая структура:

```text
/opt/license-server/
├── docker-compose.yml
├── .env                    # секреты, не коммитить
├── releases/               # опционально: zip для портала
│   └── SaturX-release.zip
```

Создание:

```bash
sudo mkdir -p /opt/license-server/releases
sudo chown -R "$USER:$USER" /opt/license-server
cd /opt/license-server
```

Скопируй сюда `docker-compose.yml` и `Dockerfile` из репозитория `license-server/` (или весь каталог `license-server` и работай из него).

### 2.1. Загрузка с компьютера на VPS через SCP / rsync

Ниже — **что именно** нужно на сервере, чтобы `docker compose build` собрал образ, и **как** это залить по SSH.

#### Что обязательно должно оказаться в каталоге деплоя

Dockerfile делает `COPY . .` и собирает `./cmd/server`, поэтому на VPS нужен **полный исходный код** каталога `license-server` из репозитория, как минимум:

| Путь / файлы | Зачем |
|--------------|--------|
| `Dockerfile` | Сборка образа |
| `docker-compose.yml` | Запуск Postgres, Redis, приложения |
| `go.mod`, `go.sum` | Зависимости Go |
| `cmd/server/` | Точка входа и `cmd/server/static/` (embed главной HTML-страницы) |
| `internal/` | Весь код сервиса, в т.ч. `handler/download_portal.html` (embed) |
| Всё, что тянет **go:embed** и импорты | Иначе `go build` упадёт |

Практически проще всего **раз за разом копировать целиком папку `license-server`**, а не собирать список вручную.

**Не обязательно для работы сервера** (можно не копировать, если не хочешь): `README.md`, `DEPLOY-VPS.md`, `.git` — но их наличие не мешает сборке.

**Отдельно** (не из репозитория или поверх):

| Что | Куда на сервере |
|-----|------------------|
| Файл `.env` с прод-секретами | `/opt/license-server/.env` — создай на сервере или залей **отдельной** командой (см. ниже) |
| Zip для портала скачивания | Например `/opt/license-server/releases/SaturX-release.zip` |

**Важно:** не клади **тяжёлый zip** внутрь копируемой папки рядом с `Dockerfile`, если не хочешь, чтобы он попадал в контекст сборки и замедлял `docker build`. Держи архив в `releases/` на сервере и монтируй volume, как в §4.

#### Переменные для примеров

Подставь свои значения:

- `USER@HOST` — пользователь и IP или домен VPS, например `deploy@203.0.113.50` или `deploy@vps.example.com`
- `LOCAL` — путь к папке `license-server` на твоём ПК  
  - Windows: `E:\kick-chat\kick-chat\kick-chat\license-server`  
  - Или из Git Bash / WSL: `/e/kick-chat/kick-chat/kick-chat/license-server`

#### Вариант A — `scp` (рекурсивно вся папка)

На **Windows 10/11** в PowerShell или `cmd` обычно есть OpenSSH (`scp`). С локальной машины:

```bash
scp -r "E:\path\to\license-server" USER@HOST:/opt/
```

На сервере получится `/opt/license-server/` со всем содержимым.

Если каталог **уже есть** и нужно **перезаписать файлы**, удобнее сначала удалить старую копию на сервере или заливать во временную папку и заменить — `scp -r` **дополняет** и не всегда удаляет устаревшие файлы. Для «чистого» обновления смотри **вариант B (rsync)**.

Первый раз создай каталог на сервере и отдай его своему пользователю (подставь имя пользователя на VPS вместо `ubuntu`, если другое):

```bash
ssh USER@HOST sudo mkdir -p /opt/license-server
ssh USER@HOST sudo chown ubuntu:ubuntu /opt/license-server
```

Либо одной строкой из **bash** на Linux/macOS/Git Bash (имя пользователя подставится на стороне сервера):

```bash
ssh USER@HOST 'sudo mkdir -p /opt/license-server && sudo chown "$USER":"$USER" /opt/license-server'
```

#### Вариант B — `rsync` (предпочтительно для обновлений)

Синхронизирует только изменения, умеет **исключать** файлы. Удобно из **Git Bash**, **WSL** или Linux/macOS.

Из родительской папки, где лежит `license-server`:

```bash
rsync -avz --delete \
  --exclude '.git' \
  --exclude '.env' \
  --exclude 'releases/*.zip' \
  ./license-server/ USER@HOST:/opt/license-server/
```

Пояснения:

- `-a` — права и время, `-v` — список, `-z` — сжатие по сети.
- `--delete` — на сервере удалятся файлы, которых больше нет локально (осторожно: не сотрёт `releases/`, если исключил zip; сам каталог `releases/` можно создать на сервере один раз).
- `--exclude '.env'` — **не перезатираем** продовый `.env` на сервере копией с ноутбука (если локальный `.env` с тестовыми паролями).
- При необходимости добавь `--exclude 'releases/'`, если zip кладёшь только на сервере.

С Windows **без** WSL: поставь `rsync` (например через cwRsync) или используй только `scp` и вручную следи за удалением старых файлов.

#### Только `.env` на сервер (первый раз или смена секретов)

**С локальной машины** (файл уже подготовлен и лежит рядом с проектом):

```bash
scp "E:\path\to\license-server\.env" USER@HOST:/opt/license-server/.env
```

Либо создай `.env` **прямо на сервере** (`nano`, см. §3) — так секреты не проходят через лишние копии на диске.

#### Только архив для портала (без пересборки образа)

```bash
scp "E:\path\to\SaturX-release.zip" USER@HOST:/opt/license-server/releases/
```

Права на чтение должны быть у пользователя, от имени которого Docker читает volume (обычно достаточно стандартных прав на файл).

#### Проверка после загрузки

```bash
ssh USER@HOST "ls -la /opt/license-server && test -f /opt/license-server/Dockerfile && test -f /opt/license-server/go.mod && echo OK"
```

Дальше на сервере: `cd /opt/license-server`, настрой `.env` (если ещё не создан), `docker compose build` и `up -d` — см. §5 и §7.

---

## 3. Файл `.env` на проде

Скопируй с локальной машины шаблон и заполни **свои** значения:

```bash
cp .env.example .env
nano .env
```

Обязательные переменные:

| Переменная | Описание |
|------------|----------|
| `POSTGRES_PASSWORD` | Пароль пользователя Postgres; в актуальном `docker-compose.yml` из него собирается `DATABASE_URL` с хостом **`postgres`** (имя сервиса). **Не задавай** в `.env` строки `DATABASE_URL`/`REDIS_URL` с `localhost` для Docker — внутри контейнера `app` localhost — это не база. |
| `HMAC_SECRET` | Длинная случайная строка (минимум ~32 символа). **Не меняй** на работающем проде без плана — иначе подписи в уже активированных клиентах перестанут сходиться. |
| `ADMIN_API_KEY` | Секрет для заголовка `X-Admin-API-Key` на `/admin/*`. |

Опционально (портал скачивания):

| Переменная | Описание |
|------------|----------|
| `DOWNLOAD_FILE_PATH` | **Путь внутри контейнера** к файлу, например `/data/releases/SaturX-release.zip` |
| `DOWNLOAD_FILE_NAME` | Имя файла в `Content-Disposition` (как увидит пользователь) |

Пример фрагмента `.env` (для compose из репозитория — **без** `DATABASE_URL`/`REDIS_URL`; они задаются в `docker-compose.yml`):

```env
PORT=8000
POSTGRES_PASSWORD=STRONG_DB_PASSWORD
HMAC_SECRET=...сгенерированный_секрет...
ADMIN_API_KEY=...длинный_случайный_ключ...
RATE_LIMIT_RPS=100
```

Сгенерировать секреты (на VPS):

```bash
openssl rand -base64 32
```

---

## 4. Docker Compose: том с файлом для портала

Базовый `docker-compose.yml` из репозитория поднимает `app`, `postgres`, `redis`. Для портала нужно:

1. Смонтировать каталог с zip в контейнер `app`.
2. Пути `DOWNLOAD_*` заданы в `environment` сервиса `app` в репозитории; при необходимости поправь там же.

`env_file: .env` для **всего** `.env` не используй: если в `.env` останется `DATABASE_URL=...@localhost`, контейнер попытается ходить в БД на localhost и получит `connection refused`. Секреты подставляются через `${HMAC_SECRET}` и т.д. в `docker-compose.yml`.

Положи файл на хост: `/opt/license-server/releases/SaturX-release.zip`.

**Важно:** если `DOWNLOAD_FILE_PATH` пустой, маршруты `/download` **не регистрируются** — это нормально, если портал не нужен.

---

## 5. Первый запуск (greenfield)

```bash
cd /opt/license-server
docker compose build --no-cache
docker compose up -d
docker compose ps
docker compose logs -f app
```

Проверки:

```bash
curl -sS http://127.0.0.1:8000/health
```

Ожидается JSON с `"status":"ok"`.

Если настроен портал:

```bash
curl -sS -o /dev/null -w "%{http_code}" http://127.0.0.1:8000/download
# ожидается 200
```

---

## 6. HTTPS перед приложением (кратко)

На той же VPS поставь **Caddy** или **Nginx**:

- Прокси: `https://твой-домен` → `http://127.0.0.1:8000`
- Сертификаты: Let’s Encrypt (Caddy выдаёт сам; в Nginx — certbot).

В боте и у клиентов указывай **только HTTPS**-URL API и, при необходимости, `https://домен/download` в `SOFTWARE_DOWNLOAD_URL`.

---

## 7. Полный передеплой (обновление кода)

Выполняй, когда вышла новая версия `license-server` (изменился код, шаблоны, зависимости).

### 7.1. Резервная копия БД (настоятельно рекомендуется)

```bash
cd /opt/license-server
docker compose exec -T postgres pg_dump -U postgres licensedb > "backup-licensedb-$(date +%F-%H%M).sql"
```

Храни дамп вне VPS или в защищённом хранилище.

### 7.2. Обновить исходники

**Вариант A — git на сервере:**

```bash
cd /opt/license-server   # или путь к клону репозитория
git fetch origin
git checkout main          # или твоя ветка/тег
git pull
```

**Вариант B — деплой с локальной машины:** залить каталог `license-server` на VPS в `/opt/license-server` через **SCP или rsync** (подробно: **§2.1**).

### 7.3. Проверить `.env` и compose

- Новые переменные из `.env.example` в репозитории — перенеси в свой `.env`.
- Если добавились volume/сервисы в `docker-compose.yml` — смержи изменения вручную, не затирая свои секреты.

### 7.4. Пересобрать образ и перезапустить

```bash
cd /opt/license-server
docker compose build --no-cache app
docker compose up -d
```

`--no-cache` гарантирует свежую сборку Go; для ускорения повторных деплоев можно иногда убрать, если уверен в кэше.

### 7.5. Проверка после деплоя

```bash
docker compose ps
docker compose logs --tail=100 app
curl -sS http://127.0.0.1:8000/health
```

Проверь снаружи через домен (если есть proxy):

```bash
curl -sS https://твой-домен/health
```

### 7.6. Только замена zip без пересборки

Если меняется **только** файл по тому же пути на хосте (тот же `DOWNLOAD_FILE_PATH` в контейнере):

```bash
# скопировать новый архив поверх старого в ./releases/
```

Перезапуск контейнера **не обязателен**. Если поменял **имя файла** или путь в `.env` — после правки `.env`:

```bash
docker compose up -d app
```

---

## 8. Откат (rollback)

1. Верни предыдущий коммит / копию каталога на сервере.
2. `docker compose build --no-cache app && docker compose up -d`
3. При необходимости восстанови БД из дампа:

```bash
docker compose exec -T postgres psql -U postgres -d licensedb < backup-licensedb-....sql
```

(уточни процедуру под свой объём данных и окно простоя).

---

## 9. Чеклист «полный передеплой»

- [ ] Есть свежий **дамп Postgres**.
- [ ] Обновлён код (`git pull` или залит архив).
- [ ] `.env` актуален, секреты не утекли в логи/чат.
- [ ] `docker compose build` прошёл без ошибок.
- [ ] `docker compose up -d`, контейнеры `running`.
- [ ] `GET /health` OK локально и по HTTPS.
- [ ] При использовании портала: `GET /download` открывается, тестовая выдача ключа работает.
- [ ] Telegram-бот / десктоп указывают на **новый** базовый URL, если домен менялся.

---

## 10. Частые проблемы

| Симптом | Что проверить |
|---------|----------------|
| Контейнер `app` перезапускается | `docker compose logs app` — часто неверный `DATABASE_URL`, нет Redis, пустой `HMAC_SECRET`/`ADMIN_API_KEY`. |
| Портал 404 | Не задан `DOWNLOAD_FILE_PATH` или файл не смонтирован — путь в контейнере должен существовать. |
| 502 снаружи | Proxy не достучался до `127.0.0.1:8000`; firewall; `app` не запущен. |

---

## Связанные файлы в репозитории

- `README.md` — API и описание портала.
- `.env.example` — список переменных.
- `docker-compose.yml` — сервисы Postgres, Redis, приложение.

После смены **только** содержимого zip на диске полный rebuild образа не нужен; после смены **кода или env** — нужен шаг **§7.4**.
