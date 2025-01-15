// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"runtime"
	"testing"

	gsyslog "github.com/hashicorp/go-syslog"
	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func Test_getSysLogPriority(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("Syslog not supported on Windows")
	}

	testCases := []struct {
		name                   string
		inputLogLevel          string
		expectedSyslogPriority gsyslog.Priority
	}{
		{
			name:                   "trace",
			inputLogLevel:          "TRACE",
			expectedSyslogPriority: gsyslog.LOG_DEBUG,
		},
		{
			name:                   "debug",
			inputLogLevel:          "DEBUG",
			expectedSyslogPriority: gsyslog.LOG_INFO,
		},
		{
			name:                   "info",
			inputLogLevel:          "INFO",
			expectedSyslogPriority: gsyslog.LOG_NOTICE,
		},
		{
			name:                   "warn",
			inputLogLevel:          "WARN",
			expectedSyslogPriority: gsyslog.LOG_WARNING,
		},
		{
			name:                   "error",
			inputLogLevel:          "ERROR",
			expectedSyslogPriority: gsyslog.LOG_ERR,
		},
		{
			name:                   "unknown",
			inputLogLevel:          "UNKNOWN",
			expectedSyslogPriority: gsyslog.LOG_NOTICE,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualPriority := getSysLogPriority(tc.inputLogLevel)
			must.Eq(t, tc.expectedSyslogPriority, actualPriority)
		})
	}
}

func Test_newSyslogWriter(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("Syslog not supported on Windows")
	}

	// Test the non-json syslog write handler generation.
	expectedSyslogWriter := newSyslogWriter(nil, false)
	_, ok := expectedSyslogWriter.(*syslogWrapper)
	must.True(t, ok)

	// Test the json syslog write handler generation.
	expectedJSONSyslogWriter := newSyslogWriter(nil, true)
	_, ok = expectedJSONSyslogWriter.(*syslogJSONWrapper)
	must.True(t, ok)
}

func Test_syslogWrapper(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("Syslog not supported on Windows")
	}

	testCases := []struct {
		name                   string
		inputLogLine           string
		expectedWrittenLogLine string
		expectedBytesWritten   int
		expectedPriority       gsyslog.Priority
	}{
		{
			name:                   "trace",
			inputLogLine:           `2025-01-14T09:29:56.747Z [TRACE] agent: i am a trace message`,
			expectedWrittenLogLine: `agent: i am a trace message`,
			expectedBytesWritten:   60,
			expectedPriority:       gsyslog.LOG_DEBUG,
		},
		{
			name:                   "debug",
			inputLogLine:           `2025-01-14T09:29:56.747Z [DEBUG] agent: i am a debug message`,
			expectedWrittenLogLine: `agent: i am a debug message`,
			expectedBytesWritten:   60,
			expectedPriority:       gsyslog.LOG_INFO,
		},
		{
			name:                   "info",
			inputLogLine:           `2025-01-14T09:29:56.747Z [INFO] agent: i am an info message`,
			expectedWrittenLogLine: `agent: i am an info message`,
			expectedBytesWritten:   59,
			expectedPriority:       gsyslog.LOG_NOTICE,
		},
		{
			name:                   "warn",
			inputLogLine:           `2025-01-14T09:29:56.747Z [WARN] agent: i am a warn message`,
			expectedWrittenLogLine: `agent: i am a warn message`,
			expectedBytesWritten:   58,
			expectedPriority:       gsyslog.LOG_WARNING,
		},
		{
			name:                   "error",
			inputLogLine:           `2025-01-14T09:29:56.747Z [ERROR] agent: i am an error message`,
			expectedWrittenLogLine: `agent: i am an error message`,
			expectedBytesWritten:   61,
			expectedPriority:       gsyslog.LOG_ERR,
		},
		{
			name:                   "no level",
			inputLogLine:           `2025-01-14T09:29:56.747Z agent: i am a message without a level`,
			expectedWrittenLogLine: `2025-01-14T09:29:56.747Z agent: i am a message without a level`,
			expectedBytesWritten:   62,
			expectedPriority:       gsyslog.LOG_NOTICE,
		},
	}

	// Generate our test backend, so we can easily read written log messages
	// back out.
	testSyslogBackend := testSysLogger{}
	syslogWriter := newSyslogWriter(&testSyslogBackend, false)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bytesWritten, err := syslogWriter.Write([]byte(tc.inputLogLine))
			must.NoError(t, err)
			must.Eq(t, tc.expectedBytesWritten, bytesWritten)
			must.Eq(t, tc.expectedWrittenLogLine, testSyslogBackend.msg)
			must.Eq(t, tc.expectedPriority, testSyslogBackend.pri)
		})
	}
}

