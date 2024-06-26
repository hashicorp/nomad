package main

import (
	"log/slog"
	"os"
	"testing"
)

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(os.Stderr,
		&slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}))
}
