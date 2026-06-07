# gmail-cli

A small Go 1.25+ terminal Gmail reader/exporter.

## Features

- Read-only Gmail OAuth scope.
- OAuth desktop flow with PKCE and a local loopback callback.
- Refresh token stored in the OS keyring (`gmail-cli` / `default`), with a git-ignored `secrets/gmail-token.json` development fallback when the keyring is unavailable.
- Human-friendly commands for finding, showing, and exporting mail; short aliases remain optional.
- Exports messages oldest-to-newest into timestamped folders like `26-06-07 142233 - Subject/email.txt`.
- Saves original attachments beside each `email.txt` with sanitized filenames.

## Install/build

```bash
go build -o gmail ./cmd/gmail
```

## First-time setup

Run:

```bash
./gmail auth
```

On first use this opens the Google Cloud setup pages in your browser. Enable the Gmail API, create an OAuth client with application type **Desktop app**, download the JSON, then run:

```bash
./gmail auth secrets/client_secret_....json
```

For development, keep downloaded OAuth client JSON files in the project-local `secrets/` directory. The whole directory is git-ignored and created by `gmail auth` when needed. If the downloaded client JSON is already in `secrets/`, `Downloads`, `Desktop`, or the current directory, `./gmail auth` auto-detects it. Authorization opens in a separate terminal window, launches your browser, and stores the token in the OS keyring, or in ignored `secrets/gmail-token.json` if the keyring is unavailable.

For terminal-only troubleshooting, use `./gmail auth --no-window`; it runs the OAuth loopback flow in the current terminal and prints the authorization URL if the browser does not open.

Check whether local credentials are ready without contacting Gmail:

```bash
./gmail doctor
```

Next runs can use the stored client config and saved token:

```bash
./gmail auth
```

## Commands

```text
gmail auth [client-json]
gmail find [flags] [what to find]
gmail show [flags] [what to show]
gmail export [flags] [what to export]
gmail doctor
gmail completion bash|fish|powershell
```

Aliases: `login=auth`, `search/list=find`, `read/view=show`, `download/save=export`, `status=doctor`. Short aliases `a`, `s`, `r`, and `d` still work.

Queries can be simple human phrases:

```bash
./gmail find emails from alice@example.com about invoice newer than 30d
./gmail show unread emails from ebay last week
./gmail export emails with attachments from alice@example.com to folder exports
```

Or use explicit flags:

```bash
./gmail find --from alice@example.com --date 2026-06-01..2026-06-07 --subject invoice
./gmail show --unread --date today
./gmail export --attachments --after 2026-06-01 --output exports
```

`--date` accepts values like `today`, `yesterday`, `last-week`, `7d`, `2026-06-01`, or `2026-06-01..2026-06-07`.

Shell completions are generated with:

```bash
./gmail completion powershell > gmail.ps1
./gmail completion fish > gmail.fish
./gmail completion bash > gmail.bash
```

Raw Gmail search syntax still works and can be mixed in when useful:

```bash
./gmail find -n 5 'from:alice@example.com newer_than:30d invoice'
./gmail show 'subject:(security alert) after:2026/01/01'
./gmail export -o exports 'has:attachment filename:pdf report'
```

Useful Gmail query terms include `from:`, `to:`, `after:YYYY/MM/DD`, `before:YYYY/MM/DD`, `newer_than:7d`, quoted body text, `has:attachment`, and `filename:`.

## Validation

```bash
go test ./...
go vet ./...
go build ./cmd/gmail
```

See `docs/architecture.md` for design decisions, boundaries, acceptance criteria, and follow-up debt. See `docs/e2e.md` for the manual Gmail OAuth smoke-test checklist.
