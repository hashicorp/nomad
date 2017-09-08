package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

type configCallback func(c *Config)

// seen is used to track which tests we have already marked as parallel
var seen map[*testing.T]struct{}

func init() {
	seen = make(map[*testing.T]struct{})
}

func makeACLClient(t *testing.T, cb1 configCallback,
	cb2 testutil.ServerConfigCallback) (*Client, *testutil.TestServer, *ACLToken) {
	client, server := makeClient(t, cb1, func(c *testutil.TestServerConfig) {
		c.ACL.Enabled = true
		if cb2 != nil {
			cb2(c)
		}
	})

	// Get the root token
	root, _, err := client.ACLTokens().Bootstrap(nil)
	if err != nil {
		t.Fatalf("failed to bootstrap ACLs: %v", err)
	}
	client.SetSecretID(root.SecretID)
	return client, server, root
}

func makeClient(t *testing.T, cb1 configCallback,
	cb2 testutil.ServerConfigCallback) (*Client, *testutil.TestServer) {
	// Make client config
	conf := DefaultConfig()
	if cb1 != nil {
		cb1(conf)
	}

	// Create server
	server := testutil.NewTestServer(t, cb2)
	conf.Address = "http://" + server.HTTPAddr

	// Create client
	client, err := NewClient(conf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	return client, server
}

func TestRequestTime(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		d, err := json.Marshal(struct{ Done bool }{true})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(d)
	}))
	defer srv.Close()

	conf := DefaultConfig()
	conf.Address = srv.URL

	client, err := NewClient(conf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out interface{}

	qm, err := client.query("/", &out, nil)
	if err != nil {
		t.Fatalf("query err: %v", err)
	}
	if qm.RequestTime == 0 {
		t.Errorf("bad request time: %d", qm.RequestTime)
	}

	wm, err := client.write("/", struct{ S string }{"input"}, &out, nil)
	if err != nil {
		t.Fatalf("write err: %v", err)
	}
	if wm.RequestTime == 0 {
		t.Errorf("bad request time: %d", wm.RequestTime)
	}

	wm, err = client.delete("/", &out, nil)
	if err != nil {
		t.Fatalf("delete err: %v", err)
	}
	if wm.RequestTime == 0 {
		t.Errorf("bad request time: %d", wm.RequestTime)
	}
}

func TestDefaultConfig_env(t *testing.T) {
	t.Parallel()
	url := "http://1.2.3.4:5678"
	auth := []string{"nomaduser", "12345"}
	region := "test"
	namespace := "dev"
	token := "foobar"

	os.Setenv("NOMAD_ADDR", url)
	defer os.Setenv("NOMAD_ADDR", "")

	os.Setenv("NOMAD_REGION", region)
	defer os.Setenv("NOMAD_REGION", "")

	os.Setenv("NOMAD_NAMESPACE", namespace)
	defer os.Setenv("NOMAD_NAMESPACE", "")

	os.Setenv("NOMAD_HTTP_AUTH", strings.Join(auth, ":"))
	defer os.Setenv("NOMAD_HTTP_AUTH", "")

	os.Setenv("NOMAD_TOKEN", token)
	defer os.Setenv("NOMAD_TOKEN", "")

	config := DefaultConfig()

	if config.Address != url {
		t.Errorf("expected %q to be %q", config.Address, url)
	}

	if config.Region != region {
		t.Errorf("expected %q to be %q", config.Region, region)
	}

	if config.Namespace != namespace {
		t.Errorf("expected %q to be %q", config.Namespace, namespace)
	}

	if config.HttpAuth.Username != auth[0] {
		t.Errorf("expected %q to be %q", config.HttpAuth.Username, auth[0])
	}

	if config.HttpAuth.Password != auth[1] {
		t.Errorf("expected %q to be %q", config.HttpAuth.Password, auth[1])
	}

	if config.SecretID != token {
		t.Errorf("Expected %q to be %q", config.SecretID, token)
	}
}

