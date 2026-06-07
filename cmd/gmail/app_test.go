package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuthWithoutClientConfigExplainsBrowserSetup(t *testing.T) {
	isolateCredentialEnvironment(t)
	var out bytes.Buffer
	runner := Runner{Stdout: &out, Stderr: &out}
	err := runner.Run(context.Background(), []string{"auth", "--no-window"})
	if err == nil {
		t.Fatal("expected auth without client config to fail")
	}
	if !strings.Contains(err.Error(), "first-time setup") || !strings.Contains(err.Error(), "gmail auth secrets/client_secret_") {
		t.Fatalf("error should explain first setup command, got: %v", err)
	}
	if strings.Contains(err.Error(), "opened in your browser") {
		t.Fatalf("--no-window error should not claim browser setup pages were opened, got: %v", err)
	}
	if strings.Contains(out.String(), "Opened Gmail authorization") {
		t.Fatalf("auth should not open terminal before config exists; output: %s", out.String())
	}
}

func isolateCredentialEnvironment(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	cwd := filepath.Join(tmp, "cwd")
	home := filepath.Join(tmp, "home")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatalf("create isolated cwd: %v", err)
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatalf("create isolated home: %v", err)
	}
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("isolate cwd: %v", err)
	}
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func TestHelpShowsSimpleWorkflow(t *testing.T) {
	var out bytes.Buffer
	runner := Runner{Stdout: &out, Stderr: &out}
	if err := runner.Run(context.Background(), []string{"help"}); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{"gmail auth [client-json]", "gmail find [flags] [what to find]", "gmail show [flags] [what to show]", "gmail export [flags] [what]", "gmail doctor", "gmail completion bash|fish|powershell", "--date VALUE", "search/list=find", "read/view=show"} {
		if !strings.Contains(text, want) {
			t.Fatalf("help missing %q:\n%s", want, text)
		}
	}
}

func TestHumanGmailQuery(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "from about newer", args: []string{"emails", "from", "alice@example.com", "about", "invoice", "newer", "than", "30d"}, want: "from:alice@example.com invoice newer_than:30d"},
		{name: "last week unread", args: []string{"unread", "emails", "from", "ebay", "last", "week"}, want: "is:unread from:ebay newer_than:7d"},
		{name: "attachments", args: []string{"messages", "with", "attachments"}, want: "has:attachment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := humanGmailQuery(tt.args); got != tt.want {
				t.Fatalf("humanGmailQuery(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestSplitHumanExportDestination(t *testing.T) {
	t.Parallel()
	query, out := splitHumanExportDestination([]string{"emails", "with", "attachments", "to", "folder", "exports"})
	if out != "exports" || strings.Join(query, " ") != "emails with attachments" {
		t.Fatalf("splitHumanExportDestination returned query=%v out=%q", query, out)
	}
}

func TestDateQueryTokens(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 7, 17, 0, 0, 0, time.UTC)
	tests := []struct {
		value string
		want  []string
	}{
		{value: "today", want: []string{"after:2026/06/07", "before:2026/06/08"}},
		{value: "yesterday", want: []string{"after:2026/06/06", "before:2026/06/07"}},
		{value: "2026-06-01", want: []string{"after:2026/06/01", "before:2026/06/02"}},
		{value: "2026-06-01..2026-06-07", want: []string{"after:2026/06/01", "before:2026/06/08"}},
		{value: "7d", want: []string{"newer_than:7d"}},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := dateQueryTokens(tt.value, now)
			if strings.Join(got, " ") != strings.Join(tt.want, " ") {
				t.Fatalf("dateQueryTokens(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestQueryFlagsArgs(t *testing.T) {
	t.Parallel()
	q := queryFlags{from: "alice@example.com", date: "7d", attachments: true, unread: true, subject: "invoice"}
	got := strings.Join(q.args([]string{"urgent"}), " ")
	want := "urgent from:alice@example.com subject:(invoice) newer_than:7d has:attachment is:unread"
	if got != want {
		t.Fatalf("queryFlags args = %q, want %q", got, want)
	}
}
