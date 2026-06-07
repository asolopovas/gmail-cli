package auth

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

const (
	keyringService = "gmail-cli"
	defaultAccount = "default"
)

var ErrTokenNotFound = errors.New("oauth token not found in keyring")

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
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("load token from keyring: %w", err)
	}
	var token oauth2.Token
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return nil, fmt.Errorf("decode keyring token: %w", err)
	}
	return &token, nil
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
	if err := keyring.Set(s.Service, s.Account, string(data)); err != nil {
		return fmt.Errorf("save token to keyring: %w", err)
	}
	return nil
}

func (s TokenStore) Delete() error {
	s = s.normalized()
	if err := keyring.Delete(s.Service, s.Account); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("delete token from keyring: %w", err)
	}
	return nil
}
