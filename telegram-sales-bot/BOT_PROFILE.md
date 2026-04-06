# Тексты и картинка бота в Telegram

## Короткое описание (BotFather → `/setdescription`)

Показывается в списке чатов и в превью. Лимит **до ~120 символов** — если не влезет, укороти.

**English**

```text
SaturX: pay with USDT in Telegram, get your Kick chat app license instantly. Standard & Pro plans, renewal reminders.
```

**Русский**

```text
SaturX: оплата USDT в Telegram — лицензия на приложение для Kick сразу после оплаты. Тарифы Standard и Pro, напоминания.
```

---

## Описание в профиле (BotFather → `/setabouttext`)

Текст на странице бота до нажатия **Start**. Обычно до **512 символов**.

**English**

```text
Official SaturX sales bot. Buy access to the SaturX desktop app for Kick: embedded chat, multiple accounts, message presets, per-account proxies, optional viewer tools.

Pay with Telegram Crypto Pay (USDT). Standard — about $29/month. Pro — about $129/year. After payment your license key is sent here automatically; you can always check it under “My license”. Renewal reminders before expiry.

Support: use your seller’s contacts.
```

**Русский**

```text
Официальный бот продаж SaturX. Покупка доступа к десктоп-приложению для Kick: чат, несколько аккаунтов, пресеты сообщений, прокси, опционально вьюер.

Оплата через Crypto Pay (USDT). Standard — около $29/мес. Pro — около $129/год. После оплаты ключ приходит в этот чат; посмотреть снова — кнопка «My license». Напоминания перед окончанием срока.

Поддержка — по контактам продавца.
```

---

## Логотип без загрузки на сайт

### Картинка в сообщении `/start`

По умолчанию бот вшивает логотип в бинарник: файл **`internal/bot/welcome_photo.jpg`** (`go:embed`). Чтобы сменить картинку — замени этот JPG и пересобери (`go build`).

Дальше по приоритету:

1. **`WELCOME_PHOTO_URL`** (если задан) — грузится с HTTPS, встроенное фото не используется.
2. Иначе — встроенный **`welcome_photo.jpg`**.
3. Иначе — **`WELCOME_PHOTO_PATH`** или файлы рядом с cwd: `welcome_logo.png` / `welcome_logo.jpg` / `logo.png` / `logo.jpg`.

### Аватар бота (кружок в Telegram)

Внешний URL для этого **не нужен**. В **@BotFather** отправь команду **`/setuserpic`**, выбери бота и **пришли файл** логотипа с компьютера — так же, как обычную картинку в чат.
