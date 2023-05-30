// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

const mockNamespaceBody = `{"Capabilities":null,"CreateIndex":1,"Description":"Default shared namespace","Hash":"C7UbjDwBK0dK8wQq7Izg7SJIzaV+lIo2X7wRtzY3pSw=","Meta":null,"ModifyIndex":1,"Name":"default","Quota":""}`

func TestUnexpectedResponseError(t *testing.T) {
	testutil.Parallel(t)
	a := mockserver(t)
	cfg := api.DefaultConfig()
	cfg.Address = a

	c, e := api.NewClient(cfg)
	must.NoError(t, e)

	type testCase struct {
		testFunc   func()
		statusCode *int
		body       *int
	}

	// ValidateServer ensures that the mock server handles the default namespace
	// correctly. This ensures that the routing rule for this path is at least
	// correct and that the mock server is passing its address to the client
	// properly.
	t.Run("ValidateServer", func(t *testing.T) {
		n, _, err := c.Namespaces().Info("default", nil)
		must.NoError(t, err)
		var ns api.Namespace
		err = unmock(t, mockNamespaceBody, &ns)
		must.NoError(t, err)
		must.Eq(t, ns, *n)
	})

	// WrongStatus tests that an UnexpectedResponseError is generated and filled
	// with the correct data when a response code that the API client wasn't
	// looking for is returned by the server.
	t.Run("WrongStatus", func(t *testing.T) {
		testutil.Parallel(t)
		n, _, err := c.Namespaces().Info("badStatus", nil)
		must.Nil(t, n)
		must.Error(t, err)
		t.Logf("err: %v", err)

		ure, ok := err.(api.UnexpectedResponseError)
		must.True(t, ok)

		must.True(t, ure.HasStatusCode())
		must.Eq(t, http.StatusAccepted, ure.StatusCode())

		must.True(t, ure.HasStatusText())
		must.Eq(t, http.StatusText(http.StatusAccepted), ure.StatusText())

		must.True(t, ure.HasBody())
		must.Eq(t, mockNamespaceBody, ure.Body())
	})

	// NotFound tests that an UnexpectedResponseError is generated and filled
	// with the correct data when a `404 Not Found`` is returned to the API
	// client, since the requireOK wrapper doesn't "expect" 404s.
	t.Run("NotFound", func(t *testing.T) {
		testutil.Parallel(t)
		n, _, err := c.Namespaces().Info("wat", nil)
		must.Nil(t, n)
		must.Error(t, err)
		t.Logf("err: %v", err)

		ure, ok := err.(api.UnexpectedResponseError)
		must.True(t, ok)

		must.True(t, ure.HasStatusCode())
		must.Eq(t, http.StatusNotFound, ure.StatusCode())

		must.True(t, ure.HasStatusText())
		must.Eq(t, http.StatusText(http.StatusNotFound), ure.StatusText())

		must.True(t, ure.HasBody())
		must.Eq(t, "Namespace not found", ure.Body())
	})

	// EarlyClose tests what happens when an error occurs during the building of
	// the UnexpectedResponseError using FromHTTPRequest.
	t.Run("EarlyClose", func(t *testing.T) {
		testutil.Parallel(t)
		n, _, err := c.Namespaces().Info("earlyClose", nil)
		must.Nil(t, n)
		must.Error(t, err)

		t.Logf("e: %v\n", err)
		ure, ok := err.(api.UnexpectedResponseError)
		must.True(t, ok)

		must.True(t, ure.HasStatusCode())
		must.Eq(t, http.StatusInternalServerError, ure.StatusCode())

		must.True(t, ure.HasStatusText())
		must.Eq(t, http.StatusText(http.StatusInternalServerError), ure.StatusText())

		must.True(t, ure.HasAdditional())
		must.ErrorContains(t, err, "the body might be truncated")

		must.True(t, ure.HasBody())
		must.Eq(t, "{", ure.Body()) // The body is truncated to the first byte
	})
}

