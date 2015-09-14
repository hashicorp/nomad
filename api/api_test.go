package api

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/nomad/testutil"
)

type configCallback func(c *Config)

func makeClient(t *testing.T, cb1 configCallback,
	cb2 testutil.ServerConfigCallback) (*Client, *testutil.TestServer) {
	// Always run these tests in parallel
	t.Parallel()

	// Make client config
	conf := DefaultConfig()
	if cb1 != nil {
		cb1(conf)
	}

	// Create server
	server := testutil.NewTestServer(t, cb2)
	conf.URL = "http://" + server.HTTPAddr

	// Create client
	client, err := NewClient(conf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	return client, server
}

func TestDefaultConfig_env(t *testing.T) {
	t.Parallel()
	url := "http://1.2.3.4:5678"

	os.Setenv("NOMAD_HTTP_URL", url)
	defer os.Setenv("NOMAD_HTTP_URL", "")

	config := DefaultConfig()

	if config.URL != url {
		t.Errorf("expected %q to be %q", config.URL, url)
	}
}

func TestSetQueryOptions(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	r := c.newRequest("GET", "/v1/jobs")
	q := &QueryOptions{
		Region:     "foo",
		AllowStale: true,
		WaitIndex:  1000,
		WaitTime:   100 * time.Second,
	}
	r.setQueryOptions(q)

	if r.params.Get("region") != "foo" {
		t.Fatalf("bad: %v", r.params)
	}
	if _, ok := r.params["stale"]; !ok {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("index") != "1000" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("wait") != "100000ms" {
		t.Fatalf("bad: %v", r.params)
	}
}

func TestSetWriteOptions(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	r := c.newRequest("GET", "/v1/jobs")
	q := &WriteOptions{
		Region: "foo",
	}
	r.setWriteOptions(q)

	if r.params.Get("region") != "foo" {
		t.Fatalf("bad: %v", r.params)
	}
}

func TestRequestToHTTP(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	r := c.newRequest("DELETE", "/v1/jobs/foo")
	q := &QueryOptions{
		Region: "foo",
	}
	r.setQueryOptions(q)
	req, err := r.toHTTP()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if req.Method != "DELETE" {
		t.Fatalf("bad: %v", req)
	}
	if req.URL.RequestURI() != "/v1/jobs/foo?region=foo" {
		t.Fatalf("bad: %v", req)
	}
}

func TestParseQueryMeta(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		Header: make(map[string][]string),
	}
	resp.Header.Set("X-Nomad-Index", "12345")
	resp.Header.Set("X-Nomad-LastContact", "80")
	resp.Header.Set("X-Nomad-KnownLeader", "true")

	qm := &QueryMeta{}
	if err := parseQueryMeta(resp, qm); err != nil {
		t.Fatalf("err: %v", err)
	}

	if qm.LastIndex != 12345 {
		t.Fatalf("Bad: %v", qm)
	}
	if qm.LastContact != 80*time.Millisecond {
		t.Fatalf("Bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("Bad: %v", qm)
	}
}

func TestParseWriteMeta(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		Header: make(map[string][]string),
	}
	resp.Header.Set("X-Nomad-Index", "12345")

	wm := &WriteMeta{}
	if err := parseWriteMeta(resp, wm); err != nil {
		t.Fatalf("err: %v", err)
	}

	if wm.LastIndex != 12345 {
		t.Fatalf("Bad: %v", wm)
	}
}

func TestQueryString(t *testing.T) {
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	r := c.newRequest("PUT", "/v1/abc?foo=bar&baz=zip")
	q := &WriteOptions{Region: "foo"}
	r.setWriteOptions(q)

	req, err := r.toHTTP()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if uri := req.URL.RequestURI(); uri != "/v1/abc?baz=zip&foo=bar&region=foo" {
		t.Fatalf("bad uri: %q", uri)
	}
}
