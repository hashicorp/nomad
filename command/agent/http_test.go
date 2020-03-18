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
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/structs/config"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeHTTPServer returns a test server whose logs will be written to
// the passed writer. If the writer is nil, the logs are written to stderr.
func makeHTTPServer(t testing.TB, cb func(c *Config)) *TestAgent {
	return NewTestAgent(t, t.Name(), cb)
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

// TestRootFallthrough tests rootFallthrough handler to
// verify redirect and 404 behavior
func TestRootFallthrough(t *testing.T) {
	t.Parallel()

	cases := []struct {
		desc         string
		path         string
		expectedPath string
		expectedCode int
	}{
		{
			desc:         "unknown endpoint 404s",
			path:         "/v1/unknown/endpoint",
			expectedCode: 404,
		},
		{
			desc:         "root path redirects to ui",
			path:         "/",
			expectedPath: "/ui/",
			expectedCode: 307,
		},
	}

	s := makeHTTPServer(t, nil)
	defer s.Shutdown()

	// setup a client that doesn't follow redirects
	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {

			reqURL := fmt.Sprintf("http://%s%s", s.Agent.config.AdvertiseAddrs.HTTP, tc.path)

			resp, err := client.Get(reqURL)
			require.NoError(t, err)
			require.Equal(t, tc.expectedCode, resp.StatusCode)

			if tc.expectedPath != "" {
				loc, err := resp.Location()
				require.NoError(t, err)
				require.Equal(t, tc.expectedPath, loc.Path)
			}
		})
	}
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

func TestWrapNonJSON(t *testing.T) {
	t.Parallel()
	s := makeHTTPServer(t, nil)
	defer s.Shutdown()

	resp := httptest.NewRecorder()

	handler := func(resp http.ResponseWriter, req *http.Request) ([]byte, error) {
		return []byte("test response"), nil
	}

	req, _ := http.NewRequest("GET", "/v1/kv/key", nil)
	s.Server.wrapNonJSON(handler)(resp, req)

	respBody, _ := ioutil.ReadAll(resp.Body)
	require.Equal(t, respBody, []byte("test response"))

}

func TestWrapNonJSON_Error(t *testing.T) {
	t.Parallel()
	s := makeHTTPServer(t, nil)
	defer s.Shutdown()

	handlerRPCErr := func(resp http.ResponseWriter, req *http.Request) ([]byte, error) {
		return nil, structs.NewErrRPCCoded(404, "not found")
	}

	handlerCodedErr := func(resp http.ResponseWriter, req *http.Request) ([]byte, error) {
		return nil, CodedError(422, "unprocessable")
	}

	// RPC coded error
	{
		resp := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/kv/key", nil)
		s.Server.wrapNonJSON(handlerRPCErr)(resp, req)
		respBody, _ := ioutil.ReadAll(resp.Body)
		require.Equal(t, []byte("not found"), respBody)
		require.Equal(t, 404, resp.Code)
	}

	// CodedError
	{
		resp := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/v1/kv/key", nil)
		s.Server.wrapNonJSON(handlerCodedErr)(resp, req)
		respBody, _ := ioutil.ReadAll(resp.Body)
		require.Equal(t, []byte("unprocessable"), respBody)
		require.Equal(t, 422, resp.Code)
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

	var expected bytes.Buffer
	var err error
	if prettyFmt {
		enc := codec.NewEncoder(&expected, structs.JsonHandlePretty)
		err = enc.Encode(r)
		expected.WriteByte('\n')
	} else {
		enc := codec.NewEncoder(&expected, structs.JsonHandle)
		err = enc.Encode(r)
	}
	if err != nil {
		t.Fatalf("failed to encode: %v", err)
	}
	actual, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if !bytes.Equal(expected.Bytes(), actual) {
		t.Fatalf("bad:\nexpected:\t%q\nactual:\t\t%q", expected.String(), string(actual))
	}
}

func TestPermissionDenied(t *testing.T) {
	s := makeHTTPServer(t, func(c *Config) {
		c.ACL.Enabled = true
	})
	defer s.Shutdown()

	{
		resp := httptest.NewRecorder()
		handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
			return nil, structs.ErrPermissionDenied
		}

		req, _ := http.NewRequest("GET", "/v1/job/foo", nil)
		s.Server.wrap(handler)(resp, req)
		assert.Equal(t, resp.Code, 403)
	}

	// When remote RPC is used the errors have "rpc error: " prependend
	{
		resp := httptest.NewRecorder()
		handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
			return nil, fmt.Errorf("rpc error: %v", structs.ErrPermissionDenied)
		}

		req, _ := http.NewRequest("GET", "/v1/job/foo", nil)
		s.Server.wrap(handler)(resp, req)
		assert.Equal(t, resp.Code, 403)
	}
}

