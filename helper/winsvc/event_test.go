// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package winsvc

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestNewEvent(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		event := NewEvent(EventServiceStarted)
		must.Eq(t, event.Kind(), EventServiceStarted)
		must.Eq(t, event.Level(), EVENTLOG_LEVEL_INFO)
		must.Eq(t, event.Message(), EventServiceStarted.String())
	})

	t.Run("WithEventMessage", func(t *testing.T) {
		event := NewEvent(EventServiceStarted, WithEventMessage("Custom service started message"))
		must.Eq(t, event.Kind(), EventServiceStarted)
		must.Eq(t, event.Level(), EVENTLOG_LEVEL_INFO)
		must.Eq(t, event.Message(), "Custom service started message")
	})

	t.Run("WithEventLevel", func(t *testing.T) {
		event := NewEvent(EventServiceStarted, WithEventLevel(EVENTLOG_LEVEL_ERROR))
		must.Eq(t, event.Kind(), EventServiceStarted)
		must.Eq(t, event.Level(), EVENTLOG_LEVEL_ERROR)
		must.Eq(t, event.Message(), EventServiceStarted.String())
	})

	t.Run("multiple options", func(t *testing.T) {
		event := NewEvent(EventServiceStopped,
			WithEventMessage("Custom service stopped message"),
			WithEventLevel(EVENTLOG_LEVEL_WARN),
		)
		must.Eq(t, event.Kind(), EventServiceStopped)
		must.Eq(t, event.Level(), EVENTLOG_LEVEL_WARN)
		must.Eq(t, event.Message(), "Custom service stopped message")
	})
}
