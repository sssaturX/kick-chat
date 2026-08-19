# Security Policy

## Reporting a vulnerability

Please report security issues privately to the repository owner. Do not file a public GitHub issue for secrets, authentication bypasses, or remote code execution.

Include:

- a short description of the issue
- affected component (`desktop app`, `license-server`, `telegram-sales-bot`, or `landing`)
- steps to reproduce
- impact assessment if you have one

## Secrets

Never commit:

- `.env`
- `.kick_accounts.json`
- `.kick_license.dat`
- `.kick_proxies`
- OAuth client secrets
- HMAC / admin API keys
- Telegram bot tokens

Use the `*.example` files as templates only.

## Disclaimer

This project talks to Kick through published APIs and local configuration. Users are responsible for complying with Kick’s terms of service and applicable law.
