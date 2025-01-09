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

func TestSyslogFilter(t *testing.T) {
	ci.Parallel(t)
	if runtime.GOOS == "windows" {
		t.Skip("Syslog not supported on Windows")
	}

	l, err := gsyslog.NewLogger(gsyslog.LOG_NOTICE, "LOCAL0", "nomad")
	must.NoError(t, err)

	filt := LevelFilter()
	filt.MinLevel = "INFO"

	s := &SyslogWrapper{l, filt}
	n, err := s.Write([]byte("[INFO] test"))
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	must.NonZero(t, n)

	n, err = s.Write([]byte("[DEBUG] test"))
	must.NoError(t, err)
	must.Zero(t, n)
}