func Test_syslogJSONWrapper(t *testing.T) {
	ci.Parallel(t)

	if runtime.GOOS == "windows" {
		t.Skip("Syslog not supported on Windows")
	}

	testCases := []struct {
		name                 string
		inputLogLine         string
		expectedBytesWritten int
		expectedPriority     gsyslog.Priority
	}{
		{
			name:                 "trace",
			inputLogLine:         `{"@level":"trace","@message":"i am a trace message","@module":"agent","@timestamp":"2025-01-14T08:54:26.245072Z"}`,
			expectedBytesWritten: 113,
			expectedPriority:     gsyslog.LOG_DEBUG,
		},
		{
			name:                 "debug",
			inputLogLine:         `{"@level":"debug","@message":"i am a debug message","@module":"agent","@timestamp":"2025-01-14T08:54:26.245072Z"}`,
			expectedBytesWritten: 113,
			expectedPriority:     gsyslog.LOG_INFO,
		},
		{
			name:                 "info",
			inputLogLine:         `{"@level":"info","@message":"i am an info message","@module":"agent","@timestamp":"2025-01-14T08:54:26.245072Z"}`,
			expectedBytesWritten: 112,
			expectedPriority:     gsyslog.LOG_NOTICE,
		},
		{
			name:                 "warn",
			inputLogLine:         `{"@level":"warn","@message":"i am a warn message","@module":"agent","@timestamp":"2025-01-14T08:54:26.245072Z"}`,
			expectedBytesWritten: 111,
			expectedPriority:     gsyslog.LOG_WARNING,
		},
		{
			name:                 "error",
			inputLogLine:         `{"@level":"error","@message":"i am an error message","@module":"agent","@timestamp":"2025-01-14T08:54:26.245072Z"}`,
			expectedBytesWritten: 114,
			expectedPriority:     gsyslog.LOG_ERR,
		},
		{
			name:                 "no level",
			inputLogLine:         `{"@message":"i am a message without a level","@module":"agent","@timestamp":"2025-01-14T08:54:26.245072Z"}`,
			expectedBytesWritten: 106,
			expectedPriority:     gsyslog.LOG_NOTICE,
		},
	}

	// Generate our test backend, so we can easily read written log messages
	// back out.
	testSyslogBackend := testSysLogger{}
	syslogWriter := newSyslogWriter(&testSyslogBackend, true)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bytesWritten, err := syslogWriter.Write([]byte(tc.inputLogLine))
			must.NoError(t, err)
			must.Eq(t, tc.expectedBytesWritten, bytesWritten)
			must.Eq(t, tc.inputLogLine, testSyslogBackend.msg)
			must.Eq(t, tc.expectedPriority, testSyslogBackend.pri)
		})
	}
}

// testSysLogger implements the gsyslog.Syslogger interface. It allows the
// tests to check written log lines.
type testSysLogger struct {
	msg string
	pri gsyslog.Priority
}

func (t *testSysLogger) WriteLevel(pri gsyslog.Priority, log []byte) error {
	_, err := t.Write(log)
	t.pri = pri
	return err
}

func (t *testSysLogger) Write(log []byte) (int, error) {
	t.msg = string(log)
	return len(log), nil
}

func (t *testSysLogger) Close() error { return nil }
