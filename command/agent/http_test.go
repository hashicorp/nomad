package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
)

type TestServer struct {
	T      testing.TB
	Dir    string
	Agent  *Agent
	Server *HTTPServer
}

func (s *TestServer) Cleanup() {
	s.Server.Shutdown()
	s.Agent.Shutdown()
	os.RemoveAll(s.Dir)
}

// makeHTTPServerNoLogs returns a test server with full logging.
func makeHTTPServer(t testing.TB, cb func(c *Config)) *TestServer {
	return makeHTTPServerWithWriter(t, nil, cb)
}

// makeHTTPServerNoLogs returns a test server which only prints agent logs and
// no http server logs
func makeHTTPServerNoLogs(t testing.TB, cb func(c *Config)) *TestServer {
	return makeHTTPServerWithWriter(t, ioutil.Discard, cb)
}

// makeHTTPServerWithWriter returns a test server whose logs will be written to
// the passed writer. If the writer is nil, the logs are written to stderr.
func makeHTTPServerWithWriter(t testing.TB, w io.Writer, cb func(c *Config)) *TestServer {
	dir, agent := makeAgent(t, cb)
	if w == nil {
		w = agent.logOutput
	}
	srv, err := NewHTTPServer(agent, agent.config, w)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	s := &TestServer{
		T:      t,
		Dir:    dir,
		Agent:  agent,
		Server: srv,
	}
	return s
}

func BenchmarkHTTPRequests(b *testing.B) {
	s := makeHTTPServerNoLogs(b, func(c *Config) {
		c.Client.Enabled = false
	})
	defer s.Cleanup()

	job := mock.Job()
	var allocs []*structs.Allocation
	count := 1000
	for i := 0; i < count; i++ {
		alloc := mock.Alloc()
		alloc.Job = job
		alloc.JobID = job.ID
		alloc.Name = fmt.Sprintf("my-job.web[%d]", i)
		allocs = append(allocs, alloc)
	}

	handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
		return allocs[:count], nil
	}
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/v1/kv/key", nil)
			s.Server.wrap(handler)(resp, req)
		}
	})
}

func TestSetIndex(t *testing.T) {
	resp := httptest.NewRecorder()
	setIndex(resp, 1000)
	header := resp.Header().Get("X-Nomad-Index")
	if header != "1000" {
		t.Fatalf("Bad: %v", header)
	}
	setIndex(resp, 2000)
	if v := resp.Header()["X-Nomad-Index"]; len(v) != 1 {
		t.Fatalf("bad: %#v", v)
	}
}

func TestSetKnownLeader(t *testing.T) {
	resp := httptest.NewRecorder()
	setKnownLeader(resp, true)
	header := resp.Header().Get("X-Nomad-KnownLeader")
	if header != "true" {
		t.Fatalf("Bad: %v", header)
	}
	resp = httptest.NewRecorder()
	setKnownLeader(resp, false)
	header = resp.Header().Get("X-Nomad-KnownLeader")
	if header != "false" {
		t.Fatalf("Bad: %v", header)
	}
}

func TestSetLastContact(t *testing.T) {
	resp := httptest.NewRecorder()
	setLastContact(resp, 123456*time.Microsecond)
	header := resp.Header().Get("X-Nomad-LastContact")
	if header != "123" {
		t.Fatalf("Bad: %v", header)
	}
}

func TestSetMeta(t *testing.T) {
	meta := structs.QueryMeta{
		Index:       1000,
		KnownLeader: true,
		LastContact: 123456 * time.Microsecond,
	}
	resp := httptest.NewRecorder()
	setMeta(resp, &meta)
	header := resp.Header().Get("X-Nomad-Index")
	if header != "1000" {
		t.Fatalf("Bad: %v", header)
	}
	header = resp.Header().Get("X-Nomad-KnownLeader")
	if header != "true" {
		t.Fatalf("Bad: %v", header)
	}
	header = resp.Header().Get("X-Nomad-LastContact")
	if header != "123" {
		t.Fatalf("Bad: %v", header)
	}
}

func TestSetHeaders(t *testing.T) {
	s := makeHTTPServer(t, nil)
	s.Agent.config.HTTPAPIResponseHeaders = map[string]string{"foo": "bar"}
	defer s.Cleanup()

	resp := httptest.NewRecorder()
	handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
		return &structs.Job{Name: "foo"}, nil
	}

	req, _ := http.NewRequest("GET", "/v1/kv/key", nil)
	s.Server.wrap(handler)(resp, req)
	header := resp.Header().Get("foo")

	if header != "bar" {
		t.Fatalf("expected header: %v, actual: %v", "bar", header)
	}

}

func TestContentTypeIsJSON(t *testing.T) {
	s := makeHTTPServer(t, nil)
	defer s.Cleanup()

	resp := httptest.NewRecorder()

	handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
		return &structs.Job{Name: "foo"}, nil
	}

	req, _ := http.NewRequest("GET", "/v1/kv/key", nil)
	s.Server.wrap(handler)(resp, req)

	contentType := resp.Header().Get("Content-Type")

	if contentType != "application/json" {
		t.Fatalf("Content-Type header was not 'application/json'")
	}
}

