package main

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
	"time"

	"github.com/spf13/cobra"

	"github.com/andrius/gmail-cli/internal/auth"
	"github.com/andrius/gmail-cli/internal/email"
	"github.com/andrius/gmail-cli/internal/export"
	gmailclient "github.com/andrius/gmail-cli/internal/gmail"
)

type Runner struct {
	Stdout io.Writer
	Stderr io.Writer
	Logger *slog.Logger
}

func newLogger(debug bool, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
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
	cmd := r.command()
	cmd.SetArgs(args)
	cmd.SetOut(r.Stdout)
	cmd.SetErr(r.Stderr)
	return cmd.ExecuteContext(ctx)
}

func (r Runner) command() *cobra.Command {
	var debug bool
	logger := func() *slog.Logger {
		if r.Logger != nil {
			return r.Logger
		}
		return newLogger(debug, r.Stderr)
	}

	root := &cobra.Command{
		Use:           "gmail",
		Short:         "Read and export Gmail as terminal-friendly text",
		SilenceUsage:  true,
		SilenceErrors: true,
		Run: func(cmd *cobra.Command, args []string) {
			printManualHelp(r.Stdout, cmd)
		},
	}
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		printManualHelp(r.Stdout, cmd)
	})
	root.PersistentFlags().BoolVar(&debug, "debug", false, "enable structured debug logs")

	var authNoWindow bool
	authCmd := &cobra.Command{
		Use:     "auth [client-json]",
		Aliases: []string{"login", "a"},
		Short:   "Authorize Gmail read-only access",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			callArgs := append([]string(nil), args...)
			if authNoWindow {
				callArgs = append([]string{"--no-window"}, callArgs...)
			}
			return r.auth(cmd.Context(), callArgs, logger(), debug)
		},
	}
	authCmd.Flags().BoolVar(&authNoWindow, "no-window", false, "run authorization in this terminal")

	var searchLimit int
	var searchQuery queryFlags
	findCmd := &cobra.Command{
		Use:     "find [what to find]",
		Aliases: []string{"search", "list", "ls", "s"},
		Short:   "List matching emails",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return r.search(cmd.Context(), appendLimitArgs(searchLimit, searchQuery.args(args)), logger())
		},
	}
	findCmd.Flags().IntVarP(&searchLimit, "number", "n", 10, "maximum messages")
	bindQueryFlags(findCmd, &searchQuery)

	var readLimit int
	var readQuery queryFlags
	showCmd := &cobra.Command{
		Use:     "show [what to show]",
		Aliases: []string{"read", "view", "r"},
		Short:   "Print matching emails as terminal text",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return r.read(cmd.Context(), appendLimitArgs(readLimit, readQuery.args(args)), logger())
		},
	}
	showCmd.Flags().IntVarP(&readLimit, "number", "n", 10, "maximum messages")
	bindQueryFlags(showCmd, &readQuery)

	var exportLimit int
	var exportOut string
	var exportQuery queryFlags
	exportCmd := &cobra.Command{
		Use:     "export [what to export]",
		Aliases: []string{"download", "save", "down", "d"},
		Short:   "Export emails plus attachments into folders",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return r.download(cmd.Context(), appendExportArgs(exportLimit, exportOut, exportQuery.args(args)), logger())
		},
	}
	exportCmd.Flags().IntVarP(&exportLimit, "number", "n", 100, "maximum messages")
	exportCmd.Flags().StringVarP(&exportOut, "output", "o", "exports", "output directory")
	bindQueryFlags(exportCmd, &exportQuery)

	doctorCmd := &cobra.Command{
		Use:     "doctor",
		Aliases: []string{"status"},
		Short:   "Check local OAuth client config and saved token",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return r.doctor()
		},
	}

	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Delete saved OAuth token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return auth.DefaultTokenStore().Delete()
		},
	}

	completionCmd := completionCommand(root, r.Stdout)
	root.AddCommand(authCmd, findCmd, showCmd, exportCmd, doctorCmd, logoutCmd, completionCmd)
	for _, child := range root.Commands() {
		child.SetHelpFunc(func(cmd *cobra.Command, args []string) {
			printManualHelp(r.Stdout, cmd)
		})
	}
	return root
}

func appendLimitArgs(limit int, args []string) []string {
	out := []string{"-n", fmt.Sprint(limit)}
	return append(out, args...)
}

func appendExportArgs(limit int, outDir string, args []string) []string {
	out := []string{"-n", fmt.Sprint(limit), "-o", outDir}
	return append(out, args...)
}

