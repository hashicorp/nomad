package stream

import (
	"testing"

	"github.com/hashicorp/nomad/nomad/structs"

	"github.com/stretchr/testify/require"
)

func TestFilter_AllTopics(t *testing.T) {
	events := make([]structs.Event, 0, 5)
	events = append(events, structs.Event{Topic: "Test", Key: "One"}, structs.Event{Topic: "Test", Key: "Two"})

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*": {"*"},
		},
	}
	actual := filter(req, events)
	require.Equal(t, events, actual)

	// ensure new array was not allocated
	require.Equal(t, cap(actual), 5)
}

func TestFilter_AllKeys(t *testing.T) {
	events := make([]structs.Event, 0, 5)
	events = append(events, structs.Event{Topic: "Test", Key: "One"}, structs.Event{Topic: "Test", Key: "Two"})

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"Test": {"*"},
		},
	}
	actual := filter(req, events)
	require.Equal(t, events, actual)

	// ensure new array was not allocated
	require.Equal(t, cap(actual), 5)
}

func TestFilter_PartialMatch_Topic(t *testing.T) {
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

	require.Equal(t, cap(actual), 2)
}

func TestFilter_PartialMatch_Key(t *testing.T) {
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

	require.Equal(t, cap(actual), 1)
}

func TestFilter_NoMatch(t *testing.T) {
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

	require.Equal(t, cap(actual), 0)
}

func TestFilter_Namespace(t *testing.T) {
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

	require.Equal(t, cap(actual), 2)
}

func TestFilter_FilterKeys(t *testing.T) {
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

	require.Equal(t, cap(actual), 1)
}
