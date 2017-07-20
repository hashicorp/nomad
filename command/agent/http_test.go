package agent

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
)

// makeHTTPServer returns a test server whose logs will be written to
// the passed writer. If the writer is nil, the logs are written to stderr.
func makeHTTPServer(t testing.TB, cb func(c *Config)) *TestAgent {
	return NewTestAgent(t.Name(), cb)
}

func BenchmarkHTTPRequests(b *testing.B) {
	s := makeHTTPServer(b, func(c *Config) {
		c.Client.Enabled = false
	})
	defer s.Shutdown()

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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	resp := httptest.NewRecorder()
	setLastContact(resp, 123456*time.Microsecond)
	header := resp.Header().Get("X-Nomad-LastContact")
	if header != "123" {
		t.Fatalf("Bad: %v", header)
	}
}

func TestSetMeta(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	s := makeHTTPServer(t, nil)
	s.Agent.config.HTTPAPIResponseHeaders = map[string]string{"foo": "bar"}
	defer s.Shutdown()

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
	t.Parallel()
	s := makeHTTPServer(t, nil)
	defer s.Shutdown()

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
	t.Parallel()
	testPrettyPrint("pretty=1", true, t)
}

func TestPrettyPrintOff(t *testing.T) {
	t.Parallel()
	testPrettyPrint("pretty=0", false, t)
}

func TestPrettyPrintBare(t *testing.T) {
	t.Parallel()
	testPrettyPrint("pretty", true, t)
}

func testPrettyPrint(pretty string, prettyFmt bool, t *testing.T) {
	s := makeHTTPServer(t, nil)
	defer s.Shutdown()

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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	s := makeHTTPServer(t, nil)
	defer s.Shutdown()

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

// TestHTTP_VerifyHTTPSClient asserts that a client certificate signed by the
// appropriate CA is required when VerifyHTTPSClient=true.
func TestHTTP_VerifyHTTPSClient(t *testing.T) {
	t.Parallel()
	const (
		cafile  = "../../helper/tlsutil/testdata/ca.pem"
		foocert = "../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey  = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
	)
	s := makeHTTPServer(t, func(c *Config) {
		c.Region = "foo" // match the region on foocert
		c.TLSConfig = &config.TLSConfig{
			EnableHTTP:        true,
			VerifyHTTPSClient: true,
			CAFile:            cafile,
			CertFile:          foocert,
			KeyFile:           fookey,
		}
	})
	defer s.Shutdown()

	reqURL := fmt.Sprintf("https://%s/v1/agent/self", s.Agent.config.AdvertiseAddrs.HTTP)

	// FAIL: Requests that expect 127.0.0.1 as the name should fail
	resp, err := http.Get(reqURL)
	if err == nil {
		resp.Body.Close()
		t.Fatalf("expected non-nil error but received: %v", resp.StatusCode)
	}
	urlErr, ok := err.(*url.Error)
	if !ok {
		t.Fatalf("expected a *url.Error but received: %T -> %v", err, err)
	}
	hostErr, ok := urlErr.Err.(x509.HostnameError)
	if !ok {
		t.Fatalf("expected a x509.HostnameError but received: %T -> %v", urlErr.Err, urlErr.Err)
	}
	if expected := "127.0.0.1"; hostErr.Host != expected {
		t.Fatalf("expected hostname on error to be %q but found %q", expected, hostErr.Host)
	}

	// FAIL: Requests that specify a valid hostname but not the CA should
	// fail
	tlsConf := &tls.Config{
		ServerName: "client.regionFoo.nomad",
	}
	transport := &http.Transport{TLSClientConfig: tlsConf}
	client := &http.Client{Transport: transport}
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		t.Fatalf("error creating request: %v", err)
	}
	resp, err = client.Do(req)
	if err == nil {
		resp.Body.Close()
		t.Fatalf("expected non-nil error but received: %v", resp.StatusCode)
	}
	urlErr, ok = err.(*url.Error)
	if !ok {
		t.Fatalf("expected a *url.Error but received: %T -> %v", err, err)
	}
	_, ok = urlErr.Err.(x509.UnknownAuthorityError)
	if !ok {
		t.Fatalf("expected a x509.UnknownAuthorityError but received: %T -> %v", urlErr.Err, urlErr.Err)
	}

	// FAIL: Requests that specify a valid hostname and CA cert but lack a
	// client certificate should fail
	cacertBytes, err := ioutil.ReadFile(cafile)
	if err != nil {
		t.Fatalf("error reading cacert: %v", err)
	}
	tlsConf.RootCAs = x509.NewCertPool()
	tlsConf.RootCAs.AppendCertsFromPEM(cacertBytes)
	req, err = http.NewRequest("GET", reqURL, nil)
	if err != nil {
		t.Fatalf("error creating request: %v", err)
	}
	resp, err = client.Do(req)
	if err == nil {
		resp.Body.Close()
		t.Fatalf("expected non-nil error but received: %v", resp.StatusCode)
	}
	urlErr, ok = err.(*url.Error)
	if !ok {
		t.Fatalf("expected a *url.Error but received: %T -> %v", err, err)
	}
	opErr, ok := urlErr.Err.(*net.OpError)
	if !ok {
		t.Fatalf("expected a *net.OpErr but received: %T -> %v", urlErr.Err, urlErr.Err)
	}
	const badCertificate = "tls: bad certificate" // from crypto/tls/alert.go:52 and RFC 5246 § A.3
	if opErr.Err.Error() != badCertificate {
		t.Fatalf("expected tls.alert bad_certificate but received: %q", opErr.Err.Error())
	}

	// PASS: Requests that specify a valid hostname, CA cert, and client
	// certificate succeed.
	tlsConf.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		c, err := tls.LoadX509KeyPair(foocert, fookey)
		if err != nil {
			return nil, err
		}
		return &c, nil
	}
	transport = &http.Transport{TLSClientConfig: tlsConf}
	client = &http.Client{Transport: transport}
	req, err = http.NewRequest("GET", reqURL, nil)
	if err != nil {
		t.Fatalf("error creating request: %v", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 status code but got: %d", resp.StatusCode)
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

func httpTest(t testing.TB, cb func(c *Config), f func(srv *TestAgent)) {
	s := makeHTTPServer(t, cb)
	defer s.Shutdown()
	testutil.WaitForLeader(t, s.Agent.RPC)
	f(s)
}

func encodeReq(obj interface{}) io.ReadCloser {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.Encode(obj)
	return ioutil.NopCloser(buf)
}
