// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"regexp"
	"strings"
)

type EventlogLevel uint8

//go:generate stringer -trimprefix=EVENTLOG_LEVEL_ -output strings_event_logger.go -linecomment -type=EventlogLevel
const (
	EVENTLOG_LEVEL_UNKNOWN EventlogLevel = iota
	EVENTLOG_LEVEL_INFO
	EVENTLOG_LEVEL_WARN
	EVENTLOG_LEVEL_ERROR
)

// EventlogLevelFromString converts a log level string to the correct constant
func EventlogLevelFromString(level string) EventlogLevel {
	switch strings.ToUpper(level) {
	case EVENTLOG_LEVEL_INFO.String():
		return EVENTLOG_LEVEL_INFO
	case EVENTLOG_LEVEL_WARN.String():
		return EVENTLOG_LEVEL_WARN
	case EVENTLOG_LEVEL_ERROR.String():
		return EVENTLOG_LEVEL_ERROR
	}

	return EVENTLOG_LEVEL_UNKNOWN
}

var logPattern = regexp.MustCompile(`(?s)\[(ERROR|WARN|INFO)\] (.+)`)

type Eventlog interface {
	Info(uint32, string) error
	Warning(uint32, string) error
	Error(uint32, string) error
	Close() error
}

type eventLogger struct {
	evtLog Eventlog
	level  EventlogLevel
}

// Close closes the eventlog
func (e *eventLogger) Close() error {
	return e.evtLog.Close()
}

// Write writes logging message to the eventlog
func (e *eventLogger) Write(p []byte) (int, error) {
	matches := logPattern.FindStringSubmatch(string(p))

	// If no match was found, or the incorrect number of
	// elements detected then ignore
	if matches == nil || len(matches) != 3 {
		return len(p), nil
	}

	level := EventlogLevelFromString(matches[1])

	// If the detected level of the message isn't currently
	// allowed then ignore
	if !e.allowed(level) {
		return len(p), nil
	}

	// Still here so send the message to the eventlog
	switch level {
	case EVENTLOG_LEVEL_INFO:
		e.evtLog.Info(uint32(EventLogMessage), matches[2])
	case EVENTLOG_LEVEL_WARN:
		e.evtLog.Warning(uint32(EventLogMessage), matches[2])
	case EVENTLOG_LEVEL_ERROR:
		e.evtLog.Error(uint32(EventLogMessage), matches[2])
	}

	return len(p), nil
}

// Check if level is allowed
func (e *eventLogger) allowed(level EventlogLevel) bool {
	return level >= e.level
}

type nullEventlog struct{}

func (n *nullEventlog) Info(uint32, string) error {
	return nil
}

func (n *nullEventlog) Warning(uint32, string) error {
	return nil
}

func (n *nullEventlog) Error(uint32, string) error {
	return nil
}

func (n *nullEventlog) Close() error {
	return nil
}
