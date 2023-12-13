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
)

func TestAzureFingerprint_nonAzure(t *testing.T) {

	t.Setenv("AZURE_ENV_URL", "http://127.0.0.1/metadata/instance/")
	f := NewEnvAzureFingerprint(testlog.HCLogger(t))
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

func testFingerprint_Azure(t *testing.T, withExternalIp bool) {
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	// configure mock server with fixture routes, data
	routes := routes{}
	if err := json.Unmarshal([]byte(AZURE_routes), &routes); err != nil {
		t.Fatalf("Failed to unmarshal JSON in GCE ENV test: %s", err)
	}
	if withExternalIp {
		networkEndpoint := &endpoint{
			Uri:         "/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress",
			ContentType: "text/plain",
			Body:        "104.44.55.66",
		}
		routes.Endpoints = append(routes.Endpoints, networkEndpoint)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value, ok := r.Header["Metadata"]
		if !ok {
			t.Fatal("Metadata not present in HTTP request header")
		}
		if value[0] != "true" {
			t.Fatalf("Expected Metadata true, saw %s", value[0])
		}

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
	t.Setenv("AZURE_ENV_URL", ts.URL+"/metadata/instance/")
	f := NewEnvAzureFingerprint(testlog.HCLogger(t))

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	keys := []string{
		"unique.platform.azure.id",
		"unique.platform.azure.name",
		"platform.azure.location",
		"platform.azure.resource-group",
		"platform.azure.scale-set",
		"platform.azure.vm-size",
		"unique.platform.azure.local-ipv4",
		"unique.platform.azure.mac",
		"platform.azure.tag.Environment",
		"platform.azure.tag.abc",
		"unique.platform.azure.tag.foo",
	}

	for _, k := range keys {
		assertNodeAttributeContains(t, response.Attributes, k)
	}

	if len(response.Links) == 0 {
		t.Fatalf("Empty links for Node in GCE Fingerprint test")
	}

	// Make sure Links contains the GCE ID.
	for _, k := range []string{"azure"} {
		assertNodeLinksContains(t, response.Links, k)
	}

	assertNodeAttributeEquals(t, response.Attributes, "unique.platform.azure.id", "13f56399-bd52-4150-9748-7190aae1ff21")
	assertNodeAttributeEquals(t, response.Attributes, "unique.platform.azure.name", "demo01.internal")
	assertNodeAttributeEquals(t, response.Attributes, "platform.azure.location", "eastus")
	assertNodeAttributeEquals(t, response.Attributes, "platform.azure.resource-group", "myrg")
	assertNodeAttributeEquals(t, response.Attributes, "platform.azure.scale-set", "nomad-clients")
	assertNodeAttributeEquals(t, response.Attributes, "unique.platform.azure.local-ipv4", "10.1.0.4")
	assertNodeAttributeEquals(t, response.Attributes, "unique.platform.azure.mac", "000D3AF806EC")
	assertNodeAttributeEquals(t, response.Attributes, "platform.azure.tag.Environment", "Test")
	assertNodeAttributeEquals(t, response.Attributes, "platform.azure.tag.abc", "def")
	assertNodeAttributeEquals(t, response.Attributes, "unique.platform.azure.tag.foo", "true")

	if withExternalIp {
		assertNodeAttributeEquals(t, response.Attributes, "unique.platform.azure.public-ipv4", "104.44.55.66")
	} else if _, ok := response.Attributes["unique.platform.azure.public-ipv4"]; ok {
		t.Fatal("unique.platform.azure.public-ipv4 is set without an external IP")
	}

}

const AZURE_routes = `
{
  "endpoints": [
	{
	  "uri": "/metadata/instance/compute/azEnvironment",
	  "content-type": "text/plain",
      "body": "AzurePublicCloud"
	},

	{
	  "uri": "/metadata/instance/compute/location",
	  "content-type": "text/plain",
      "body": "eastus"
	},
	{
		"uri": "/metadata/instance/compute/name",
		"content-type": "text/plain",
		"body": "demo01.internal"
	},
	{
		"uri": "/metadata/instance/compute/resourceGroupName",
		"content-type": "text/plain",
		"body": "myrg"
	},
	{
		"uri": "/metadata/instance/compute/vmId",
		"content-type": "text/plain",
		"body": "13f56399-bd52-4150-9748-7190aae1ff21"
	},
	{
		"uri": "/metadata/instance/compute/vmScaleSetName",
		"content-type": "text/plain",
		"body": "nomad-clients"
	},
	{
		"uri": "/metadata/instance/compute/vmSize",
		"content-type": "text/plain",
		"body": "Standard_A1_v2"
	},
	{
		"uri": "/metadata/instance/compute/tagsList",
		"content-type": "application/json",
		"body": "[{ \"name\":\"Environment\", \"value\":\"Test\"}, { \"name\":\"abc\", \"value\":\"def\"}, { \"name\":\"unique.foo\", \"value\":\"true\"}]"
	},
	{
		"uri": "/metadata/instance/network/interface/0/ipv4/ipAddress/0/privateIpAddress",
		"content-type": "text/plain",
		"body": "10.1.0.4"
	},
	{
		"uri": "/metadata/instance/network/interface/0/macAddress",
		"content-type": "text/plain",
		"body": "000D3AF806EC"
	}
  ]
}
`

func TestFingerprint_AzureWithExternalIp(t *testing.T) {
	testFingerprint_Azure(t, true)
}

func TestFingerprint_AzureWithoutExternalIp(t *testing.T) {
	testFingerprint_Azure(t, false)
}
