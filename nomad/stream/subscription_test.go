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

	events := make([]structs.Event, 0, 5)
	events = append(events, structs.Event{Topic: "Test", Key: "One"}, structs.Event{Topic: "Test", Key: "Two"}, structs.Event{Topic: "Exclude", Key: "Two"})

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"Test": {"*"},
		},
	}
	actual := filter(req, events)
	expected := []structs.Event{{Topic: "Test", Key: "One"}, {Topic: "Test", Key: "Two"}}
	require.Equal(t, expected, actual)

	require.Equal(t, 2, cap(actual))
}

func TestFilter_Match_TopicAll_SpecificKey(t *testing.T) {
	ci.Parallel(t)

	events := []structs.Event{
		{Topic: "Match", Key: "Two"},
		{Topic: "NoMatch", Key: "One"},
		{Topic: "OtherMatch", Key: "Two"},
	}

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*": {"Two"},
		},
	}

	actual := filter(req, events)
	expected := []structs.Event{
		{Topic: "Match", Key: "Two"},
		{Topic: "OtherMatch", Key: "Two"},
	}
	require.Equal(t, expected, actual)
}

func TestFilter_Match_TopicAll_SpecificKey_Plus(t *testing.T) {
	ci.Parallel(t)

	events := []structs.Event{
		{Topic: "FirstTwo", Key: "Two"},
		{Topic: "Test", Key: "One"},
		{Topic: "SecondTwo", Key: "Two"},
	}

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*":    {"Two"},
			"Test": {"One"},
		},
	}

	actual := filter(req, events)
	expected := []structs.Event{
		{Topic: "FirstTwo", Key: "Two"},
		{Topic: "Test", Key: "One"},
		{Topic: "SecondTwo", Key: "Two"},
	}
	require.Equal(t, expected, actual)
}

func TestFilter_PartialMatch_Key(t *testing.T) {
	ci.Parallel(t)

	events := make([]structs.Event, 0, 5)
	events = append(events, structs.Event{Topic: "Test", Key: "One"}, structs.Event{Topic: "Test", Key: "Two"})

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"Test": {"One"},
		},
	}
	actual := filter(req, events)
	expected := []structs.Event{{Topic: "Test", Key: "One"}}
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

	events := make([]structs.Event, 0, 5)
	events = append(events, structs.Event{Topic: "Test", Key: "One", Namespace: "foo"}, structs.Event{Topic: "Test", Key: "Two"}, structs.Event{Topic: "Test", Key: "Two", Namespace: "bar"})

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*": {"*"},
		},
		Namespace: "foo",
	}
	actual := filter(req, events)
	expected := []structs.Event{
		{Topic: "Test", Key: "One", Namespace: "foo"},
		{Topic: "Test", Key: "Two"},
	}
	require.Equal(t, expected, actual)

	require.Equal(t, 2, cap(actual))
}

func TestFilter_NamespaceAll(t *testing.T) {
	ci.Parallel(t)

	events := make([]structs.Event, 0, 5)
	events = append(events,
		structs.Event{Topic: "Test", Key: "One", Namespace: "foo"},
		structs.Event{Topic: "Test", Key: "Two", Namespace: "bar"},
		structs.Event{Topic: "Test", Key: "Three", Namespace: "default"},
	)

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*": {"*"},
		},
		Namespace: "*",
	}
	actual := filter(req, events)
	expected := []structs.Event{
		{Topic: "Test", Key: "One", Namespace: "foo"},
		{Topic: "Test", Key: "Two", Namespace: "bar"},
		{Topic: "Test", Key: "Three", Namespace: "default"},
	}
	require.Equal(t, expected, actual)
}

func TestFilter_FilterKeys(t *testing.T) {
	ci.Parallel(t)

	events := make([]structs.Event, 0, 5)
	events = append(events, structs.Event{Topic: "Test", Key: "One", FilterKeys: []string{"extra-key"}}, structs.Event{Topic: "Test", Key: "Two"}, structs.Event{Topic: "Test", Key: "Two"})

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"Test": {"extra-key"},
		},
		Namespace: "foo",
	}
	actual := filter(req, events)
	expected := []structs.Event{
		{Topic: "Test", Key: "One", FilterKeys: []string{"extra-key"}},
	}
	require.Equal(t, expected, actual)

	require.Equal(t, 1, cap(actual))
}
