// Package testlog creates a *log.Logger backed by testing.T to ease logging in
// tests.
package testlog

import (
	"log"
)

// TestLogger is the methods of testing.T (or testing.B) needed by the test
// logger.
type TestLogger interface {
	Logf(format string, args ...interface{})
}

type testWriter struct {
	t TestLogger
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	w.t.Logf(string(p))
	return len(p), nil
}

// New test logger. See https://golang.org/pkg/log/#New
func New(t TestLogger, prefix string, flag int) *log.Logger {
	return log.New(&testWriter{t}, prefix, flag)
}

// NewTest logger with "TEST" prefix and the Lmicroseconds flag.
func NewTest(t TestLogger) *log.Logger {
	return New(t, "TEST ", log.Lmicroseconds)
}
