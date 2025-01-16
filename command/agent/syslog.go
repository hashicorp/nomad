// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"io"
	"regexp"
	"strings"

	gsyslog "github.com/hashicorp/go-syslog"
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

// newSyslogWriter generates a new syslog wrapper depending on whether the
// agent is logging in JSON format.
func newSyslogWriter(sysLogger gsyslog.Syslogger, json bool) io.Writer {
	if json {
		return &syslogJSONWrapper{logger: sysLogger}
	} else {
		return &syslogWrapper{l: sysLogger}
	}
}

// SyslogWrapper is used to cleanup log messages before
// writing them to a Syslogger. Implements the io.Writer
// interface.
type syslogWrapper struct {
	l gsyslog.Syslogger
}

// Write is used to implement io.Writer.
//
// Nomad's syslog is fed by go-hclog which is responsible for performing the
// log level filtering. It is not needed here.
func (s *syslogWrapper) Write(p []byte) (int, error) {

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

var (
	// jsonLogLineLevelRegex is used to find the log level key/value entry
	// within a JSON log line. It will match string entries such as
	// `"@level":"debug",`, so we can pull these out for syslog capabilities.
	jsonLogLineLevelRegex = regexp.MustCompile(`"@level":"\w+",`)
)

// syslogJSONWrapper is a syslog writer for Nomad logs when the operator has
// enabled the JSON logging format.
type syslogJSONWrapper struct {
	logger gsyslog.Syslogger
}

// Write is used to implement io.Writer. It dissects the passed JSON log line,
// identifying the log level and removing the contextual entry, before
// performing the syslog write.
//
// Nomad's syslog is fed by go-hclog which is responsible for performing the
// log level filtering. It is not needed here.
func (s *syslogJSONWrapper) Write(logBytes []byte) (int, error) {

	// Find the start and finish index of the regex match, so we know where in
	// the byte array the level contextual entry is.
	indexes := jsonLogLineLevelRegex.FindAllIndex(logBytes, 1)

	// If the indexes are not what we expected, write the log line with the
	// notice level. It's better to have a log line at the incorrect level
	// than have a log line saying we couldn't write a log line.
	if len(indexes) != 1 || len(indexes[0]) != 2 {
		return len(logBytes), s.logger.WriteLevel(gsyslog.LOG_NOTICE, logBytes)
	}

	// Pull the log level from the message using the identified indexes and
	// knowledge of the JSON formatting from go-hclog.
	level := strings.ToTitle(string(logBytes[indexes[0][0]+10 : indexes[0][1]-2]))

	// Attempt to write using the converted syslog priority.
	return len(logBytes), s.logger.WriteLevel(getSysLogPriority(level), logBytes)
}
