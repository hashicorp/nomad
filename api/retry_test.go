package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

func Test_RetryPut_multiple_calls(t *testing.T) {
	t.Run("successfully retries until no error, delayed capped to 100ms", func(t *testing.T) {
		callsCounter := []time.Time{}

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			callsCounter = append(callsCounter, time.Now())

			// return a populated meta after 7 tries to test he retries stops after a
			// successful call
			if len(callsCounter) < 7 {
				http.Error(rw, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
				return
			}

			rw.WriteHeader(http.StatusOK)
			rw.Header().Set("Content-Type", "application/json")

			resp := &WriteMeta{}
			jsonResp, _ := json.Marshal(resp)

			rw.Write(jsonResp)
			return
		}))

		cm := &Client{
			httpClient: server.Client(),
			config: Config{
				Address: server.URL,
				retryOptions: &retryOptions{
					delayBase:       10 * time.Millisecond,
					maxRetries:      10,
					maxBetweenCalls: 100 * time.Millisecond,
				},
			},
		}

		md, err := cm.retryPut(context.TODO(), "/endpoint", nil, nil, &WriteOptions{})
		must.NoError(t, err)

		must.Len(t, 7, callsCounter)

		must.NotNil(t, md)
		must.Greater(t, 10*time.Millisecond, callsCounter[1].Sub(callsCounter[0]))
		must.Greater(t, 20*time.Millisecond, callsCounter[2].Sub(callsCounter[1]))
		must.Greater(t, 40*time.Millisecond, callsCounter[3].Sub(callsCounter[2]))
		must.Greater(t, 80*time.Millisecond, callsCounter[4].Sub(callsCounter[3]))
		must.Greater(t, 100*time.Millisecond, callsCounter[5].Sub(callsCounter[4]))
		must.Greater(t, 100*time.Millisecond, callsCounter[6].Sub(callsCounter[5]))
	})
}

func Test_RetryPut_one_call(t *testing.T) {
	t.Run("successfully retries until no error, delayed capped to 100ms", func(t *testing.T) {
		callsCounter := []time.Time{}

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			callsCounter = append(callsCounter, time.Now())

			// return a populated meta after 7 tries to test he retries stops after a
			// successful call
			if len(callsCounter) < 7 {
				http.Error(rw, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
				return
			}

			rw.WriteHeader(http.StatusOK)
			rw.Header().Set("Content-Type", "application/json")

			resp := &WriteMeta{}
			jsonResp, _ := json.Marshal(resp)

			rw.Write(jsonResp)
			return
		}))

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

		must.Len(t, 1, callsCounter)
	})
}