func TestTokenNotFound(t *testing.T) {
	s := makeHTTPServer(t, func(c *Config) {
		c.ACL.Enabled = true
	})
	defer s.Shutdown()

	resp := httptest.NewRecorder()
	handler := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
		return nil, structs.ErrTokenNotFound
	}

	urlStr := "/v1/job/foo"
	req, _ := http.NewRequest("GET", urlStr, nil)
	s.Server.wrap(handler)(resp, req)
	assert.Equal(t, resp.Code, 403)
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

func TestParseToken(t *testing.T) {
	t.Parallel()
	s := makeHTTPServer(t, nil)
	defer s.Shutdown()

	req, err := http.NewRequest("GET", "/v1/jobs", nil)
	req.Header.Add("X-Nomad-Token", "foobar")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var token string
	s.Server.parseToken(req, &token)
	if token != "foobar" {
		t.Fatalf("bad %s", token)
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

func TestHTTP_VerifyHTTPSClient_AfterConfigReload(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	const (
		cafile   = "../../helper/tlsutil/testdata/ca.pem"
		foocert  = "../../helper/tlsutil/testdata/nomad-bad.pem"
		fookey   = "../../helper/tlsutil/testdata/nomad-bad-key.pem"
		foocert2 = "../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey2  = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
	)

	agentConfig := &Config{
		TLSConfig: &config.TLSConfig{
			EnableHTTP:        true,
			VerifyHTTPSClient: true,
			CAFile:            cafile,
			CertFile:          foocert,
			KeyFile:           fookey,
		},
	}

	newConfig := &Config{
		TLSConfig: &config.TLSConfig{
			EnableHTTP:        true,
			VerifyHTTPSClient: true,
			CAFile:            cafile,
			CertFile:          foocert2,
			KeyFile:           fookey2,
		},
	}

	s := makeHTTPServer(t, func(c *Config) {
		c.TLSConfig = agentConfig.TLSConfig
	})
	defer s.Shutdown()

	// Make an initial request that should fail.
	// Requests that specify a valid hostname, CA cert, and client
	// certificate succeed.
	tlsConf := &tls.Config{
		ServerName: "client.regionFoo.nomad",
		RootCAs:    x509.NewCertPool(),
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			c, err := tls.LoadX509KeyPair(foocert, fookey)
			if err != nil {
				return nil, err
			}
			return &c, nil
		},
	}

	// HTTPS request should succeed
	httpsReqURL := fmt.Sprintf("https://%s/v1/agent/self", s.Agent.config.AdvertiseAddrs.HTTP)

	cacertBytes, err := ioutil.ReadFile(cafile)
	assert.Nil(err)
	tlsConf.RootCAs.AppendCertsFromPEM(cacertBytes)

	transport := &http.Transport{TLSClientConfig: tlsConf}
	client := &http.Client{Transport: transport}
	req, err := http.NewRequest("GET", httpsReqURL, nil)
	assert.Nil(err)

	// Check that we get an error that the certificate isn't valid for the
	// region we are contacting.
	_, err = client.Do(req)
	assert.Contains(err.Error(), "certificate is valid for")

	// Reload the TLS configuration==
	assert.Nil(s.Agent.Reload(newConfig))

	// Requests that specify a valid hostname, CA cert, and client
	// certificate succeed.
	tlsConf = &tls.Config{
		ServerName: "client.regionFoo.nomad",
		RootCAs:    x509.NewCertPool(),
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			c, err := tls.LoadX509KeyPair(foocert2, fookey2)
			if err != nil {
				return nil, err
			}
			return &c, nil
		},
	}

	cacertBytes, err = ioutil.ReadFile(cafile)
	assert.Nil(err)
	tlsConf.RootCAs.AppendCertsFromPEM(cacertBytes)

	transport = &http.Transport{TLSClientConfig: tlsConf}
	client = &http.Client{Transport: transport}
	req, err = http.NewRequest("GET", httpsReqURL, nil)
	assert.Nil(err)

	resp, err := client.Do(req)
	if assert.Nil(err) {
		resp.Body.Close()
		assert.Equal(resp.StatusCode, 200)
	}
}