func completionCommand(root *cobra.Command, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|fish|powershell]",
		Short: "Generate shell completion script",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch strings.ToLower(args[0]) {
			case "bash":
				return root.GenBashCompletion(out)
			case "fish":
				return root.GenFishCompletion(out, true)
			case "powershell", "pwsh", "power-shell":
				return root.GenPowerShellCompletionWithDesc(out)
			default:
				return fmt.Errorf("unsupported shell %q; expected bash, fish, or powershell", args[0])
			}
		},
	}
}

type queryFlags struct {
	query       string
	from        string
	to          string
	subject     string
	date        string
	after       string
	before      string
	newer       string
	older       string
	attachments bool
	unread      bool
	starred     bool
	important   bool
}

func bindQueryFlags(cmd *cobra.Command, q *queryFlags) {
	cmd.Flags().StringVar(&q.query, "query", "", "raw Gmail search query")
	cmd.Flags().StringVar(&q.from, "from", "", "sender email/name")
	cmd.Flags().StringVar(&q.to, "to", "", "recipient email/name")
	cmd.Flags().StringVar(&q.subject, "subject", "", "subject text")
	cmd.Flags().StringVar(&q.date, "date", "", "date filter: today, yesterday, last-week, 7d, YYYY-MM-DD, or START..END")
	cmd.Flags().StringVar(&q.after, "after", "", "messages after date, e.g. 2026-06-01")
	cmd.Flags().StringVar(&q.before, "before", "", "messages before date, e.g. 2026-07-01")
	cmd.Flags().StringVar(&q.newer, "newer", "", "messages newer than duration, e.g. 7d or 1m")
	cmd.Flags().StringVar(&q.older, "older", "", "messages older than duration, e.g. 30d or 1y")
	cmd.Flags().BoolVar(&q.attachments, "attachments", false, "only messages with attachments")
	cmd.Flags().BoolVar(&q.unread, "unread", false, "only unread messages")
	cmd.Flags().BoolVar(&q.starred, "starred", false, "only starred messages")
	cmd.Flags().BoolVar(&q.important, "important", false, "only important messages")
}

func (q queryFlags) args(base []string) []string {
	out := append([]string(nil), base...)
	if q.query != "" {
		out = append(out, q.query)
	}
	if q.from != "" {
		out = append(out, "from:"+q.from)
	}
	if q.to != "" {
		out = append(out, "to:"+q.to)
	}
	if q.subject != "" {
		out = append(out, "subject:("+q.subject+")")
	}
	out = append(out, dateQueryTokens(q.date, time.Now())...)
	if q.after != "" {
		out = append(out, "after:"+gmailDate(q.after, time.Now()))
	}
	if q.before != "" {
		out = append(out, "before:"+gmailDate(q.before, time.Now()))
	}
	if q.newer != "" {
		out = append(out, "newer_than:"+humanDuration(q.newer))
	}
	if q.older != "" {
		out = append(out, "older_than:"+humanDuration(q.older))
	}
	if q.attachments {
		out = append(out, "has:attachment")
	}
	if q.unread {
		out = append(out, "is:unread")
	}
	if q.starred {
		out = append(out, "is:starred")
	}
	if q.important {
		out = append(out, "is:important")
	}
	return out
}

func dateQueryTokens(value string, now time.Time) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	lower := strings.ToLower(value)
	switch lower {
	case "today":
		start := startOfDay(now)
		return []string{"after:" + formatGmailDate(start), "before:" + formatGmailDate(start.AddDate(0, 0, 1))}
	case "yesterday":
		start := startOfDay(now).AddDate(0, 0, -1)
		return []string{"after:" + formatGmailDate(start), "before:" + formatGmailDate(start.AddDate(0, 0, 1))}
	case "week", "this-week", "last-week", "last week":
		return []string{"newer_than:7d"}
	case "month", "this-month", "last-month", "last month":
		return []string{"newer_than:30d"}
	case "year", "this-year", "last-year", "last year":
		return []string{"newer_than:365d"}
	}
	if looksLikeDuration(lower) {
		return []string{"newer_than:" + humanDuration(lower)}
	}
	if start, end, ok := strings.Cut(value, ".."); ok {
		return []string{"after:" + gmailDate(start, now), "before:" + formatGmailDate(parseDateOrNow(end, now).AddDate(0, 0, 1))}
	}
	day := parseDateOrNow(value, now)
	return []string{"after:" + formatGmailDate(day), "before:" + formatGmailDate(day.AddDate(0, 0, 1))}
}

func gmailDate(value string, now time.Time) string {
	return formatGmailDate(parseDateOrNow(value, now))
}

