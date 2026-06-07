# gmail-cli

A small Go 1.25+ terminal Gmail reader/exporter.

## Features

- Read-only Gmail OAuth scope.
- OAuth desktop flow with PKCE and a local loopback callback.
- Refresh token stored in the OS keyring (`gmail-cli` / `default`), not a plaintext token file.
- Concise commands for search, terminal reading, and exporting.
- Exports messages oldest-to-newest into timestamped folders like `26-06-07 142233 - Subject/email.txt`.
- Saves original attachments beside each `email.txt` with sanitized filenames.

## Install/build

```bash
go build -o gmail ./cmd/gmail
```

## First-time setup

1. Enable the Gmail API in Google Cloud.
2. Create an OAuth client with application type **Desktop app**.
3. Download the client JSON.
4. Authorize locally:

```bash
./gmail auth ~/Downloads/client_secret_....json
```

This opens a separate terminal window for the authorization flow, launches your browser, and automatically stores the token in the OS keyring after successful login.

Next runs can use the stored client config and keyring token:

```bash
./gmail auth
```

## Commands

```text
gmail auth [client-json]
gmail s [-n N] [query]
gmail r [-n N] [query]
gmail d [-n N] [-o DIR] [query]
```

Aliases: `a`, `s`, `r`, `d`.

Queries are Gmail search expressions and can mix filters:

```bash
./gmail s -n 5 'from:alice@example.com newer_than:30d invoice'
./gmail r 'subject:(security alert) after:2026/01/01'
./gmail d -o exports 'has:attachment filename:pdf report'
```

Useful Gmail query terms include `from:`, `to:`, `after:YYYY/MM/DD`, `before:YYYY/MM/DD`, `newer_than:7d`, quoted body text, `has:attachment`, and `filename:`.

## Validation

```bash
go test ./...
go vet ./...
go build ./cmd/gmail
```

See `docs/architecture.md` for design decisions, boundaries, acceptance criteria, and follow-up debt.
