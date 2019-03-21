// Package testlog creates a *log.Logger backed by *testing.T to ease logging
// in tests. This allows logs from components being tested to only be printed
// if the test fails (or the verbose flag is specified).
package testlog

import (
	"bytes"
	"io"
	"log"
	"os"

	hclog "github.com/hashicorp/go-hclog"
)

// UseStdout returns true if NOMAD_TEST_STDOUT=1 and sends logs to stdout.
func UseStdout() bool {
	return os.Getenv("NOMAD_TEST_STDOUT") == "1"
}

// LogPrinter is the methods of testing.T (or testing.B) needed by the test
// logger.
type LogPrinter interface {
	Logf(format string, args ...interface{})
}

// writer implements io.Writer on top of a Logger.
type writer struct {
	prefix string
	t      LogPrinter
}

// Write to an underlying Logger. Never returns an error.
func (w *writer) Write(p []byte) (n int, err error) {
	w.t.Logf("%s%s", w.prefix, p)
	return len(p), nil
}

// NewWriter creates a new io.Writer backed by a Logger.
func NewWriter(t LogPrinter) io.Writer {
	if UseStdout() {
		return os.Stdout
	}
	return &writer{t: t}
}

// NewPrefixWriter creates a new io.Writer backed by a Logger with a custom
// prefix per Write.
func NewPrefixWriter(t LogPrinter, prefix string) io.Writer {
	if UseStdout() {
		return &prefixStdout{[]byte(prefix)}
	}
	return &writer{prefix, t}
}

// New returns a new test logger. See https://golang.org/pkg/log/#New
func New(t LogPrinter, prefix string, flag int) *log.Logger {
	if UseStdout() {
		return log.New(os.Stdout, prefix, flag)
	}
	return log.New(&writer{t: t}, prefix, flag)
}

// WithPrefix returns a new test logger with the Lmicroseconds flag set.
func WithPrefix(t LogPrinter, prefix string) *log.Logger {
	return New(t, prefix, log.Lmicroseconds)
}

// Logger returns a new test logger with the Lmicroseconds flag set and no
// prefix.
func Logger(t LogPrinter) *log.Logger {
	return WithPrefix(t, "")
}

//HCLogger returns a new test hc-logger.
func HCLogger(t LogPrinter) hclog.Logger {
	opts := &hclog.LoggerOptions{
		Level:           hclog.Trace,
		Output:          NewWriter(t),
		IncludeLocation: true,
	}
	return hclog.New(opts)
}

type prefixStdout struct {
	prefix []byte
}

// Write to stdout with a prefix per call containing non-whitespace characters.
func (w *prefixStdout) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	// Skip prefix if only writing whitespace
	if len(bytes.TrimSpace(p)) > 0 {
		_, err := os.Stdout.Write(w.prefix)
		if err != nil {
			return 0, err
		}
	}

	return os.Stdout.Write(p)
}
