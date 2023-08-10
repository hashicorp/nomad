// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"os"
	"runtime"
	"testing"

	gsyslog "github.com/hashicorp/go-syslog"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/nomad/ci"
)

func TestSyslogFilter(t *testing.T) {
	ci.Parallel(t)
	if runtime.GOOS == "windows" {
		t.Skip("Syslog not supported on Windows")
	}
	if os.Getenv("TRAVIS") == "true" {
		t.Skip("Syslog not supported on travis-ci")
	}

	l, err := gsyslog.NewLogger(gsyslog.LOG_NOTICE, "LOCAL0", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	filt := LevelFilter()
	filt.MinLevel = logutils.LogLevel("INFO")

	s := &SyslogWrapper{l, filt}
	n, err := s.Write([]byte("[INFO] test"))
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if n == 0 {
		t.Fatalf("should have logged")
	}

	n, err = s.Write([]byte("[DEBUG] test"))
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if n != 0 {
		t.Fatalf("should not have logged")
	}
}
