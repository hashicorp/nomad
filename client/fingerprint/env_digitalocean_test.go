// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestDigitalOceanFingerprint_nonDigitalOcean(t *testing.T) {

	t.Setenv("DO_ENV_URL", "http://127.0.0.1/metadata/v1/")
	f := NewEnvDigitalOceanFingerprint(testlog.HCLogger(t))
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if response.Detected {
		t.Fatalf("expected response to not be applicable")
	}

	if len(response.Attributes) > 0 {
		t.Fatalf("Should have zero attributes without test server")
	}
}

func TestFingerprint_DigitalOcean(t *testing.T) {

	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	// configure mock server with fixture routes, data
	routes := routes{}
	if err := json.Unmarshal([]byte(DO_routes), &routes); err != nil {
		t.Fatalf("Failed to unmarshal JSON in DO ENV test: %s", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uavalue, ok := r.Header["User-Agent"]
		if !ok {
			t.Fatal("User-Agent not present in HTTP request header")
		}
		if !strings.Contains(uavalue[0], "Nomad/") {
			t.Fatalf("Expected User-Agent to contain Nomad/, got %s", uavalue[0])
		}

		uri := r.RequestURI
		if r.URL.RawQuery != "" {
			uri = strings.Replace(uri, "?"+r.URL.RawQuery, "", 1)
		}

		found := false
		for _, e := range routes.Endpoints {
			if uri == e.Uri {
				w.Header().Set("Content-Type", e.ContentType)
				fmt.Fprintln(w, e.Body)
				found = true
			}
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()
	t.Setenv("DO_ENV_URL", ts.URL+"/metadata/v1/")
	f := NewEnvDigitalOceanFingerprint(testlog.HCLogger(t))

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	assert.NoError(t, err)
	assert.True(t, response.Detected, "expected response to be applicable")

	keys := []string{
		"unique.platform.digitalocean.id",
		"unique.platform.digitalocean.hostname",
		"platform.digitalocean.region",
		"unique.platform.digitalocean.private-ipv4",
		"unique.platform.digitalocean.public-ipv4",
		"unique.platform.digitalocean.public-ipv6",
		"unique.platform.digitalocean.mac",
	}

	for _, k := range keys {
		assertNodeAttributeContains(t, response.Attributes, k)
	}

	assert.NotEmpty(t, response.Links, "Empty links for Node in DO Fingerprint test")

	// Make sure Links contains the DO ID.
	for _, k := range []string{"digitalocean"} {
		assertNodeLinksContains(t, response.Links, k)
	}

	assertNodeAttributeEquals(t, response.Attributes, "unique.platform.digitalocean.id", "13f56399-bd52-4150-9748-7190aae1ff21")
	assertNodeAttributeEquals(t, response.Attributes, "unique.platform.digitalocean.hostname", "demo01.internal")
	assertNodeAttributeEquals(t, response.Attributes, "platform.digitalocean.region", "sfo3")
	assertNodeAttributeEquals(t, response.Attributes, "unique.platform.digitalocean.private-ipv4", "10.1.0.4")
	assertNodeAttributeEquals(t, response.Attributes, "unique.platform.digitalocean.mac", "000D3AF806EC")
	assertNodeAttributeEquals(t, response.Attributes, "unique.platform.digitalocean.public-ipv4", "100.100.100.100")
	assertNodeAttributeEquals(t, response.Attributes, "unique.platform.digitalocean.public-ipv6", "c99c:8ac5:3112:204b:48b0:41aa:e085:d11a")
}

const DO_routes = `
{
  "endpoints": [
	{
	  "uri": "/metadata/v1/region",
	  "content-type": "text/plain",
      "body": "sfo3"
	},
	{
		"uri": "/metadata/v1/hostname",
		"content-type": "text/plain",
		"body": "demo01.internal"
	},
	{
		"uri": "/metadata/v1/id",
		"content-type": "text/plain",
		"body": "13f56399-bd52-4150-9748-7190aae1ff21"
	},
	{
		"uri": "/metadata/v1/interfaces/private/0/ipv4/address",
		"content-type": "text/plain",
		"body": "10.1.0.4"
	},
	{
		"uri": "/metadata/v1/interfaces/public/0/mac",
		"content-type": "text/plain",
		"body": "000D3AF806EC"
	},
  {
		"uri": "/metadata/v1/interfaces/public/0/ipv4/address",
		"content-type": "text/plain",
		"body": "100.100.100.100"
	},
  {
		"uri": "/metadata/v1/interfaces/public/0/ipv6/address",
		"content-type": "text/plain",
		"body": "c99c:8ac5:3112:204b:48b0:41aa:e085:d11a"
	}
  ]
}
`
