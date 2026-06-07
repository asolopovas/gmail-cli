package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestAuthWithoutClientConfigExplainsBrowserSetup(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer
	runner := Runner{Stdout: &out, Stderr: &out}
	err := runner.Run(context.Background(), []string{"auth", "--no-window"})
	if err == nil {
		t.Fatal("expected auth without client config to fail")
	}
	if !strings.Contains(err.Error(), "first-time setup") || !strings.Contains(err.Error(), "gmail auth ~/Downloads/client_secret_") {
		t.Fatalf("error should explain first setup command, got: %v", err)
	}
	if strings.Contains(out.String(), "Opened Gmail authorization") {
		t.Fatalf("auth should not open terminal before config exists; output: %s", out.String())
	}
}

func TestHelpShowsSimpleWorkflow(t *testing.T) {
	var out bytes.Buffer
	runner := Runner{Stdout: &out, Stderr: &out}
	if err := runner.Run(context.Background(), []string{"help"}); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{"gmail auth [client-json]", "gmail s [-n N] [query]", "gmail r [-n N] [query]", "gmail d [-n N] [-o DIR] [query]"} {
		if !strings.Contains(text, want) {
			t.Fatalf("help missing %q:\n%s", want, text)
		}
	}
}
