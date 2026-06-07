package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

const (
	keyringService       = "gmail-cli"
	defaultAccount       = "default"
	projectTokenFileName = "gmail-token.json"
)

var ErrTokenNotFound = errors.New("oauth token not found")

type TokenStore struct {
	Service string
	Account string
}

func DefaultTokenStore() TokenStore {
	return TokenStore{Service: keyringService, Account: defaultAccount}
}

func (s TokenStore) normalized() TokenStore {
	if s.Service == "" {
		s.Service = keyringService
	}
	if s.Account == "" {
		s.Account = defaultAccount
	}
	return s
}

func (s TokenStore) Load() (*oauth2.Token, error) {
	s = s.normalized()
	data, err := keyring.Get(s.Service, s.Account)
	if err == nil {
		return decodeToken([]byte(data), "keyring token")
	}

	keyringErr := err
	fileToken, fileErr := loadProjectTokenFile()
	if fileErr == nil {
		return fileToken, nil
	}
	if errors.Is(keyringErr, keyring.ErrNotFound) && errors.Is(fileErr, os.ErrNotExist) {
		return nil, ErrTokenNotFound
	}
	if errors.Is(fileErr, os.ErrNotExist) {
		return nil, fmt.Errorf("load token from keyring: %w; ignored secrets fallback token not found", keyringErr)
	}
	return nil, fmt.Errorf("load token from keyring: %w; load ignored secrets fallback token: %v", keyringErr, fileErr)
}

func (s TokenStore) Save(token *oauth2.Token) error {
	if token == nil {
		return errors.New("cannot save nil oauth token")
	}
	s = s.normalized()
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("encode oauth token: %w", err)
	}
	if err := keyring.Set(s.Service, s.Account, string(data)); err == nil {
		return nil
	} else if fileErr := saveProjectTokenFile(data); fileErr != nil {
		return fmt.Errorf("save token to keyring: %w; save ignored secrets fallback token: %v", err, fileErr)
	}
	return nil
}

func (s TokenStore) Delete() error {
	s = s.normalized()
	var deleteErr error
	if err := keyring.Delete(s.Service, s.Account); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		deleteErr = fmt.Errorf("delete token from keyring: %w", err)
	}
	if path, err := ProjectTokenFile(); err == nil {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) && deleteErr == nil {
			deleteErr = fmt.Errorf("delete ignored secrets fallback token %q: %w", path, err)
		}
	}
	return deleteErr
}

func ProjectTokenFile() (string, error) {
	secretsDir, err := ProjectSecretsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(secretsDir, projectTokenFileName), nil
}

func loadProjectTokenFile() (*oauth2.Token, error) {
	path, err := ProjectTokenFile()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeToken(data, path)
}

func saveProjectTokenFile(data []byte) error {
	secretsDir, err := ProjectSecretsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(secretsDir, 0o700); err != nil {
		return fmt.Errorf("create secrets dir %q: %w", secretsDir, err)
	}
	path := filepath.Join(secretsDir, projectTokenFileName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write token file %q: %w", path, err)
	}
	return nil
}

func decodeToken(data []byte, source string) (*oauth2.Token, error) {
	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("decode %s: %w", source, err)
	}
	return &token, nil
}
