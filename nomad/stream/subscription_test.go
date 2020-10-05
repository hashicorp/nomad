package stream

import (
	"github.com/hashicorp/nomad/nomad/structs"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubscription(t *testing.T) {

}

func TestFilter_AllTopics(t *testing.T) {
	events := make([]structs.Event, 0, 5)
	events = append(events, structs.Event{Topic: "Test", Key: "One"}, structs.Event{Topic: "Test", Key: "Two"})

	req := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"*": []string{"*"},
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
			"Test": []string{"*"},
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
			"Test": []string{"*"},
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
			"Test": []string{"One"},
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
			"NodeEvents": []string{"*"},
			"Test":       []string{"Highly-Specific-Key"},
		},
	}
	actual := filter(req, events)
	var expected []structs.Event
	require.Equal(t, expected, actual)

	require.Equal(t, cap(actual), 0)
}