func parseDateOrNow(value string, now time.Time) time.Time {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	switch lower {
	case "today":
		return startOfDay(now)
	case "yesterday":
		return startOfDay(now).AddDate(0, 0, -1)
	}
	layouts := []string{time.RFC3339, "2006-01-02", "2006/01/02", "02/01/2006", "02-01-2006", "Jan 2 2006", "2 Jan 2006", "January 2 2006", "2 January 2006"}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, now.Location()); err == nil {
			return startOfDay(parsed)
		}
	}
	return startOfDay(now)
}

func formatGmailDate(t time.Time) string {
	return t.Format("2006/01/02")
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func looksLikeDuration(value string) bool {
	if len(value) < 2 {
		return false
	}
	unit := value[len(value)-1]
	if unit != 'd' && unit != 'm' && unit != 'y' {
		return false
	}
	for _, r := range value[:len(value)-1] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
		secretPath, err := auth.StoreProjectClientConfig(fs.Arg(0))
		if err != nil {
			return err
		}
		if err := auth.StoreClientConfig(secretPath, paths); err != nil {
			return err
		}
		fmt.Fprintf(r.Stdout, "Stored OAuth client config at %s\n", paths.ClientFile)
		fmt.Fprintf(r.Stdout, "Kept OAuth client JSON in ignored secrets folder at %s\n", secretPath)
	} else if _, err := auth.LoadClientConfig(paths); err != nil {
		candidate, findErr := auth.FindClientConfigCandidate()
		if findErr != nil {
			return findErr
		}
		if candidate == "" {
			openedSetupPages := false
			if !*noWindow {
				if openErr := auth.OpenOAuthSetupPages(); openErr != nil {
					return fmt.Errorf("open Google OAuth setup pages: %w", openErr)
				}
				openedSetupPages = true
			}
			return firstTimeSetupError(openedSetupPages)
		}
		secretPath, err := auth.StoreProjectClientConfig(candidate)
		if err != nil {
			return err
		}
		if err := auth.StoreClientConfig(secretPath, paths); err != nil {
			return err
		}
		fmt.Fprintf(r.Stdout, "Found and stored OAuth client config from %s\n", secretPath)
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
		return fmt.Errorf("load oauth client config: %w (run `gmail auth secrets/client_secret_....json` first)", err)
	}
	if err := auth.Authorize(ctx, config, auth.DefaultTokenStore(), logger); err != nil {
		return err
	}
	fmt.Fprintln(r.Stdout, "Authorized Gmail read-only access. Token is stored in the OS keyring, or in ignored secrets/gmail-token.json if the keyring is unavailable.")
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
	messages, err := client.Search(ctx, gmailclient.SearchOptions{Query: humanGmailQuery(fs.Args()), Limit: *limit, IncludePayload: false})
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
	messages, err := client.Search(ctx, gmailclient.SearchOptions{Query: humanGmailQuery(fs.Args()), Limit: *limit, IncludePayload: true})
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
	queryArgs, humanOutDir := splitHumanExportDestination(fs.Args())
	if *outDir == "exports" && humanOutDir != "" {
		*outDir = humanOutDir
	}
	messages, err := client.Search(ctx, gmailclient.SearchOptions{Query: humanGmailQuery(queryArgs), Limit: *limit, IncludePayload: true, IncludeAttachments: true})
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

func (r Runner) doctor() error {
	paths, err := auth.DefaultPaths()
	if err != nil {
		return err
	}
	ready := true
	if _, err := auth.LoadClientConfig(paths); err != nil {
		ready = false
		fmt.Fprintf(r.Stdout, "OAuth client config: missing (%s)\n", paths.ClientFile)
		fmt.Fprintln(r.Stdout, "Next: create a Google Cloud Desktop OAuth client, download its JSON, then run `gmail auth secrets/client_secret_....json`.")
	} else {
		fmt.Fprintf(r.Stdout, "OAuth client config: found (%s)\n", paths.ClientFile)
	}

	if _, err := auth.DefaultTokenStore().Load(); err != nil {
		ready = false
		if errors.Is(err, auth.ErrTokenNotFound) {
			fmt.Fprintln(r.Stdout, "OAuth token: missing from OS keyring and ignored secrets fallback")
			fmt.Fprintln(r.Stdout, "Next: run `gmail auth` after the OAuth client config is stored.")
		} else {
			fmt.Fprintf(r.Stdout, "OAuth token: could not be read: %v\n", err)
			fmt.Fprintln(r.Stdout, "Next: verify your OS keyring or ignored secrets fallback is available, then run `gmail auth` again.")
		}
	} else {
		fmt.Fprintln(r.Stdout, "OAuth token: found")
	}

	if !ready {
		return errors.New("credentials are not ready")
	}
	fmt.Fprintln(r.Stdout, "Credentials are ready. Try `gmail find emails from the last 7 days` for a live Gmail smoke test.")
	return nil
}

func firstTimeSetupError(openedSetupPages bool) error {
	prefix := "first-time setup required."
	if openedSetupPages {
		prefix = "first-time setup required; Google Cloud setup pages opened in your browser."
	}
	return fmt.Errorf("%s In Google Cloud: enable Gmail API, create an OAuth client with Application type 'Desktop app', download the JSON, move it to `secrets/`, then run `gmail auth secrets/client_secret_....json`", prefix)
}

func newGmailClient(ctx context.Context, logger *slog.Logger) (*gmailclient.Client, error) {
	paths, err := auth.DefaultPaths()
	if err != nil {
		return nil, err
	}
	config, err := auth.LoadClientConfig(paths)
	if err != nil {
		return nil, fmt.Errorf("not set up yet; run `gmail auth` to open setup pages, then run `gmail auth secrets/client_secret_....json` after downloading a Desktop OAuth client JSON (use `gmail doctor` to check status): %w", err)
	}
	httpClient, err := auth.NewHTTPClient(ctx, config, auth.DefaultTokenStore())
	if err != nil {
		if errors.Is(err, auth.ErrTokenNotFound) {
			return nil, fmt.Errorf("no OAuth token found; run `gmail auth` first (use `gmail doctor` to check credential status)")
		}
		return nil, err
	}
	return gmailclient.New(ctx, httpClient, logger)
}

func humanGmailQuery(args []string) string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		token := strings.TrimSpace(args[i])
		if token == "" {
			continue
		}
		lower := strings.ToLower(token)
		switch {
		case isFillerWord(lower):
			continue
		case lower == "about" || lower == "containing" || lower == "contains":
			continue
		case lower == "from" && i+1 < len(args):
			i++
			out = append(out, "from:"+args[i])
		case lower == "to" && i+1 < len(args):
			i++
			out = append(out, "to:"+args[i])
		case (lower == "after" || lower == "since") && i+1 < len(args):
			i++
			out = append(out, "after:"+args[i])
		case (lower == "before" || lower == "until") && i+1 < len(args):
			i++
			out = append(out, "before:"+args[i])
		case lower == "newer" && nextIs(args, i, "than") && i+2 < len(args):
			i += 2
			out = append(out, "newer_than:"+args[i])
		case lower == "older" && nextIs(args, i, "than") && i+2 < len(args):
			i += 2
			out = append(out, "older_than:"+args[i])
		case lower == "last" && i+1 < len(args):
			i++
			out = append(out, "newer_than:"+humanDuration(args[i]))
		case (lower == "with" || lower == "has") && i+1 < len(args) && isAttachmentWord(args[i+1]):
			i++
			out = append(out, "has:attachment")
		case isAttachmentWord(lower):
			out = append(out, "has:attachment")
		case lower == "unread":
			out = append(out, "is:unread")
		case lower == "starred":
			out = append(out, "is:starred")
		case lower == "important":
			out = append(out, "is:important")
		default:
			out = append(out, token)
		}
	}
	return strings.TrimSpace(strings.Join(out, " "))
}