// TestHTTPServer_Limits_Error asserts invalid Limits cause errors. This is the
// HTTP counterpart to TestAgent_ServerConfig_Limits_Error.
func TestHTTPServer_Limits_Error(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tls         bool
		timeout     string
		limit       *int
		expectedErr string
	}{
		{
			tls:         true,
			timeout:     "",
			limit:       nil,
			expectedErr: "error parsing https_handshake_timeout: ",
		},
		{
			tls:         false,
			timeout:     "",
			limit:       nil,
			expectedErr: "error parsing https_handshake_timeout: ",
		},
		{
			tls:         true,
			timeout:     "-1s",
			limit:       nil,
			expectedErr: "https_handshake_timeout must be >= 0",
		},
		{
			tls:         false,
			timeout:     "-1s",
			limit:       nil,
			expectedErr: "https_handshake_timeout must be >= 0",
		},
		{
			tls:         true,
			timeout:     "5s",
			limit:       helper.IntToPtr(-1),
			expectedErr: "http_max_conns_per_client must be >= 0",
		},
		{
			tls:         false,
			timeout:     "5s",
			limit:       helper.IntToPtr(-1),
			expectedErr: "http_max_conns_per_client must be >= 0",
		},
	}

	for i := range cases {
		tc := cases[i]
		name := fmt.Sprintf("%d-tls-%t-timeout-%s-limit-%v", i, tc.tls, tc.timeout, tc.limit)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Use a fake agent since the HTTP server should never start
			agent := &Agent{
				logger: testlog.HCLogger(t),
			}

			conf := &Config{
				normalizedAddrs: &Addresses{
					HTTP: "localhost:0", // port is never used
				},
				TLSConfig: &config.TLSConfig{
					EnableHTTP: tc.tls,
				},
				Limits: config.Limits{
					HTTPSHandshakeTimeout: tc.timeout,
					HTTPMaxConnsPerClient: tc.limit,
				},
			}

			srv, err := NewHTTPServer(agent, conf)
			require.Error(t, err)
			require.Nil(t, srv)
			require.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

// TestHTTPServer_Limits_OK asserts that all valid limits combinations
// (tls/timeout/conns) work.
func TestHTTPServer_Limits_OK(t *testing.T) {
	t.Parallel()
	const (
		cafile   = "../../helper/tlsutil/testdata/ca.pem"
		foocert  = "../../helper/tlsutil/testdata/nomad-foo.pem"
		fookey   = "../../helper/tlsutil/testdata/nomad-foo-key.pem"
		maxConns = 10 // limit must be < this for testing
	)

	cases := []struct {
		tls           bool
		timeout       string
		limit         *int
		assertTimeout bool
		assertLimit   bool
	}{
		{
			tls:           false,
			timeout:       "5s",
			limit:         nil,
			assertTimeout: false,
			assertLimit:   false,
		},
		{
			tls:           true,
			timeout:       "5s",
			limit:         nil,
			assertTimeout: true,
			assertLimit:   false,
		},
		{
			tls:           false,
			timeout:       "0",
			limit:         nil,
			assertTimeout: false,
			assertLimit:   false,
		},
		{
			tls:           true,
			timeout:       "0",
			limit:         nil,
			assertTimeout: false,
			assertLimit:   false,
		},
		{
			tls:           false,
			timeout:       "0",
			limit:         helper.IntToPtr(2),
			assertTimeout: false,
			assertLimit:   true,
		},
		{
			tls:           true,
			timeout:       "0",
			limit:         helper.IntToPtr(2),
			assertTimeout: false,
			assertLimit:   true,
		},
		{
			tls:           false,
			timeout:       "5s",
			limit:         helper.IntToPtr(2),
			assertTimeout: false,
			assertLimit:   true,
		},
		{
			tls:           true,
			timeout:       "5s",
			limit:         helper.IntToPtr(2),
			assertTimeout: true,
			assertLimit:   true,
		},
	}

	assertTimeout := func(t *testing.T, a *TestAgent, assertTimeout bool, timeout string) {
		timeoutDeadline, err := time.ParseDuration(timeout)
		require.NoError(t, err)

		// Increase deadline to detect timeouts
		deadline := timeoutDeadline + time.Second

		conn, err := net.DialTimeout("tcp", a.Server.Addr, deadline)
		require.NoError(t, err)
		defer conn.Close()

		buf := []byte{0}
		readDeadline := time.Now().Add(deadline)
		conn.SetReadDeadline(readDeadline)
		n, err := conn.Read(buf)
		require.Zero(t, n)
		if assertTimeout {
			// Server timeouts == EOF
			require.Equal(t, io.EOF, err)

			// Perform blocking query to assert timeout is not
			// enabled post-TLS-handshake.
			q := &api.QueryOptions{
				WaitIndex: 10000, // wait a looong time
				WaitTime:  deadline,
			}

			// Assertions don't require certificate validation
			conf := api.DefaultConfig()
			conf.Address = a.HTTPAddr()
			conf.TLSConfig.Insecure = true
			client, err := api.NewClient(conf)
			require.NoError(t, err)

			// Assert a blocking query isn't timed out by the
			// handshake timeout
			jobs, meta, err := client.Jobs().List(q)
			require.NoError(t, err)
			require.Len(t, jobs, 0)
			require.Truef(t, meta.RequestTime >= deadline,
				"expected RequestTime (%s) >= Deadline (%s)",
				meta.RequestTime, deadline)

			return
		}

		// HTTP Server should *not* have timed out.
		// Now() should always be after the read deadline, but
		// isn't a sufficient assertion for correctness as slow
		// tests may cause this to be true even if the server
		// timed out.
		require.True(t, time.Now().After(readDeadline))

		testutil.RequireDeadlineErr(t, err)
	}

	assertNoLimit := func(t *testing.T, addr string) {
		var err error

		// Create max connections
		conns := make([]net.Conn, maxConns)
		errCh := make(chan error, maxConns)
		for i := 0; i < maxConns; i++ {
			conns[i], err = net.DialTimeout("tcp", addr, 1*time.Second)
			require.NoError(t, err)
			defer conns[i].Close()

			go func(i int) {
				buf := []byte{0}
				readDeadline := time.Now().Add(1 * time.Second)
				conns[i].SetReadDeadline(readDeadline)
				n, err := conns[i].Read(buf)
				if n > 0 {
					errCh <- fmt.Errorf("n > 0: %d", n)
					return
				}
				errCh <- err
			}(i)
		}

		// Now assert each error is a clientside read deadline error
		for i := 0; i < maxConns; i++ {
			select {
			case <-time.After(2 * time.Second):
				t.Fatalf("timed out waiting for conn error %d", i)
			case err := <-errCh:
				testutil.RequireDeadlineErr(t, err)
			}
		}
	}

	assertLimit := func(t *testing.T, addr string, limit int) {
		var err error

		// Create limit connections
		conns := make([]net.Conn, limit)
		errCh := make(chan error, limit)
		for i := range conns {
			conns[i], err = net.DialTimeout("tcp", addr, 1*time.Second)
			require.NoError(t, err)
			defer conns[i].Close()

			go func(i int) {
				buf := []byte{0}
				n, err := conns[i].Read(buf)
				if n > 0 {
					errCh <- fmt.Errorf("n > 0: %d", n)
					return
				}
				errCh <- err
			}(i)
		}

		select {
		case err := <-errCh:
			t.Fatalf("unexpected error from connection prior to limit: %T %v", err, err)
		case <-time.After(500 * time.Millisecond):
		}

		// Assert a new connection is dropped
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		require.NoError(t, err)
		defer conn.Close()

		buf := []byte{0}
		deadline := time.Now().Add(10 * time.Second)
		conn.SetReadDeadline(deadline)
		n, err := conn.Read(buf)
		require.Zero(t, n)

		// Soft-fail as following assertion helps with debugging
		assert.Equal(t, io.EOF, err)

		// Assert existing connections are ok
		require.Len(t, errCh, 0)

		// Cleanup
		for _, conn := range conns {
			conn.Close()
		}
		for range conns {
			err := <-errCh
			require.Contains(t, err.Error(), "use of closed network connection")
		}
	}

	for i := range cases {
		tc := cases[i]
		name := fmt.Sprintf("%d-tls-%t-timeout-%s-limit-%v", i, tc.tls, tc.timeout, tc.limit)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if tc.limit != nil && *tc.limit >= maxConns {
				t.Fatalf("test fixture failure: cannot assert limit (%d) >= max (%d)", *tc.limit, maxConns)
			}

			s := makeHTTPServer(t, func(c *Config) {
				if tc.tls {
					c.TLSConfig = &config.TLSConfig{
						EnableHTTP: true,
						CAFile:     cafile,
						CertFile:   foocert,
						KeyFile:    fookey,
					}
				}
				c.Limits.HTTPSHandshakeTimeout = tc.timeout
				c.Limits.HTTPMaxConnsPerClient = tc.limit
			})
			defer s.Shutdown()

			assertTimeout(t, s, tc.assertTimeout, tc.timeout)

			if tc.assertLimit {
				// There's a race between assertTimeout(false) closing
				// its connection and the HTTP server noticing and
				// untracking it. Since there's no way to coordiante
				// when this occurs, sleeping is the only way to avoid
				// asserting limits before the timed out connection is
				// untracked.
				time.Sleep(1 * time.Second)

				assertLimit(t, s.Server.Addr, *tc.limit)
			} else {
				assertNoLimit(t, s.Server.Addr)
			}
		})
	}
}

func httpTest(t testing.TB, cb func(c *Config), f func(srv *TestAgent)) {
	s := makeHTTPServer(t, cb)
	defer s.Shutdown()
	testutil.WaitForLeader(t, s.Agent.RPC)
	f(s)
}

func httpACLTest(t testing.TB, cb func(c *Config), f func(srv *TestAgent)) {
	s := makeHTTPServer(t, func(c *Config) {
		c.ACL.Enabled = true
		if cb != nil {
			cb(c)
		}
	})
	defer s.Shutdown()
	testutil.WaitForLeader(t, s.Agent.RPC)
	f(s)
}

func setToken(req *http.Request, token *structs.ACLToken) {
	req.Header.Set("X-Nomad-Token", token.SecretID)
}

func encodeReq(obj interface{}) io.ReadCloser {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.Encode(obj)
	return ioutil.NopCloser(buf)
}
