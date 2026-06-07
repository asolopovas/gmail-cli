package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

const (
	appDirName       = "gmail-cli"
	clientConfigName = "oauth-client.json"
)

type Paths struct {
	ConfigDir  string
	ClientFile string
}

func DefaultPaths() (Paths, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("locate user config dir: %w", err)
	}
	configDir := filepath.Join(base, appDirName)
	return Paths{ConfigDir: configDir, ClientFile: filepath.Join(configDir, clientConfigName)}, nil
}

func StoreClientConfig(src string, paths Paths) error {
	if src == "" {
		return errors.New("client credential path is required")
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read oauth client file %q: %w", src, err)
	}
	if _, err := ConfigFromJSON(data); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.ConfigDir, 0o700); err != nil {
		return fmt.Errorf("create config dir %q: %w", paths.ConfigDir, err)
	}
	if err := os.WriteFile(paths.ClientFile, data, 0o600); err != nil {
		return fmt.Errorf("write oauth client config %q: %w", paths.ClientFile, err)
	}
	return nil
}

func LoadClientConfig(paths Paths) (*oauth2.Config, error) {
	data, err := os.ReadFile(paths.ClientFile)
	if err != nil {
		return nil, fmt.Errorf("read oauth client config %q: %w", paths.ClientFile, err)
	}
	return ConfigFromJSON(data)
}

func ConfigFromJSON(data []byte) (*oauth2.Config, error) {
	if !json.Valid(data) {
		return nil, errors.New("oauth client config is not valid JSON")
	}
	config, err := google.ConfigFromJSON(data, gmail.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("parse oauth client config: %w", err)
	}
	return config, nil
}