func splitHumanExportDestination(args []string) ([]string, string) {
	if len(args) < 2 {
		return args, ""
	}
	last := args[len(args)-1]
	prev := strings.ToLower(args[len(args)-2])
	if prev == "to" || prev == "into" || prev == "in" {
		return args[:len(args)-2], last
	}
	if len(args) >= 3 {
		marker := strings.ToLower(args[len(args)-3])
		kind := strings.ToLower(args[len(args)-2])
		if (marker == "to" || marker == "into" || marker == "in") && (kind == "folder" || kind == "directory" || kind == "dir") {
			return args[:len(args)-3], last
		}
	}
	return args, ""
}

func isFillerWord(value string) bool {
	switch value {
	case "email", "emails", "mail", "message", "messages", "that", "are", "matching", "for", "please":
		return true
	default:
		return false
	}
}

func nextIs(args []string, i int, want string) bool {
	return i+1 < len(args) && strings.EqualFold(args[i+1], want)
}

func isAttachmentWord(value string) bool {
	value = strings.ToLower(value)
	return value == "attachment" || value == "attachments" || value == "attached"
}

func humanDuration(value string) string {
	switch strings.ToLower(value) {
	case "day", "today":
		return "1d"
	case "week":
		return "7d"
	case "month":
		return "30d"
	case "year":
		return "365d"
	default:
		return value
	}
}

