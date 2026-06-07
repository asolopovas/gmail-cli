package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const fakeDesktopClientJSON = `{
  "installed": {
    "client_id": "test-client.apps.googleusercontent.com",
    "project_id": "gmail-cli-test",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
    "client_secret": "fake-test-secret",
    "redirect_uris": ["http://localhost"]
  }
}`

func TestStoreProjectClientConfigCopiesIntoSecretsDirectory(t *testing.T) {
	tmp := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Errorf("restore wd: %v", err)
		}
	})
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	src := filepath.Join(tmp, "client_secret_source.json")
	if err := os.WriteFile(src, []byte(fakeDesktopClientJSON), 0o600); err != nil {
		t.Fatalf("write source config: %v", err)
	}

	got, err := StoreProjectClientConfig(src)
	if err != nil {
		t.Fatalf("store project client config: %v", err)
	}
	want := filepath.Join(tmp, "secrets", "client_secret_source.json")
	if got != want {
		t.Fatalf("stored path = %q, want %q", got, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("stat copied config: %v", err)
	}
}

func TestFindClientConfigCandidatePrefersSecretsDirectory(t *testing.T) {
	tmp := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Errorf("restore wd: %v", err)
		}
	})
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	home := filepath.Join(tmp, "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.MkdirAll(filepath.Join(tmp, "secrets"), 0o700); err != nil {
		t.Fatalf("create secrets dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(home, "Downloads"), 0o700); err != nil {
		t.Fatalf("create downloads dir: %v", err)
	}

	secretsPath := filepath.Join(tmp, "secrets", "client_secret_secrets.json")
	downloadsPath := filepath.Join(home, "Downloads", "client_secret_downloads.json")
	if err := os.WriteFile(secretsPath, []byte(fakeDesktopClientJSON), 0o600); err != nil {
		t.Fatalf("write secrets config: %v", err)
	}
	if err := os.WriteFile(downloadsPath, []byte(fakeDesktopClientJSON), 0o600); err != nil {
		t.Fatalf("write downloads config: %v", err)
	}

	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now()
	if err := os.Chtimes(secretsPath, oldTime, oldTime); err != nil {
		t.Fatalf("set secrets mtime: %v", err)
	}
	if err := os.Chtimes(downloadsPath, newTime, newTime); err != nil {
		t.Fatalf("set downloads mtime: %v", err)
	}

	got, err := FindClientConfigCandidate()
	if err != nil {
		t.Fatalf("find client config candidate: %v", err)
	}
	if got != secretsPath {
		t.Fatalf("candidate = %q, want secrets path %q", got, secretsPath)
	}
}
