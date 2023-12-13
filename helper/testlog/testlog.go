// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package testlog creates a *log.Logger backed by *testing.T to ease logging
// in tests. This allows logs from components being tested to only be printed
// if the test fails (or the verbose flag is specified).
package testlog

import (
	"bytes"
	"fmt"
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

// Logger returns a new test logger with the Lmicroseconds flag set and no prefix.
//
// Note: only use this where HCLogger cannot be used (i.e. RPC yamux configuration).
func Logger(t LogPrinter) *log.Logger {
	return WithPrefix(t, "")
}

// HCLogger returns a new test hc-logger.
//
// Default log level is TRACE. Set NOMAD_TEST_LOG_LEVEL for custom log level.
func HCLogger(t LogPrinter) hclog.InterceptLogger {
	logger, _ := HCLoggerNode(t, -1)
	return logger
}

// HCLoggerTestLevel returns the level in which hc log should emit logs.
//
// Default log level is TRACE. Set NOMAD_TEST_LOG_LEVEL for custom log level.
func HCLoggerTestLevel() hclog.Level {
	level := hclog.Trace
	envLogLevel := os.Getenv("NOMAD_TEST_LOG_LEVEL")
	if envLogLevel != "" {
		level = hclog.LevelFromString(envLogLevel)
	}
	return level
}

// HCLoggerNode returns a new hc-logger, but with a prefix indicating the node number
// on each log line. Useful for TestServer in tests with more than one server.
//
// Default log level is TRACE. Set NOMAD_TEST_LOG_LEVEL for custom log level.
func HCLoggerNode(t LogPrinter, node int32) (hclog.InterceptLogger, io.Writer) {
	var output io.Writer = os.Stderr
	if node > -1 {
		output = NewPrefixWriter(t, fmt.Sprintf("node-%03d ", node))
	}
	opts := &hclog.LoggerOptions{
		Level:           HCLoggerTestLevel(),
		Output:          output,
		IncludeLocation: true,
	}
	return hclog.NewInterceptLogger(opts), output
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