func printManualHelp(w io.Writer, cmd *cobra.Command) {
	if cmd == nil || !cmd.HasParent() {
		printRootManual(w)
		return
	}
	switch cmd.Name() {
	case "auth":
		printCommandManual(w, "AUTH", "gmail auth [client-json] [--no-window]", "Authorize Gmail read-only access and save credentials.", cmd, []string{
			"gmail auth",
			"gmail auth secrets/client_secret_....json",
			"gmail auth --no-window",
		})
	case "find":
		printCommandManual(w, "FIND", "gmail find [flags] [what to find]", "List matching Gmail messages.", cmd, []string{
			"gmail find --from alice@example.com --date 7d --subject invoice",
			"gmail find emails from alice@example.com about invoice newer than 30d",
			"gmail find --query 'from:alice@example.com newer_than:30d invoice'",
		})
	case "show":
		printCommandManual(w, "SHOW", "gmail show [flags] [what to show]", "Print matching messages as terminal-friendly text.", cmd, []string{
			"gmail show --unread --date today --number 1",
			"gmail show emails from ebay last week",
			"gmail show --query 'subject:(security alert) after:2026/01/01'",
		})
	case "export":
		printCommandManual(w, "EXPORT", "gmail export [flags] [what to export]", "Export messages into folders containing email.txt and attachments.", cmd, []string{
			"gmail export --attachments --date 30d --output exports",
			"gmail export emails with attachments from alice@example.com to folder exports",
			"gmail export --query 'has:attachment filename:pdf report' --output exports",
		})
	case "doctor":
		printCommandManual(w, "DOCTOR", "gmail doctor", "Check OAuth client config and saved token status.", cmd, []string{"gmail doctor", "gmail status"})
	case "logout":
		printCommandManual(w, "LOGOUT", "gmail logout", "Delete the saved OAuth token.", cmd, []string{"gmail logout"})
	case "completion":
		printCommandManual(w, "COMPLETION", "gmail completion bash|fish|powershell", "Generate shell completion scripts.", cmd, []string{
			"gmail completion powershell > gmail.ps1",
			"gmail completion fish > gmail.fish",
			"gmail completion bash > gmail.bash",
		})
	default:
		printRootManual(w)
	}
}

func printRootManual(w io.Writer) {
	fmt.Fprint(w, `NAME
  gmail - read and export Gmail as terminal-friendly text

SYNOPSIS
  gmail COMMAND [FLAGS] [QUERY]

COMMANDS
  auth        authorize Gmail read-only access
  find        list matching emails
  show        print matching emails as clean terminal text
  export      export emails plus attachments into folders
  doctor      check local credentials
  completion  generate bash, fish, or PowerShell completions
  logout      delete saved OAuth token

QUERY FLAGS
  --from, --to, --subject, --date, --after, --before
  --attachments, --unread, --starred, --important, --query
  --date accepts today, yesterday, last-week, 7d, YYYY-MM-DD, or START..END.

ALIASES
  login=auth, search/list=find, read/view=show, download/save=export, status=doctor
  Short aliases remain available: a, s, r, d.

EXAMPLES
  gmail find --from alice@example.com --date 7d --subject invoice
  gmail show --unread --date today --number 1
  gmail export --attachments --after 2026-06-01 --output exports
  gmail completion powershell > gmail.ps1

Use "gmail COMMAND --help" for command-specific help.
`)
}

func printCommandManual(w io.Writer, name string, synopsis string, description string, cmd *cobra.Command, examples []string) {
	fmt.Fprintf(w, "NAME\n  gmail %s - %s\n\n", strings.ToLower(name), description)
	fmt.Fprintf(w, "SYNOPSIS\n  %s\n\n", synopsis)
	if flagText := strings.TrimSpace(cmd.NonInheritedFlags().FlagUsagesWrapped(78)); flagText != "" {
		fmt.Fprintf(w, "FLAGS\n%s\n\n", indentLines(flagText, "  "))
	}
	if inheritedText := strings.TrimSpace(cmd.InheritedFlags().FlagUsagesWrapped(78)); inheritedText != "" {
		fmt.Fprintf(w, "GLOBAL FLAGS\n%s\n\n", indentLines(inheritedText, "  "))
	}
	if len(examples) > 0 {
		fmt.Fprintln(w, "EXAMPLES")
		for _, example := range examples {
			fmt.Fprintf(w, "  %s\n", example)
		}
	}
}

func indentLines(value string, prefix string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}