func TestSetQueryOptions(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	r, _ := c.newRequest("GET", "/v1/jobs")
	q := &QueryOptions{
		Region:     "foo",
		Namespace:  "bar",
		AllowStale: true,
		WaitIndex:  1000,
		WaitTime:   100 * time.Second,
		SecretID:   "foobar",
	}
	r.setQueryOptions(q)

	if r.params.Get("region") != "foo" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("namespace") != "bar" {
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
	if r.token != "foobar" {
		t.Fatalf("bad: %v", r.token)
	}
}

func TestSetWriteOptions(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	r, _ := c.newRequest("GET", "/v1/jobs")
	q := &WriteOptions{
		Region:    "foo",
		Namespace: "bar",
		SecretID:  "foobar",
	}
	r.setWriteOptions(q)

	if r.params.Get("region") != "foo" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("namespace") != "bar" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.token != "foobar" {
		t.Fatalf("bad: %v", r.token)
	}
}

func TestRequestToHTTP(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	r, _ := c.newRequest("DELETE", "/v1/jobs/foo")
	q := &QueryOptions{
		Region:    "foo",
		Namespace: "bar",
		SecretID:  "foobar",
	}
	r.setQueryOptions(q)
	req, err := r.toHTTP()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if req.Method != "DELETE" {
		t.Fatalf("bad: %v", req)
	}
	if req.URL.RequestURI() != "/v1/jobs/foo?namespace=bar&region=foo" {
		t.Fatalf("bad: %v", req)
	}
	if req.Header.Get("X-Nomad-Token") != "foobar" {
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
	t.Parallel()
	c, s := makeClient(t, nil, nil)
	defer s.Stop()

	r, _ := c.newRequest("PUT", "/v1/abc?foo=bar&baz=zip")
	q := &WriteOptions{
		Region:    "foo",
		Namespace: "bar",
	}
	r.setWriteOptions(q)

	req, err := r.toHTTP()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if uri := req.URL.RequestURI(); uri != "/v1/abc?baz=zip&foo=bar&namespace=bar&region=foo" {
		t.Fatalf("bad uri: %q", uri)
	}
}

func TestClient_NodeClient(t *testing.T) {
	http := "testdomain:4646"
	tlsNode := func(string, *QueryOptions) (*Node, *QueryMeta, error) {
		return &Node{
			ID:         structs.GenerateUUID(),
			Status:     "ready",
			HTTPAddr:   http,
			TLSEnabled: true,
		}, nil, nil
	}
	noTlsNode := func(string, *QueryOptions) (*Node, *QueryMeta, error) {
		return &Node{
			ID:         structs.GenerateUUID(),
			Status:     "ready",
			HTTPAddr:   http,
			TLSEnabled: false,
		}, nil, nil
	}

	optionNoRegion := &QueryOptions{}
	optionRegion := &QueryOptions{
		Region: "foo",
	}

	clientNoRegion, err := NewClient(DefaultConfig())
	assert.Nil(t, err)

	regionConfig := DefaultConfig()
	regionConfig.Region = "bar"
	clientRegion, err := NewClient(regionConfig)
	assert.Nil(t, err)

	expectedTLSAddr := fmt.Sprintf("https://%s", http)
	expectedNoTLSAddr := fmt.Sprintf("http://%s", http)

	cases := []struct {
		Node                  nodeLookup
		QueryOptions          *QueryOptions
		Client                *Client
		ExpectedAddr          string
		ExpectedRegion        string
		ExpectedTLSServerName string
	}{
		{
			Node:                  tlsNode,
			QueryOptions:          optionNoRegion,
			Client:                clientNoRegion,
			ExpectedAddr:          expectedTLSAddr,
			ExpectedRegion:        "global",
			ExpectedTLSServerName: "client.global.nomad",
		},
		{
			Node:                  tlsNode,
			QueryOptions:          optionRegion,
			Client:                clientNoRegion,
			ExpectedAddr:          expectedTLSAddr,
			ExpectedRegion:        "foo",
			ExpectedTLSServerName: "client.foo.nomad",
		},
		{
			Node:                  tlsNode,
			QueryOptions:          optionRegion,
			Client:                clientRegion,
			ExpectedAddr:          expectedTLSAddr,
			ExpectedRegion:        "foo",
			ExpectedTLSServerName: "client.foo.nomad",
		},
		{
			Node:                  tlsNode,
			QueryOptions:          optionNoRegion,
			Client:                clientRegion,
			ExpectedAddr:          expectedTLSAddr,
			ExpectedRegion:        "bar",
			ExpectedTLSServerName: "client.bar.nomad",
		},
		{
			Node:                  noTlsNode,
			QueryOptions:          optionNoRegion,
			Client:                clientNoRegion,
			ExpectedAddr:          expectedNoTLSAddr,
			ExpectedRegion:        "global",
			ExpectedTLSServerName: "",
		},
		{
			Node:                  noTlsNode,
			QueryOptions:          optionRegion,
			Client:                clientNoRegion,
			ExpectedAddr:          expectedNoTLSAddr,
			ExpectedRegion:        "foo",
			ExpectedTLSServerName: "",
		},
		{
			Node:                  noTlsNode,
			QueryOptions:          optionRegion,
			Client:                clientRegion,
			ExpectedAddr:          expectedNoTLSAddr,
			ExpectedRegion:        "foo",
			ExpectedTLSServerName: "",
		},
		{
			Node:                  noTlsNode,
			QueryOptions:          optionNoRegion,
			Client:                clientRegion,
			ExpectedAddr:          expectedNoTLSAddr,
			ExpectedRegion:        "bar",
			ExpectedTLSServerName: "",
		},
	}

	for _, c := range cases {
		name := fmt.Sprintf("%s__%s__%s", c.ExpectedAddr, c.ExpectedRegion, c.ExpectedTLSServerName)
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			nodeClient, err := c.Client.getNodeClientImpl("testID", c.QueryOptions, c.Node)
			assert.Nil(err)
			assert.Equal(c.ExpectedRegion, nodeClient.config.Region)
			assert.Equal(c.ExpectedAddr, nodeClient.config.Address)
			assert.NotNil(nodeClient.config.TLSConfig)
			assert.Equal(c.ExpectedTLSServerName, nodeClient.config.TLSConfig.TLSServerName)
		})
	}
}
