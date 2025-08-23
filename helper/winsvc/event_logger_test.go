// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"io"
	"testing"

	"github.com/shoenig/test/must"
)

func testEventLogger(e Eventlog, l EventlogLevel) io.WriteCloser {
	return &eventLogger{
		level:  l,
		evtLog: e,
	}
}

func TestEventlogLevelFromString(t *testing.T) {
	t.Run("INFO", func(t *testing.T) {
		for _, val := range []string{"INFO", "info"} {
			l := EventlogLevelFromString(val)
			must.Eq(t, EVENTLOG_LEVEL_INFO, l)
		}
	})
	t.Run("WARN", func(t *testing.T) {
		for _, val := range []string{"WARN", "warn"} {
			l := EventlogLevelFromString(val)
			must.Eq(t, EVENTLOG_LEVEL_WARN, l)
		}
	})
	t.Run("ERROR", func(t *testing.T) {
		for _, val := range []string{"ERROR", "error"} {
			l := EventlogLevelFromString(val)
			must.Eq(t, EVENTLOG_LEVEL_ERROR, l)
		}
	})
}

func TestEventLogger(t *testing.T) {
	defaultmsgs := []string{
		"1970-01-01T16:27:16.116Z [INFO] Information line",
		"1970-01-01T16:27:16.116Z [WARN] Warning line",
		"1970-01-01T16:27:16.116Z [ERROR] Error line",
	}

	testCases := []struct {
		desc  string
		msgs  []string
		level EventlogLevel
		setup func(*MockEventlog)
	}{
		{
			desc:  "basic usage",
			level: EVENTLOG_LEVEL_INFO,
			setup: func(m *MockEventlog) {
				m.ExpectInfo(EventLogMessage, "Information line")
				m.ExpectWarning(EventLogMessage, "Warning line")
				m.ExpectError(EventLogMessage, "Error line")
			},
		},
		{
			desc:  "higher level",
			level: EVENTLOG_LEVEL_ERROR,
			setup: func(m *MockEventlog) {
				m.ExpectError(EventLogMessage, "Error line")
			},
		},
		{
			desc:  "debug and trace logs",
			level: EVENTLOG_LEVEL_INFO,
			setup: func(m *MockEventlog) {
				m.ExpectInfo(EventLogMessage, "Information line")
				m.ExpectWarning(EventLogMessage, "Warning line")
				m.ExpectError(EventLogMessage, "Error line")
			},
			msgs: append(defaultmsgs, []string{
				"[DEBUG] Debug line",
				"[TRACE] Trace line",
			}...),
		},
		{
			desc:  "with multi-line logs",
			level: EVENTLOG_LEVEL_INFO,
			setup: func(m *MockEventlog) {
				m.ExpectInfo(EventLogMessage, "Information line")
				m.ExpectWarning(EventLogMessage, "Warning line")
				m.ExpectError(EventLogMessage, "Error line")
				m.ExpectInfo(EventLogMessage, "Information log\nthat includes\nmultiple lines")
				m.ExpectWarning(EventLogMessage, "Warning log\nthat includes second line")
			},
			msgs: append(defaultmsgs, []string{
				"[INFO] Information log\nthat includes\nmultiple lines",
				"[WARN] Warning log\nthat includes second line",
			}...),
		},
	}

	for _, tc := range testCases {
		if len(tc.msgs) < 1 {
			tc.msgs = defaultmsgs
		}

		el := NewMockEventlog(t)
		tc.setup(el)
		eventLogger := testEventLogger(el, tc.level)

		for _, msg := range tc.msgs {
			eventLogger.Write([]byte(msg))
		}
		el.AssertExpectations()
	}
}
