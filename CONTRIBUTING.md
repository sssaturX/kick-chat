# Contributing

Thank you for considering a contribution to SaturX.

## Ground rules

- Open an issue before large architectural changes.
- Keep pull requests focused: one concern per PR.
- Do not commit secrets, tokens, account files, proxy lists, or local license blobs.
- Do not add third-party authorship trailers (`Co-authored-by`, `Made-with`, or similar) or extra commit identities. Use only your own git `user.name` / `user.email`.
- Keep examples generic. Do not include real Kick channel names, account names, or personal paths.

## Development setup

1. Install [Go 1.24+](https://go.dev/dl/).
2. Copy `.env.example` to `.env` and fill in your Kick OAuth app credentials.
3. From the repository root:

```bash
go test ./...
go run .
```

The dashboard listens on `http://localhost:8080` by default.

Optional components:

- License server: see `license-server/README.md`
- Telegram sales bot: see `telegram-sales-bot/README.md`
- Landing page: see `landing/README.md`

## Tests

Run the Go test suite before opening a PR:

```bash
go test ./...
```

## Code style

- Match the surrounding Go style (`gofmt`).
- Prefer small, named helpers over large copy-paste blocks.
- Do not introduce unrelated refactors in a bugfix PR.

## Security reports

If you found a vulnerability, do not open a public issue. See [SECURITY.md](SECURITY.md).
