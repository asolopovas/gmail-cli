# Execution plan

## Goal

Create a secure, maintainable Go terminal Gmail client for searching, reading, and exporting emails with attachments as clean local text snapshots.

## Scope

- Initialize a Go 1.25 CLI project.
- Add Gmail read-only OAuth integration.
- Store refresh/access tokens in the OS keyring.
- Provide concise commands for auth, search, read, and download.
- Export messages in chronological order into timestamped folders with safe filenames.
- Add tests and documentation for core parsing/export safety.

## Acceptance criteria

- `gmail auth [client-json]` stores OAuth client config, opens a separate authorization terminal, and performs PKCE loopback auth.
- `gmail s [query]` lists matching messages with date, sender, subject, attachment marker, and ID.
- `gmail r [query]` renders matching messages as clean terminal text.
- `gmail d [query]` writes oldest-to-newest message folders named with `yy-mm-dd hhmmss` and includes `email.txt` plus attachments.
- Tokens are stored through the OS keyring, not plaintext token files.
- Filenames derived from message subjects and attachments are sanitized and constrained.
- `go test ./...`, `go vet ./...`, and `go build ./cmd/gmail` pass.

## Pending work

- Manual OAuth/Gmail smoke test with a real Google account and downloaded Desktop OAuth client JSON.
- Optional encrypted file-keyring fallback for headless Linux/CI environments.
- Optional multi-account profiles if required by future users.

## Completed work

- Created Go module and cohesive `cmd/` + `internal/` package layout.
- Implemented OAuth PKCE auth in a separate terminal window, keyring token store, and token refresh persistence.
- Implemented Gmail search/read/download adapter.
- Implemented domain parsing, text rendering, HTML fallback cleanup, and attachment extraction.
- Implemented durable export writer with timestamped directories and sanitized attachment paths.
- Added README and architecture documentation.
- Added unit tests for Gmail payload parsing, HTML cleanup, filename safety, and export safety.

## Decisions

- Use Gmail's native search syntax rather than inventing a second query language; it already covers sender/name, email, date, body terms, and attachment filename filters in one concise query.
- Use `gmail.readonly` only; no send/modify/delete capabilities are requested.
- Store OAuth tokens in OS keyring under `gmail-cli/default`; store OAuth client JSON in the user config directory with owner-only permissions.
- Export snapshots are not a cache or database; Gmail remains source of truth.

## Validation

- `go test ./...` passed.
- `go vet ./...` passed.
- `go build ./cmd/gmail` passed.
- `./gmail help` prints the expected command contract.

## Follow-up debt

- Add integration tests against a fake Gmail API HTTP server.
- Add shell completions once the CLI stabilizes.
- Add release artifacts and CI workflow when repository hosting is configured.
