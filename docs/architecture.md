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

Short command aliases keep common workflows easy to type:

```text
gmail auth [client-json]       open separate auth terminal, authorize, and store local credentials
gmail s [query] [-n N]         search and list matching emails
gmail r [query] [-n N]         read matching emails in terminal text
gmail d [query] [-n N] [-o DIR] download matching emails and attachments
```

Queries are Gmail search expressions, so one concise string can mix name, email, date, body, and attachment filters, for example:

```text
from:alice@example.com after:2026/01/01 invoice filename:pdf
"Andrius" newer_than:30d has:attachment report
```

## Boundaries

- `cmd/gmail`: process entry point only.
- `internal/app`: CLI parsing, command orchestration, cancellation, logging setup.
- `internal/auth`: OAuth client config, PKCE loopback authorization, keyring token storage.
- `internal/gmail`: Gmail API adapter and application operations for search/read/export retrieval.
- `internal/email`: domain message model, Gmail payload conversion, text rendering, safe filename helpers.
- `internal/export`: durable filesystem export implementation.
- `internal/logging`: structured `slog` runtime wiring.

Dependencies flow from presentation/application into domain and infrastructure adapters; domain rendering and filesystem safety remain testable without Gmail.

## Data ownership

- Gmail remains the source of truth for email data.
- Local keyring owns OAuth refresh/access tokens; tokens are never written to source files, logs, or terminal output.
- Local config owns only OAuth desktop client JSON, copied with owner-only permissions.
- Export directories are user-requested snapshots and are not authoritative caches.

## Security decisions

- Request only `gmail.readonly` scope.
- Use OAuth PKCE with a random state and localhost loopback callback in a separate authorization terminal window.
- Store tokens in OS keyring under service `gmail-cli`, account `default`.
- Store OAuth client JSON under the user config directory with `0600` permissions.
- Sanitize all filenames from email headers and attachment names; reject path traversal.
- Avoid logging query results bodies, OAuth codes, tokens, or client secrets.

## Reliability and observability

- Every network operation receives caller context and can be cancelled by SIGINT/SIGTERM.
- Gmail retrieval is bounded by `-n` limits.
- Export writes are directory-scoped and deterministic; duplicate names are suffixed.
- Structured logs use stable events such as `auth_start`, `gmail_search`, and `export_message`.
- CLI errors include actionable remediation without exposing secrets.

## Acceptance criteria

- `go test ./...`, `go vet ./...`, and `go build ./cmd/gmail` pass.
- Authentication stores tokens through keyring, not plaintext token files.
- Search lists date, sender, subject, attachment marker, and Gmail ID.
- Read prints essential metadata and a clean text body.
- Download exports messages oldest-to-newest into `yy-mm-dd hhmmss .../email.txt` plus attachments.
- Malicious or awkward subjects/attachment names cannot escape the output directory.

## Follow-up debt

- Add optional multi-account profiles (`-a` or config default) when needed.
- Add encrypted file-keyring fallback for headless machines without an OS secret service.
- Add integration tests with a fake Gmail HTTP server if command behavior grows.
- Add shell completions and release packaging after the CLI contract stabilizes.
