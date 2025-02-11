// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package stream

import (
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestFilter_AllTopics(t *testing.T) {
	ci.Parallel(t)

	events := make([]structs.Event, 0, 5)
	events = append(events, structs.Event{Topic: "Test", Key: "One"}, structs.Event{Topic: "Test", Key: "Two"})

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*": {"*"},
		},
	}
	actual := filter(req, events)
	require.Equal(t, events, actual)
}

func TestFilter_AllKeys(t *testing.T) {
	ci.Parallel(t)

	events := make([]structs.Event, 0, 5)
	events = append(events, structs.Event{Topic: "Test", Key: "One"}, structs.Event{Topic: "Test", Key: "Two"})

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"Test": {"*"},
		},
	}
	actual := filter(req, events)
	require.Equal(t, events, actual)
}

func TestFilter_PartialMatch_Topic(t *testing.T) {
	ci.Parallel(t)

	event1 := structs.Event{Topic: "Test", Key: "One"}
	event2 := structs.Event{Topic: "Test", Key: "Two"}
	event3 := structs.Event{Topic: "Exclude", Key: "Two"}
	events := []structs.Event{event1, event2, event3}

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"Test": {"*"},
		},
	}
	actual := filter(req, events)
	expected := []structs.Event{event1, event2}
	require.Equal(t, expected, actual)

	require.Equal(t, 2, cap(actual))
}

func TestFilter_Match_TopicAll_SpecificKey(t *testing.T) {
	ci.Parallel(t)

	event1 := structs.Event{Topic: "Match", Key: "Two"}
	event2 := structs.Event{Topic: "NoMatch", Key: "One"}
	event3 := structs.Event{Topic: "OtherMatch", Key: "Two"}
	events := []structs.Event{event1, event2, event3}

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*": {"Two"},
		},
	}

	actual := filter(req, events)
	expected := []structs.Event{event1, event3}
	require.Equal(t, expected, actual)
}

func TestFilter_Match_TopicAll_SpecificKey_Plus(t *testing.T) {
	ci.Parallel(t)

	event1 := structs.Event{Topic: "FirstTwo", Key: "Two"}
	event2 := structs.Event{Topic: "Test", Key: "One"}
	event3 := structs.Event{Topic: "SecondTwo", Key: "Two"}
	events := []structs.Event{event1, event2, event3}

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*":    {"Two"},
			"Test": {"One"},
		},
	}

	actual := filter(req, events)
	expected := []structs.Event{event1, event2, event3}
	require.Equal(t, expected, actual)
}

func TestFilter_PartialMatch_Key(t *testing.T) {
	ci.Parallel(t)

	event1 := structs.Event{Topic: "Test", Key: "One"}
	event2 := structs.Event{Topic: "Test", Key: "Two"}
	events := []structs.Event{event1, event2}

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"Test": {"One"},
		},
	}
	actual := filter(req, events)
	expected := []structs.Event{event1}
	require.Equal(t, expected, actual)

	require.Equal(t, 1, cap(actual))
}

func TestFilter_NoMatch(t *testing.T) {
	ci.Parallel(t)

	events := make([]structs.Event, 0, 5)
	events = append(events, structs.Event{Topic: "Test", Key: "One"}, structs.Event{Topic: "Test", Key: "Two"})

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"NodeEvents": {"*"},
			"Test":       {"Highly-Specific-Key"},
		},
	}
	actual := filter(req, events)
	var expected []structs.Event
	require.Equal(t, expected, actual)

	require.Equal(t, 0, cap(actual))
}

func TestFilter_Namespace(t *testing.T) {
	ci.Parallel(t)

	event1 := structs.Event{Topic: "Test", Key: "One", Namespace: "foo"}
	event2 := structs.Event{Topic: "Test", Key: "Two", Namespace: "foo"}
	event3 := structs.Event{Topic: "Test", Key: "Two", Namespace: "bar"}
	events := []structs.Event{event1, event2, event3}

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*": {"*"},
		},
		Namespaces: []string{"foo"},
	}
	actual := filter(req, events)
	// expect namespace "bar" to be filtered out
	expected := []structs.Event{event1, event2}
	require.Equal(t, expected, actual)
	require.Equal(t, 2, cap(actual))
}

func TestFilter_FilterKeys(t *testing.T) {
	ci.Parallel(t)

	event1 := structs.Event{Topic: "Test", Key: "One", FilterKeys: []string{"extra-key"}}
	event2 := structs.Event{Topic: "Test", Key: "Two"}
	event3 := structs.Event{Topic: "Test", Key: "Two"}
	events := []structs.Event{event1, event2, event3}

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"Test": {"extra-key"},
		},
	}
	actual := filter(req, events)
	expected := []structs.Event{event1}
	require.Equal(t, expected, actual)

	require.Equal(t, 1, cap(actual))
}
