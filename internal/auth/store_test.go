package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestProjectTokenFileRoundTrip(t *testing.T) {
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

	token := &oauth2.Token{AccessToken: "access", RefreshToken: "refresh", Expiry: time.Now().UTC().Round(0)}
	data, err := encodeTestToken(token)
	if err != nil {
		t.Fatalf("encode test token: %v", err)
	}
	if err := saveProjectTokenFile(data); err != nil {
		t.Fatalf("save project token file: %v", err)
	}

	path := filepath.Join(tmp, "secrets", "gmail-token.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat token file: %v", err)
	}
	got, err := loadProjectTokenFile()
	if err != nil {
		t.Fatalf("load project token file: %v", err)
	}
	if got.AccessToken != token.AccessToken || got.RefreshToken != token.RefreshToken || !got.Expiry.Equal(token.Expiry) {
		t.Fatalf("loaded token = %#v, want %#v", got, token)
	}
}

func encodeTestToken(token *oauth2.Token) ([]byte, error) {
	return json.Marshal(token)
}
