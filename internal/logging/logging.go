package logging

import (
	"io"
	"log/slog"
	"os"
)

func New(debug bool, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stderr
	}
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}
