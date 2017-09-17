// Package testlog creates a *log.Logger backed by testing.T to ease logging in
// tests.
package testlog

import (
	"io"
	"log"
)

// Logger is the methods of testing.T (or testing.B) needed by the test
// logger.
type Logger interface {
	Logf(format string, args ...interface{})
}

// writer implements io.Writer on top of a Logger.
type writer struct {
	t Logger
}

// Write to an underlying Logger. Never returns an error.
func (w *writer) Write(p []byte) (n int, err error) {
	w.t.Logf(string(p))
	return len(p), nil
}

// NewWriter creates a new io.Writer backed by a Logger.
func NewWriter(t Logger) io.Writer {
	return &writer{t}
}

// NewLog returns a new test logger. See https://golang.org/pkg/log/#New
func NewLog(t Logger, prefix string, flag int) *log.Logger {
	return log.New(&writer{t}, prefix, flag)
}

// New logger with "TEST" prefix and the Lmicroseconds flag.
func New(t Logger) *log.Logger {
	return NewLog(t, "TEST ", log.Lmicroseconds)
}
