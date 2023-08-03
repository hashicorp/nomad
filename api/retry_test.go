package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

type mockClient struct {
	callsCounter []time.Time
}

func (mc *mockClient) put(_ string, _, _ any, _ *WriteOptions) (*WriteMeta, error) {
	mc.callsCounter = append(mc.callsCounter, time.Now())

	// return a populated meta after 7 tries to test he retries stops after a
	// successful call
	if len(mc.callsCounter) < 7 {
		return nil, &UnexpectedResponseError{
			statusCode: http.StatusBadGateway,
		}
	}
	return &WriteMeta{}, nil
}

func Test_RetryPut(t *testing.T) {
	t.Run("successfully retries until no error, delayed capped to 100ms", func(t *testing.T) {

		mc := &mockClient{}
		rc := newRetryClient(mc, retryOptions{
			DelayBase:       10 * time.Millisecond,
			MaxRetries:      10,
			MaxBetweenCalls: 100 * time.Millisecond,
		})

		_, err := rc.retryPut(context.TODO(), "/endpoint/", nil, nil, &WriteOptions{})
		must.NoError(t, err)

		must.Len(t, 7, mc.callsCounter)

		must.Greater(t, 10*time.Millisecond, mc.callsCounter[1].Sub(mc.callsCounter[0]))
		must.Greater(t, 20*time.Millisecond, mc.callsCounter[2].Sub(mc.callsCounter[1]))
		must.Greater(t, 40*time.Millisecond, mc.callsCounter[3].Sub(mc.callsCounter[2]))
		must.Greater(t, 80*time.Millisecond, mc.callsCounter[4].Sub(mc.callsCounter[3]))
		must.Greater(t, 100*time.Millisecond, mc.callsCounter[5].Sub(mc.callsCounter[4]))
		must.Greater(t, 100*time.Millisecond, mc.callsCounter[6].Sub(mc.callsCounter[5]))
	})
	t.Run("successfully calls 1 time", func(t *testing.T) {
		mc := &mockClient{}
		rc := newRetryClient(mc, retryOptions{
			MaxRetries: 1,
		})

		_, err := rc.retryPut(context.TODO(), "/endpoint/", nil, nil, &WriteOptions{})
		must.Error(t, err)

		must.Len(t, 1, mc.callsCounter)
	})
}
