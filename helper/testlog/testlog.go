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

// LogPrinter is the methods of testing.T (or testing.B) needed by the test
// logger.
type LogPrinter interface {
	Logf(format string, args ...interface{})
}

// NewWriter creates a new io.Writer backed by a Logger.
func NewWriter(t LogPrinter) io.Writer {
	return os.Stderr
}

// NewPrefixWriter creates a new io.Writer backed by a Logger with a custom
// prefix per Write.
func NewPrefixWriter(t LogPrinter, prefix string) io.Writer {
	return &prefixStderr{[]byte(prefix)}
}

// New returns a new test logger. See https://golang.org/pkg/log/#New
func New(t LogPrinter, prefix string, flag int) *log.Logger {
	return log.New(os.Stderr, prefix, flag)
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
func HCLogger(t LogPrinter) hclog.InterceptLogger {
	level := hclog.Trace
	envLogLevel := os.Getenv("NOMAD_TEST_LOG_LEVEL")
	if envLogLevel != "" {
		level = hclog.LevelFromString(envLogLevel)
	}
	opts := &hclog.LoggerOptions{
		Level:           level,
		Output:          os.Stderr,
		IncludeLocation: true,
	}
	return hclog.NewInterceptLogger(opts)
}

type prefixStderr struct {
	prefix []byte
}

// Write to stdout with a prefix per call containing non-whitespace characters.
func (w *prefixStderr) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	// Skip prefix if only writing whitespace
	if len(bytes.TrimSpace(p)) == 0 {
		return os.Stderr.Write(p)
	}

	// decrease likely hood of partial line writes that may mess up test
	// indicator success detection
	buf := make([]byte, 0, len(w.prefix)+len(p))
	buf = append(buf, w.prefix...)
	buf = append(buf, p...)

	return os.Stderr.Write(buf)
}
