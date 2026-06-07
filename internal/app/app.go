package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/andrius/gmail-cli/internal/auth"
	"github.com/andrius/gmail-cli/internal/email"
	"github.com/andrius/gmail-cli/internal/export"
	gmailclient "github.com/andrius/gmail-cli/internal/gmail"
	"github.com/andrius/gmail-cli/internal/logging"
)

type Runner struct {
	Stdout io.Writer
	Stderr io.Writer
	Logger *slog.Logger
}

func Main(args []string) int {
	runner := Runner{Stdout: os.Stdout, Stderr: os.Stderr}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runner.Run(ctx, args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func (r Runner) Run(ctx context.Context, args []string) error {
	if r.Stdout == nil {
		r.Stdout = io.Discard
	}
	if r.Stderr == nil {
		r.Stderr = io.Discard
	}
	if len(args) == 0 {
		printHelp(r.Stdout)
		return nil
	}
	debug, args := splitGlobalFlags(args)
	logger := r.Logger
	if logger == nil {
		logger = logging.New(debug, r.Stderr)
	}

	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "help", "h", "-h", "--help":
		printHelp(r.Stdout)
		return nil
	case "auth", "a":
		return r.auth(ctx, rest, logger, debug)
	case "search", "s":
		return r.search(ctx, rest, logger)
	case "read", "r":
		return r.read(ctx, rest, logger)
	case "download", "down", "d":
		return r.download(ctx, rest, logger)
	case "logout":
		return auth.DefaultTokenStore().Delete()
	default:
		return fmt.Errorf("unknown command %q; run `gmail help`", cmd)
	}
}

func splitGlobalFlags(args []string) (bool, []string) {
	debug := false
	out := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--debug":
			debug = true
		default:
			out = append(out, arg)
		}
	}
	return debug, out
}

func (r Runner) auth(ctx context.Context, args []string, logger *slog.Logger, debug bool) error {
	fs := flag.NewFlagSet("auth", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	noWindow := fs.Bool("no-window", false, "run authorization in this terminal")
	if err := fs.Parse(args); err != nil {
		return err
	}
	paths, err := auth.DefaultPaths()
	if err != nil {
		return err
	}
	if fs.NArg() > 0 {
		if err := auth.StoreClientConfig(fs.Arg(0), paths); err != nil {
			return err
		}
		fmt.Fprintf(r.Stdout, "Stored OAuth client config at %s\n", paths.ClientFile)
	}
	if !*noWindow {
		childArgs := []string{"auth", "--no-window"}
		if debug {
			childArgs = append([]string{"--debug"}, childArgs...)
		}
		if err := auth.LaunchAuthTerminal(childArgs); err != nil {
			return fmt.Errorf("open separate authorization terminal: %w", err)
		}
		fmt.Fprintln(r.Stdout, "Opened Gmail authorization in a separate terminal window. Complete browser login there; the token will be stored automatically in the OS keyring.")
		return nil
	}
	config, err := auth.LoadClientConfig(paths)
	if err != nil {
		return fmt.Errorf("load oauth client config: %w (run `gmail auth /path/to/credentials.json` first)", err)
	}
	if err := auth.Authorize(ctx, config, auth.DefaultTokenStore(), logger); err != nil {
		return err
	}
	fmt.Fprintln(r.Stdout, "Authorized Gmail read-only access. Token is stored in the OS keyring.")
	return nil
}

func (r Runner) search(ctx context.Context, args []string, logger *slog.Logger) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	limit := fs.Int("n", 10, "maximum messages")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := newGmailClient(ctx, logger)
	if err != nil {
		return err
	}
	messages, err := client.Search(ctx, gmailclient.SearchOptions{Query: joinQuery(fs.Args()), Limit: *limit, IncludePayload: false})
	if err != nil {
		return err
	}
	for _, msg := range messages {
		fmt.Fprintln(r.Stdout, email.Summary(msg))
	}
	return nil
}

func (r Runner) read(ctx context.Context, args []string, logger *slog.Logger) error {
	fs := flag.NewFlagSet("read", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	limit := fs.Int("n", 10, "maximum messages")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := newGmailClient(ctx, logger)
	if err != nil {
		return err
	}
	messages, err := client.Search(ctx, gmailclient.SearchOptions{Query: joinQuery(fs.Args()), Limit: *limit, IncludePayload: true})
	if err != nil {
		return err
	}
	for i, msg := range messages {
		if i > 0 {
			fmt.Fprintln(r.Stdout, strings.Repeat("=", 80))
		}
		fmt.Fprint(r.Stdout, email.Render(msg))
	}
	return nil
}

func (r Runner) download(ctx context.Context, args []string, logger *slog.Logger) error {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	limit := fs.Int("n", 100, "maximum messages")
	outDir := fs.String("o", "exports", "output directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	client, err := newGmailClient(ctx, logger)
	if err != nil {
		return err
	}
	messages, err := client.Search(ctx, gmailclient.SearchOptions{Query: joinQuery(fs.Args()), Limit: *limit, IncludePayload: true, IncludeAttachments: true})
	if err != nil {
		return err
	}
	writer := export.Writer{Root: *outDir}
	for _, msg := range messages {
		result, err := writer.Save(msg)
		if err != nil {
			return err
		}
		logger.Info("export_message", "gmail_id", msg.ID, "directory", result.Directory, "attachments", len(result.Attachments))
		fmt.Fprintf(r.Stdout, "%s\n", result.Directory)
	}
	return nil
}

func newGmailClient(ctx context.Context, logger *slog.Logger) (*gmailclient.Client, error) {
	paths, err := auth.DefaultPaths()
	if err != nil {
		return nil, err
	}
	config, err := auth.LoadClientConfig(paths)
	if err != nil {
		return nil, fmt.Errorf("load oauth client config: %w (run `gmail auth /path/to/credentials.json` first)", err)
	}
	httpClient, err := auth.NewHTTPClient(ctx, config, auth.DefaultTokenStore())
	if err != nil {
		if errors.Is(err, auth.ErrTokenNotFound) {
			return nil, fmt.Errorf("no OAuth token found; run `gmail auth` first")
		}
		return nil, err
	}
	return gmailclient.New(ctx, httpClient, logger)
}

func joinQuery(args []string) string {
	return strings.TrimSpace(strings.Join(args, " "))
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, `gmail - read and export Gmail as terminal-friendly text

Usage:
  gmail auth [client-json]       open separate auth terminal and authorize Gmail read-only access
  gmail s [-n N] [query]         search emails
  gmail r [-n N] [query]         read emails as clean text in the terminal
  gmail d [-n N] [-o DIR] [query] download emails plus attachments into timestamped folders

Short aliases: a=auth, s=search, r=read, d=download.
Global flag: --debug

Query examples:
  gmail s -n 5 'from:alice@example.com newer_than:30d invoice'
  gmail r 'subject:(security alert) after:2026/01/01'
  gmail d -o exports 'has:attachment filename:pdf report'

Queries use Gmail search syntax, so name, email, date, body text, and attachment filters can be mixed in one string.
`)
}
