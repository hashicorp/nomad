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

func TestWebhookSink_Basic(t *testing.T) {

	received := make(chan struct{})

	sub := &SubscribeRequest{
		Topics: map[structs.Topic][]string{
			"Deployment": {"*"},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var event structs.Events
		dec := json.NewDecoder(r.Body)
		require.NoError(t, dec.Decode(&event))
		require.Equal(t, "Deployment", string(event.Events[0].Topic))

		close(received)
	}))
	defer ts.Close()

	pub := NewEventBroker(ctx, EventBrokerCfg{EventBufferSize: 100})
	sCfg := &SinkCfg{
		Address: ts.URL,
	}

	sink, err := NewWebhookSink(sCfg, pub, sub)
	require.NoError(t, err)

	go func() {
		sink.Start(ctx)
	}()

	pub.Publish(&structs.Events{Index: 1,
		Events: []structs.Event{{Topic: "Deployment", Payload: mock.Deployment()}},
	})

	select {
	case <-received:
		// success
	case <-time.After(2 * time.Second):
		require.Fail(t, "expected test server to receive webhook")
	}

}
