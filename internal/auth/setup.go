package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const oauthClientsURL = "https://console.cloud.google.com/auth/clients"
const gmailAPIURL = "https://console.cloud.google.com/apis/library/gmail.googleapis.com"

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

// FindClientConfigCandidate locates the newest valid Google OAuth Desktop
// client JSON in common download/current directories.
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

type clientConfigCandidate struct {
	path    string
	modTime int64
}

func clientConfigCandidates() ([]clientConfigCandidate, error) {
	var dirs []string
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, cwd)
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "Downloads"), filepath.Join(home, "Desktop"))
	}
	patterns := []string{"client_secret*.json", "credentials.json", "oauth-client.json"}
	seen := map[string]struct{}{}
	var candidates []clientConfigCandidate
	for _, dir := range dirs {
		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(dir, pattern))
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
				candidates = append(candidates, clientConfigCandidate{path: match, modTime: info.ModTime().UnixNano()})
			}
		}
	}
	slices.SortFunc(candidates, func(a, b clientConfigCandidate) int {
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