// mockserver creates a httptest.Server that can be used to serve simple mock
// data, which is faster than starting a real Nomad agent.
func mockserver(t *testing.T) string {
	port := testutil.PortAllocator.One()

	mux := http.NewServeMux()
	mux.Handle("/v1/namespace/earlyClose", closingHandler(http.StatusInternalServerError, mockNamespaceBody))
	mux.Handle("/v1/namespace/badStatus", testHandler(http.StatusAccepted, mockNamespaceBody))
	mux.Handle("/v1/namespace/default", testHandler(http.StatusOK, mockNamespaceBody))
	mux.Handle("/v1/namespace/", testNotFoundHandler("Namespace not found"))
	mux.Handle("/v1/namespace", http.NotFoundHandler())
	mux.Handle("/v1", http.NotFoundHandler())
	mux.Handle("/", testHandler(http.StatusOK, "ok"))

	lMux := testLogRequestHandler(t, mux)
	ts := httptest.NewUnstartedServer(lMux)
	ts.Config.Addr = fmt.Sprintf("127.0.0.1:%d", port)

	t.Logf("starting mock server on %s", ts.Config.Addr)
	ts.Start()
	t.Cleanup(func() {
		t.Log("stopping mock server")
		ts.Close()
	})

	// Test the server
	tc := ts.Client()
	resp, err := tc.Get(func() string { p, _ := url.JoinPath(ts.URL, "/"); return p }())
	must.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	must.NoError(t, err)
	t.Logf("checking mock server, got resp: %s", b)

	// If we get here, the mock server is running and ready for requests.
	return ts.URL
}

// addMockHeaders sets the common Nomad headers to values sufficient to be
// parsed into api.QueryMeta
func addMockHeaders(h http.Header) {
	h.Add("X-Nomad-Knownleader", "true")
	h.Add("X-Nomad-Lastcontact", "0")
	h.Add("X-Nomad-Index", "1")
	h.Add("Content-Type", "application/json")
}

// testNotFoundHandler creates a testHandler preconfigured with status code 404.
func testNotFoundHandler(b string) http.Handler { return testHandler(http.StatusNotFound, b) }

// testNotFoundHandler creates a testHandler preconfigured with status code 200.
func testOKHandler(b string) http.Handler { return testHandler(http.StatusOK, b) }

// testHandler is a helper function that writes a Nomad-like server response
// with the necessary headers to make the API client happy
func testHandler(sc int, b string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addMockHeaders(w.Header())
		w.WriteHeader(sc)
		w.Write([]byte(b))
	})
}

// closingHandler is a handler that terminates the response body early in the
// reading process
func closingHandler(sc int, b string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// We need a misbehaving reader to test network effects when collecting
		// the http.Response data into a UnexpectedResponseError
		er := iotest.TimeoutReader( // TimeoutReader throws an error on the second read
			iotest.OneByteReader( // OneByteReader yields a byte at a time, causing multiple reads
				strings.NewReader(mockNamespaceBody),
			),
		)

		// We need to set content-length to the true value it _should_ be so the
		// API-side reader knows it's a short read.
		w.Header().Set("content-length", fmt.Sprint(len(mockNamespaceBody)))
		addMockHeaders(w.Header())
		w.WriteHeader(sc)

		// Using io.Copy to send the data into w prevents golang from setting the
		// content-length itself.
		io.Copy(w, er)
	})
}

// testLogRequestHandler wraps a http.Handler with a logger that writes to the
// test log output
func testLogRequestHandler(t *testing.T, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// call the original http.Handler wrapped in a httpsnoop
		m := httpsnoop.CaptureMetrics(h, w, r)
		ri := httpReqInfo{
			uri:       r.URL.String(),
			method:    r.Method,
			ipaddr:    ipAddrFromRemoteAddr(r.RemoteAddr),
			code:      m.Code,
			duration:  m.Duration,
			size:      m.Written,
			userAgent: r.UserAgent(),
		}
		t.Logf(ri.String())
	})
}

// httpReqInfo holds all the information used to log a request to the mock server
type httpReqInfo struct {
	method    string
	uri       string
	referer   string
	ipaddr    string
	code      int
	size      int64
	duration  time.Duration
	userAgent string
}

func (i httpReqInfo) String() string {
	return fmt.Sprintf(
		"method=%q uri=%q referer=%q ipaddr=%q code=%d size=%d duration=%q userAgent=%q",
		i.method, i.uri, i.referer, i.ipaddr, i.code, i.size, i.duration, i.userAgent,
	)
}

// ipAddrFromRemoteAddr removes the port from the address:port in remote addr
// in case of a parse error, the original value is returned unparsed
func ipAddrFromRemoteAddr(s string) string {
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().String()
	}
	return s
}

// unmock attempts to unmarshal a given mock json body into dst, which should
// be a pointer to the correct API struct.
func unmock(t *testing.T, src string, dst any) error {
	if err := json.Unmarshal([]byte(src), dst); err != nil {
		return fmt.Errorf("error unmarshaling mock: %w", err)
	}
	return nil
}
