package auth

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const oauthClientsURL = "https://console.cloud.google.com/auth/clients"
const gmailAPIURL = "https://console.cloud.google.com/apis/library/gmail.googleapis.com"
const projectSecretsDirName = "secrets"

// OpenOAuthSetupPages opens the Google Cloud pages a user needs for first-run
// Gmail OAuth setup. Creating the OAuth client cannot be done safely without
// the user's Google Cloud account/project choices, so the CLI opens the exact
// browser workflow and then auto-detects downloaded client JSON on later runs.
func OpenOAuthSetupPages() error {
	if err := openBrowser(gmailAPIURL); err != nil {
		return fmt.Errorf("open Gmail API page: %w", err)
	}
	if err := openBrowser(oauthClientsURL); err != nil {
		return fmt.Errorf("open OAuth clients page: %w", err)
	}
	return nil
}

// ProjectSecretsDir returns the project-local secret drop zone used during development.
func ProjectSecretsDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locate current directory: %w", err)
	}
	return filepath.Join(cwd, projectSecretsDirName), nil
}

// StoreProjectClientConfig validates src and copies it into the project-local
// git-ignored secrets directory. The returned path is safe to pass to
// StoreClientConfig for runtime config installation.
func StoreProjectClientConfig(src string) (string, error) {
	if src == "" {
		return "", fmt.Errorf("client credential path is required")
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("read oauth client file %q: %w", src, err)
	}
	if _, err := ConfigFromJSON(data); err != nil {
		return "", err
	}
	secretsDir, err := ProjectSecretsDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(secretsDir, 0o700); err != nil {
		return "", fmt.Errorf("create secrets dir %q: %w", secretsDir, err)
	}
	dst := filepath.Join(secretsDir, filepath.Base(src))
	if sameFile(src, dst) {
		return dst, nil
	}
	in, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("open oauth client file %q: %w", src, err)
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("create secrets oauth client file %q: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return "", fmt.Errorf("copy oauth client file to secrets: %w", err)
	}
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("close secrets oauth client file %q: %w", dst, err)
	}
	return dst, nil
}

// FindClientConfigCandidate locates the newest valid Google OAuth Desktop
// client JSON in the project secrets directory or common download/current directories.
func FindClientConfigCandidate() (string, error) {
	candidates, err := clientConfigCandidates()
	if err != nil {
		return "", err
	}
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate.path)
		if err != nil {
			continue
		}
		if _, err := ConfigFromJSON(data); err == nil {
			return candidate.path, nil
		}
	}
	return "", nil
}

func sameFile(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return filepath.Clean(absA) == filepath.Clean(absB)
}

type clientConfigCandidate struct {
	path     string
	priority int
	modTime  int64
}

type clientConfigDir struct {
	path     string
	priority int
}

func clientConfigCandidates() ([]clientConfigCandidate, error) {
	var dirs []clientConfigDir
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs,
			clientConfigDir{path: filepath.Join(cwd, projectSecretsDirName), priority: 0},
			clientConfigDir{path: cwd, priority: 1},
		)
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs,
			clientConfigDir{path: filepath.Join(home, "Downloads"), priority: 2},
			clientConfigDir{path: filepath.Join(home, "Desktop"), priority: 3},
		)
	}
	patterns := []string{"client_secret*.json", "credentials.json", "oauth-client.json"}
	seen := map[string]struct{}{}
	var candidates []clientConfigCandidate
	for _, dir := range dirs {
		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(dir.path, pattern))
			if err != nil {
				return nil, fmt.Errorf("scan OAuth client files: %w", err)
			}
			for _, match := range matches {
				match = filepath.Clean(match)
				if _, ok := seen[match]; ok {
					continue
				}
				seen[match] = struct{}{}
				info, err := os.Stat(match)
				if err != nil || info.IsDir() || strings.HasPrefix(filepath.Base(match), ".") {
					continue
				}
				candidates = append(candidates, clientConfigCandidate{path: match, priority: dir.priority, modTime: info.ModTime().UnixNano()})
			}
		}
	}
	slices.SortFunc(candidates, func(a, b clientConfigCandidate) int {
		if a.priority < b.priority {
			return -1
		}
		if a.priority > b.priority {
			return 1
		}
		if a.modTime > b.modTime {
			return -1
		}
		if a.modTime < b.modTime {
			return 1
		}
		return strings.Compare(a.path, b.path)
	})
	return candidates, nil
}
