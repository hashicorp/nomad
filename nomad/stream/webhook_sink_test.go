package stream

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

var _ SinkWriter = &WebhookSink{}

func TestWebhookSink_Basic(t *testing.T) {
	received := make(chan struct{})

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var event structs.Events
		dec := json.NewDecoder(r.Body)
		require.NoError(t, dec.Decode(&event))
		require.Equal(t, "Deployment", string(event.Events[0].Topic))

		close(received)
	}))
	defer ts.Close()

	sink := mock.EventSink()
	sink.Address = ts.URL

	webhook, err := NewWebhookSink(sink)
	require.NoError(t, err)

	e := &structs.Events{
		Index: 1,
		Events: []structs.Event{
			{
				Topic: "Deployment",
			},
		},
	}
	webhook.Send(context.Background(), e)

	select {
	case <-received:
		// success
	case <-time.After(2 * time.Second):
		require.Fail(t, "expected test server to receive webhook")
	}
}
