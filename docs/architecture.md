# Gmail CLI architecture

## Goal

Build a small, secure terminal Gmail reader/exporter that can:

- authenticate a local user with Gmail read-only OAuth scopes;
- store refresh tokens in the operating-system keyring;
- search Gmail using concise, mixed Gmail search queries;
- read matching emails as clean terminal text;
- export matching emails in chronological order into timestamped folders with `email.txt` metadata/body and original attachments.

## Scope

In scope:

- Go 1.25+ command-line application;
- Gmail read-only API integration;
- local OAuth desktop flow with PKCE and loopback callback;
- OS keyring token storage and `0600` OAuth client config copy;
- deterministic text rendering and safe export paths;
- unit tests for parsing, rendering, and filesystem safety.

Out of scope for the first version:

- sending, modifying, deleting, or labeling mail;
- background sync/index database;
- full MIME fidelity or attachment-to-text conversion;
- multi-account profile management beyond one default local account.

## CLI contract

Commands use readable verbs first; terse aliases are optional only:

```text
gmail auth [client-json]             authorize and store local credentials
gmail find [flags] [what to find]    search and list matching emails
gmail show [flags] [what to show]    render matching emails in terminal text
gmail export [flags] [what]          export matching emails and attachments
gmail doctor                         check local OAuth client config and token status
gmail completion bash|fish|powershell generate shell completions
```

Aliases remain for experienced users: `login=auth`, `search/list=find`, `read/view=show`, `download/save=export`, `status=doctor`, plus `a`, `s`, `r`, and `d`.

Queries can be human phrases or explicit flags (`--from`, `--to`, `--subject`, `--date`, `--after`, `--before`, `--attachments`, `--unread`) that are normalized to Gmail search syntax, while raw Gmail expressions still work:

```text
emails from alice@example.com about invoice newer than 30d
unread emails from ebay last week
emails with attachments from alice@example.com to folder exports
--from alice@example.com --date 2026-06-01..2026-06-07 --subject invoice
from:alice@example.com after:2026/01/01 invoice filename:pdf
"Andrius" newer_than:30d has:attachment report
```

## Boundaries

- `cmd/gmail`: process entry point plus CLI parsing/orchestration. This avoids a one-file `internal/app` package with no grouping value.
- `internal/*`: only retained where package boundaries group domain/infrastructure code.
- `internal/auth`: OAuth client config, PKCE loopback authorization, keyring/token storage.
- `internal/gmail`: Gmail API adapter for message retrieval.
- `internal/email`: domain message model, Gmail payload conversion, text rendering, safe filename helpers.
- `internal/export`: durable filesystem export implementation.

Dependencies flow from presentation/application into domain and infrastructure adapters; domain rendering and filesystem safety remain testable without Gmail.

## Data ownership

- Gmail remains the source of truth for email data.
- Local keyring owns OAuth refresh/access tokens when available; in development environments where keyring writes fail, ignored `secrets/gmail-token.json` owns the token fallback. Tokens are never written to tracked source files, logs, or terminal output.
- Local config owns only OAuth desktop client JSON, copied with owner-only permissions.
- During development, downloaded OAuth client JSON and keyring-fallback token files may be placed in project `secrets/`; that directory is fully ignored and is only an input/drop zone/fallback store for auth setup.
- Export directories are user-requested snapshots and are not authoritative caches.

## Security decisions

- Request only `gmail.readonly` scope.
- Use OAuth PKCE with a random state and localhost loopback callback in a separate authorization terminal window.
- Store tokens in OS keyring under service `gmail-cli`, account `default`; fall back to ignored `secrets/gmail-token.json` when Windows Credential Manager or another OS keyring is unavailable.
- Store OAuth client JSON under the user config directory with `0600` permissions.
- Keep project-local OAuth client downloads and fallback tokens under `secrets/`, with the whole directory git-ignored.
- Sanitize all filenames from email headers and attachment names; reject path traversal.
- Avoid logging query results bodies, OAuth codes, tokens, or client secrets.

## Reliability and observability

- Every network operation receives caller context and can be cancelled by SIGINT/SIGTERM.
- Gmail retrieval is bounded by `-n` limits.
- Export writes are directory-scoped and deterministic; duplicate names are suffixed.
- Structured logs use stable events such as `auth_start`, `gmail_search`, and `export_message`.
- CLI errors and `gmail doctor` include actionable remediation without exposing secrets.

## Acceptance criteria

- `go test ./...`, `go vet ./...`, and `go build ./cmd/gmail` pass.
- Authentication stores tokens through keyring when available and otherwise through the ignored project `secrets/` fallback.
- `gmail doctor` reports whether local OAuth client config and token prerequisites exist before live Gmail smoke tests.
- Search lists date, sender, subject, attachment marker, and Gmail ID.
- Read prints essential metadata and a clean text body.
- Download exports messages oldest-to-newest into `yy-mm-dd hhmmss .../email.txt` plus attachments.
- Malicious or awkward subjects/attachment names cannot escape the output directory.

## Follow-up debt

- Add optional multi-account profiles (`-a` or config default) when needed.
- Add encrypted file-keyring fallback for headless machines without an OS secret service.
- Add integration tests with a fake Gmail HTTP server if command behavior grows.
- Add shell completions and release packaging after the CLI contract stabilizes.
