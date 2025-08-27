// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestNewEvent(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		event := NewEvent(EventServiceReady)
		must.Eq(t, EventServiceReady, event.Kind())
		must.Eq(t, EVENTLOG_LEVEL_INFO, event.Level())
		must.Eq(t, EventServiceReady.String(), event.Message())
	})

	t.Run("WithEventMessage", func(t *testing.T) {
		event := NewEvent(EventServiceReady, WithEventMessage("Custom service ready message"))
		must.Eq(t, EventServiceReady, event.Kind())
		must.Eq(t, EVENTLOG_LEVEL_INFO, event.Level())
		must.Eq(t, "Custom service ready message", event.Message())
	})

	t.Run("WithEventLevel", func(t *testing.T) {
		event := NewEvent(EventServiceReady, WithEventLevel(EVENTLOG_LEVEL_ERROR))
		must.Eq(t, EventServiceReady, event.Kind())
		must.Eq(t, EVENTLOG_LEVEL_ERROR, event.Level())
		must.Eq(t, EventServiceReady.String(), event.Message())
	})

	t.Run("multiple options", func(t *testing.T) {
		event := NewEvent(EventServiceStopped,
			WithEventMessage("Custom service stopped message"),
			WithEventLevel(EVENTLOG_LEVEL_WARN),
		)
		must.Eq(t, EventServiceStopped, event.Kind())
		must.Eq(t, EVENTLOG_LEVEL_WARN, event.Level())
		must.Eq(t, "Custom service stopped message", event.Message())
	})
}
