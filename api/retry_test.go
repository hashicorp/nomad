// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: MPL-2.0

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
		cm, err := NewClient(&Config{
			Address: server.URL,
			retryOptions: &retryOptions{
				delayBase:       10 * time.Millisecond,
				maxRetries:      10,
				maxBackoffDelay: 100 * time.Millisecond,
			},
		})
		must.NoError(t, err)

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
		cm, err := NewClient(&Config{
			Address: server.URL,
			retryOptions: &retryOptions{
				delayBase:  10 * time.Millisecond,
				maxRetries: 1,
			},
		})
		must.NoError(t, err)

		md, err := cm.retryPut(context.TODO(), "/endpoint/", nil, nil, &WriteOptions{})
		must.Error(t, err)
		must.Nil(t, md)

		must.Len(t, 2, mh.callsCounter)
	})
}

func Test_RetryPut_capped_base_too_big(t *testing.T) {
	t.Run("successfully retries until no error, delayed capped to 100ms", func(t *testing.T) {
		mh := mockHandler{
			callsCounter: []time.Time{},
		}

		server := httptest.NewServer(http.HandlerFunc(mh.Handle))
		cm, err := NewClient(&Config{
			Address: server.URL,
			retryOptions: &retryOptions{
				delayBase:       math.MaxInt64 * time.Nanosecond,
				maxRetries:      3,
				maxBackoffDelay: 200 * time.Millisecond,
			},
		})
		must.NoError(t, err)

		md, err := cm.retryPut(context.TODO(), "/endpoint", nil, nil, &WriteOptions{})
		must.Error(t, err)

		must.Len(t, 4, mh.callsCounter)

		must.Nil(t, md)
		must.Greater(t, cm.config.retryOptions.maxBackoffDelay, mh.callsCounter[1].Sub(mh.callsCounter[0]))
		must.Greater(t, cm.config.retryOptions.maxBackoffDelay, mh.callsCounter[2].Sub(mh.callsCounter[1]))
	})
}

func Test_isCallRetriable(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		expect     bool
	}{
		{"too many requests", http.StatusTooManyRequests, true},
		{"internal server error", http.StatusInternalServerError, true},
		{"not implemented", http.StatusNotImplemented, true},
		{"bad gateway", http.StatusBadGateway, true},
		{"service unavailable", http.StatusServiceUnavailable, true},
		{"network authentication required", http.StatusNetworkAuthenticationRequired, false},
		{"bad request", http.StatusBadRequest, false},
		{"not found", http.StatusNotFound, false},
		{"ok", http.StatusOK, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.expect, isCallRetriable(tc.statusCode))
		})
	}
}

// internalErrorHandler always responds with HTTP 500 and counts the calls it
// receives, so we can assert that those responses are retried.
type internalErrorHandler struct {
	callsCounter []time.Time
}

func (ih *internalErrorHandler) Handle(rw http.ResponseWriter, req *http.Request) {
	ih.callsCounter = append(ih.callsCounter, time.Now())
	http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func Test_RetryPut_retries_internal_server_error(t *testing.T) {
	ih := &internalErrorHandler{
		callsCounter: []time.Time{},
	}

	server := httptest.NewServer(http.HandlerFunc(ih.Handle))
	t.Cleanup(server.Close)

	cm, err := NewClient(&Config{
		Address: server.URL,
		retryOptions: &retryOptions{
			delayBase:       10 * time.Millisecond,
			maxRetries:      3,
			maxBackoffDelay: 100 * time.Millisecond,
		},
	})
	must.NoError(t, err)

	md, err := cm.retryPut(context.TODO(), "/endpoint", nil, nil, &WriteOptions{})
	must.Error(t, err)
	must.Nil(t, md)

	// maxRetries+1 attempts are expected because HTTP 500 is retriable.
	must.Len(t, 4, ih.callsCounter)
}
