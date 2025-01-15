// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"

	gsyslog "github.com/hashicorp/go-syslog"
	"github.com/hashicorp/logutils"
)

// levelPriority is used to map a log level to a syslog priority level. The
// level strings should match those described within LevelFilter except for
// "OFF" which disables syslog.
var levelPriority = map[string]gsyslog.Priority{
	"TRACE": gsyslog.LOG_DEBUG,
	"DEBUG": gsyslog.LOG_INFO,
	"INFO":  gsyslog.LOG_NOTICE,
	"WARN":  gsyslog.LOG_WARNING,
	"ERROR": gsyslog.LOG_ERR,
}

// getSysLogPriority returns the syslog priority value associated to the passed
// log level. If the log level does not have a known mapping, the notice
// priority is returned.
func getSysLogPriority(level string) gsyslog.Priority {
	priority, ok := levelPriority[level]
	if !ok {
		priority = gsyslog.LOG_NOTICE
	}
	return priority
}

// SyslogWrapper is used to cleanup log messages before
// writing them to a Syslogger. Implements the io.Writer
// interface.
type SyslogWrapper struct {
	l    gsyslog.Syslogger
	filt *logutils.LevelFilter
}

// Write is used to implement io.Writer
func (s *SyslogWrapper) Write(p []byte) (int, error) {
	// Skip syslog if the log level doesn't apply
	if !s.filt.Check(p) {
		return 0, nil
	}

	// Extract log level
	var level string
	afterLevel := p
	x := bytes.IndexByte(p, '[')
	if x >= 0 {
		y := bytes.IndexByte(p[x:], ']')
		if y >= 0 {
			level = string(p[x+1 : x+y])
			afterLevel = p[x+y+2:]
		}
	}

	// Attempt to write using the converted syslog priority.
	err := s.l.WriteLevel(getSysLogPriority(level), afterLevel)
	return len(p), err
}