func TestPrettyPrint(t *testing.T) {
	testPrettyPrint("pretty=1", true, t)
}

func TestPrettyPrintOff(t *testing.T) {
	testPrettyPrint("pretty=0", false, t)
}

func TestPrettyPrintBare(t *testing.T) {
	testPrettyPrint("pretty", true, t)
}

func testPrettyPrint(pretty string, prettyFmt bool, t *testing.T) {
	s := makeHTTPServer(t, nil)
	defer s.Cleanup()

	r := &structs.Job{Name: "foo"}

	resp := httptest.NewRecorder()
	handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
		return r, nil
	}

	urlStr := "/v1/job/foo?" + pretty
	req, _ := http.NewRequest("GET", urlStr, nil)
	s.Server.wrap(handler)(resp, req)

	var expected []byte
	if prettyFmt {
		expected, _ = json.MarshalIndent(r, "", "    ")
		expected = append(expected, "\n"...)
	} else {
		expected, _ = json.Marshal(r)
	}
	actual, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !bytes.Equal(expected, actual) {
		t.Fatalf("bad:\nexpected:\t%q\nactual:\t\t%q", string(expected), string(actual))
	}
}

func TestParseWait(t *testing.T) {
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, err := http.NewRequest("GET",
		"/v1/catalog/nodes?wait=60s&index=1000", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if d := parseWait(resp, req, &b); d {
		t.Fatalf("unexpected done")
	}

	if b.MinQueryIndex != 1000 {
		t.Fatalf("Bad: %v", b)
	}
	if b.MaxQueryTime != 60*time.Second {
		t.Fatalf("Bad: %v", b)
	}
}

func TestParseWait_InvalidTime(t *testing.T) {
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, err := http.NewRequest("GET",
		"/v1/catalog/nodes?wait=60foo&index=1000", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if d := parseWait(resp, req, &b); !d {
		t.Fatalf("expected done")
	}

	if resp.Code != 400 {
		t.Fatalf("bad code: %v", resp.Code)
	}
}

func TestParseWait_InvalidIndex(t *testing.T) {
	resp := httptest.NewRecorder()
	var b structs.QueryOptions

	req, err := http.NewRequest("GET",
		"/v1/catalog/nodes?wait=60s&index=foo", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if d := parseWait(resp, req, &b); !d {
		t.Fatalf("expected done")
	}

	if resp.Code != 400 {
		t.Fatalf("bad code: %v", resp.Code)
	}
}

func TestParseConsistency(t *testing.T) {
	var b structs.QueryOptions

	req, err := http.NewRequest("GET",
		"/v1/catalog/nodes?stale", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	parseConsistency(req, &b)
	if !b.AllowStale {
		t.Fatalf("Bad: %v", b)
	}

	b = structs.QueryOptions{}
	req, err = http.NewRequest("GET",
		"/v1/catalog/nodes?consistent", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	parseConsistency(req, &b)
	if b.AllowStale {
		t.Fatalf("Bad: %v", b)
	}
}

func TestParseRegion(t *testing.T) {
	s := makeHTTPServer(t, nil)
	defer s.Cleanup()

	req, err := http.NewRequest("GET",
		"/v1/jobs?region=foo", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var region string
	s.Server.parseRegion(req, &region)
	if region != "foo" {
		t.Fatalf("bad %s", region)
	}

	region = ""
	req, err = http.NewRequest("GET", "/v1/jobs", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	s.Server.parseRegion(req, &region)
	if region != "global" {
		t.Fatalf("bad %s", region)
	}
}

// assertIndex tests that X-Nomad-Index is set and non-zero
func assertIndex(t *testing.T, resp *httptest.ResponseRecorder) {
	header := resp.Header().Get("X-Nomad-Index")
	if header == "" || header == "0" {
		t.Fatalf("Bad: %v", header)
	}
}

// checkIndex is like assertIndex but returns an error
func checkIndex(resp *httptest.ResponseRecorder) error {
	header := resp.Header().Get("X-Nomad-Index")
	if header == "" || header == "0" {
		return fmt.Errorf("Bad: %v", header)
	}
	return nil
}

// getIndex parses X-Nomad-Index
func getIndex(t *testing.T, resp *httptest.ResponseRecorder) uint64 {
	header := resp.Header().Get("X-Nomad-Index")
	if header == "" {
		t.Fatalf("Bad: %v", header)
	}
	val, err := strconv.Atoi(header)
	if err != nil {
		t.Fatalf("Bad: %v", header)
	}
	return uint64(val)
}

func httpTest(t testing.TB, cb func(c *Config), f func(srv *TestServer)) {
	s := makeHTTPServer(t, cb)
	defer s.Cleanup()
	testutil.WaitForLeader(t, s.Agent.RPC)
	f(s)
}

func encodeReq(obj interface{}) io.ReadCloser {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.Encode(obj)
	return ioutil.NopCloser(buf)
}
