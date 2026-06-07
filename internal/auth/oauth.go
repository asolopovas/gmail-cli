package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"
)

const oauthTimeout = 10 * time.Minute

type savingTokenSource struct {
	source oauth2.TokenSource
	store  TokenStore
	last   string
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	token, err := s.source.Token()
	if err != nil {
		return nil, err
	}
	fingerprint := token.AccessToken + "\x00" + token.RefreshToken + "\x00" + token.Expiry.UTC().Format(time.RFC3339Nano)
	if fingerprint != s.last {
		if err := s.store.Save(token); err != nil {
			return nil, err
		}
		s.last = fingerprint
	}
	return token, nil
}

func NewHTTPClient(ctx context.Context, config *oauth2.Config, store TokenStore) (*http.Client, error) {
	if config == nil {
		return nil, errors.New("oauth config is required")
	}
	token, err := store.Load()
	if err != nil {
		return nil, err
	}
	source := config.TokenSource(ctx, token)
	return oauth2.NewClient(ctx, &savingTokenSource{source: source, store: store}), nil
}

func Authorize(ctx context.Context, config *oauth2.Config, store TokenStore, logger *slog.Logger) error {
	if config == nil {
		return errors.New("oauth config is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithTimeout(ctx, oauthTimeout)
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start oauth loopback listener: %w", err)
	}
	defer listener.Close()

	config = cloneConfig(config)
	config.RedirectURL = "http://" + listener.Addr().String() + "/oauth2/callback"

	state, err := randomURLToken(32)
	if err != nil {
		return fmt.Errorf("generate oauth state: %w", err)
	}
	verifier := oauth2.GenerateVerifier()

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "invalid oauth state", http.StatusBadRequest)
			select {
			case errCh <- errors.New("invalid oauth state"):
			default:
			}
			return
		}
		if msg := r.URL.Query().Get("error"); msg != "" {
			http.Error(w, "oauth error: "+msg, http.StatusBadRequest)
			select {
			case errCh <- fmt.Errorf("oauth provider returned error: %s", msg):
			default:
			}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing oauth code", http.StatusBadRequest)
			select {
			case errCh <- errors.New("missing oauth code"):
			default:
			}
			return
		}
		_, _ = w.Write([]byte("Authorization complete. You may close this browser tab and return to the terminal.\n"))
		select {
		case codeCh <- code:
		default:
		}
	})

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case errCh <- fmt.Errorf("oauth callback server: %w", err):
			default:
			}
		}
	}()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("prompt", "consent"))
	logger.Info("auth_start", "redirect", config.RedirectURL)
	fmt.Printf("Open this URL if your browser does not open automatically:\n%s\n\n", authURL)
	if err := openBrowser(authURL); err != nil {
		logger.Warn("auth_browser_open_failed", "error", err)
	}

	var code string
	select {
	case <-ctx.Done():
		return fmt.Errorf("oauth authorization timed out or was cancelled: %w", ctx.Err())
	case err := <-errCh:
		return err
	case code = <-codeCh:
	}

	token, err := config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return fmt.Errorf("exchange oauth code: %w", err)
	}
	if err := store.Save(token); err != nil {
		return err
	}
	logger.Info("auth_complete")
	return nil
}

func cloneConfig(config *oauth2.Config) *oauth2.Config {
	copy := *config
	copy.Scopes = append([]string(nil), config.Scopes...)
	return &copy
}

func randomURLToken(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
