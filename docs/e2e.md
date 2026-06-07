# Manual Gmail OAuth E2E checklist

Use this checklist when validating the CLI against a real Gmail account. It intentionally avoids printing or committing secrets.

## 1. Build and run the local binary

```bash
go build -o gmail.exe ./cmd/gmail
./gmail.exe help
```

On Unix-like systems, use `-o gmail` and `./gmail` instead.

## 2. Check credential readiness

```bash
./gmail.exe doctor
```

Expected before setup: the OAuth client config and/or OS keyring token are reported missing, followed by remediation steps.

## 3. Create the Google OAuth desktop client

```bash
./gmail.exe auth
```

This opens the Google Cloud setup pages. In Google Cloud:

1. Select or create a project.
2. Enable the Gmail API.
3. Configure the OAuth consent screen if prompted.
4. Create an OAuth client with application type **Desktop app**.
5. Download the JSON. Do not commit this file.
6. Move the JSON into the project-local ignored credential folder:

```bash
mkdir -p secrets
mv "$HOME/Downloads/client_secret_"*.json secrets/
```

If browser/window automation is not desired, run:

```bash
./gmail.exe auth --no-window
```

## 4. Store local client config and authorize

```bash
./gmail.exe auth "secrets/client_secret_....json"
```

If the downloaded JSON is in `secrets`, `Downloads`, `Desktop`, or the current directory, this can usually be shortened to:

```bash
./gmail.exe auth
```

Complete the browser consent flow. If Google shows **Google hasn’t verified this app**, click **Advanced** and continue to the app only if this is your own OAuth client/project. If Google blocks access, add your Gmail address as a test user on the OAuth consent screen. The refresh/access token is stored in the OS keyring under service `gmail-cli`, account `default`; if the keyring is unavailable, the CLI writes ignored `secrets/gmail-token.json` instead.

## 5. Verify credentials and run live Gmail smoke tests

```bash
./gmail.exe doctor
./gmail.exe find --number 5 --date 30d
./gmail.exe show --number 1 --date 30d
./gmail.exe export --number 1 --date 30d --output e2e-exports
```

Confirm that find prints summaries, show prints terminal-friendly message text, export creates one timestamped export folder containing `email.txt` and any attachments, and completions can be generated with `./gmail.exe completion powershell|fish|bash`.

## Current local run notes

- `go test ./...` passed.
- `go build -o gmail.exe ./cmd/gmail` passed.
- OAuth client JSON and any keyring-fallback token should be kept in `secrets/`, which is fully git-ignored.
- Live Gmail calls are blocked until a Desktop OAuth client JSON is stored with `./gmail.exe auth secrets/client_secret_....json` and authorized.
