package api

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

type mockHandler struct {
	callsCounter []time.Time
}

func (mh *mockHandler) Handle(rw http.ResponseWriter, req *http.Request) {
	mh.callsCounter = append(mh.callsCounter, time.Now())

	// return a populated meta after 7 tries to test he retries stops after a
	// successful call
	if len(mh.callsCounter) < 7 {
		http.Error(rw, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}

	rw.WriteHeader(http.StatusOK)
	rw.Header().Set("Content-Type", "application/json")

	resp := &WriteMeta{}
	jsonResp, _ := json.Marshal(resp)

	rw.Write(jsonResp)
	return
}

func Test_RetryPut_multiple_calls(t *testing.T) {
	t.Run("successfully retries until no error, delayed capped to 100ms", func(t *testing.T) {
		mh := mockHandler{
			callsCounter: []time.Time{},
		}

		server := httptest.NewServer(http.HandlerFunc(mh.Handle))

		cm := &Client{
			httpClient: server.Client(),
			config: Config{
				Address: server.URL,
				retryOptions: &retryOptions{
					delayBase:       10 * time.Millisecond,
					maxRetries:      10,
					maxBackoffDelay: 100 * time.Millisecond,
				},
			},
		}

		md, err := cm.retryPut(context.TODO(), "/endpoint", nil, nil, &WriteOptions{})
		must.NoError(t, err)

		must.Len(t, 7, mh.callsCounter)

		must.NotNil(t, md)
		must.Greater(t, 10*time.Millisecond, mh.callsCounter[1].Sub(mh.callsCounter[0]))
		must.Greater(t, 20*time.Millisecond, mh.callsCounter[2].Sub(mh.callsCounter[1]))
		must.Greater(t, 40*time.Millisecond, mh.callsCounter[3].Sub(mh.callsCounter[2]))
		must.Greater(t, 80*time.Millisecond, mh.callsCounter[4].Sub(mh.callsCounter[3]))
		must.Greater(t, 100*time.Millisecond, mh.callsCounter[5].Sub(mh.callsCounter[4]))
		must.Greater(t, 100*time.Millisecond, mh.callsCounter[6].Sub(mh.callsCounter[5]))
	})
}

func Test_RetryPut_one_call(t *testing.T) {
	t.Run("successfully retries until no error, delayed capped to 100ms", func(t *testing.T) {
		mh := mockHandler{
			callsCounter: []time.Time{},
		}

		server := httptest.NewServer(http.HandlerFunc(mh.Handle))

		cm := &Client{
			httpClient: server.Client(),
			config: Config{
				Address: server.URL,
				retryOptions: &retryOptions{
					delayBase:  10 * time.Millisecond,
					maxRetries: 1,
				},
			},
		}

		md, err := cm.retryPut(context.TODO(), "/endpoint/", nil, nil, &WriteOptions{})
		must.Error(t, err)
		must.Nil(t, md)

		must.Len(t, 1, mh.callsCounter)
	})
}

func Test_RetryPut_capped_base_too_big(t *testing.T) {
	t.Run("successfully retries until no error, delayed capped to 100ms", func(t *testing.T) {
		mh := mockHandler{
			callsCounter: []time.Time{},
		}

		server := httptest.NewServer(http.HandlerFunc(mh.Handle))
		cm := &Client{
			httpClient: server.Client(),
			config: Config{
				Address: server.URL,
				retryOptions: &retryOptions{
					delayBase:       math.MaxInt64 * time.Nanosecond,
					maxRetries:      3,
					maxBackoffDelay: 200 * time.Millisecond,
				},
			},
		}

		md, err := cm.retryPut(context.TODO(), "/endpoint", nil, nil, &WriteOptions{})
		must.Error(t, err)

		must.Len(t, 3, mh.callsCounter)

		must.Nil(t, md)
		must.Greater(t, cm.config.retryOptions.maxBackoffDelay, mh.callsCounter[1].Sub(mh.callsCounter[0]))
		must.Greater(t, cm.config.retryOptions.maxBackoffDelay, mh.callsCounter[2].Sub(mh.callsCounter[1]))
	})
}
