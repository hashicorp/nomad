package fingerprint

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestEnvAWSFingerprint_nonAws(t *testing.T) {
	f := NewEnvAWSFingerprint(testlog.HCLogger(t))
	f.(*EnvAWSFingerprint).endpoint = "http://127.0.0.1/latest"

	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	require.NoError(t, err)
	require.Empty(t, response.Attributes)
}

func TestEnvAWSFingerprint_aws(t *testing.T) {
	endpoint, cleanup := startFakeEC2Metadata(t)
	defer cleanup()

	f := NewEnvAWSFingerprint(testlog.HCLogger(t))
	f.(*EnvAWSFingerprint).endpoint = endpoint

	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	require.NoError(t, err)

	keys := []string{
		"platform.aws.ami-id",
		"unique.platform.aws.hostname",
		"unique.platform.aws.instance-id",
		"platform.aws.instance-type",
		"unique.platform.aws.local-hostname",
		"unique.platform.aws.local-ipv4",
		"unique.platform.aws.public-hostname",
		"unique.platform.aws.public-ipv4",
		"platform.aws.placement.availability-zone",
		"unique.network.ip-address",
	}

	for _, k := range keys {
		assertNodeAttributeContains(t, response.Attributes, k)
	}

	require.NotEmpty(t, response.Links)

	// confirm we have at least instance-id and ami-id
	for _, k := range []string{"aws.ec2"} {
		assertNodeLinksContains(t, response.Links, k)
	}
}

func TestNetworkFingerprint_AWS(t *testing.T) {
	endpoint, cleanup := startFakeEC2Metadata(t)
	defer cleanup()

	f := NewEnvAWSFingerprint(testlog.HCLogger(t))
	f.(*EnvAWSFingerprint).endpoint = endpoint

	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	require.NoError(t, err)

	assertNodeAttributeContains(t, response.Attributes, "unique.network.ip-address")

	require.NotNil(t, response.NodeResources)
	require.Len(t, response.NodeResources.Networks, 1)

	// Test at least the first Network Resource
	net := response.NodeResources.Networks[0]
	require.NotEmpty(t, net.IP, "Expected Network Resource to have an IP")
	require.NotEmpty(t, net.CIDR, "Expected Network Resource to have a CIDR")
	require.NotEmpty(t, net.Device, "Expected Network Resource to have a Device Name")
}

func TestNetworkFingerprint_AWS_network(t *testing.T) {
	endpoint, cleanup := startFakeEC2Metadata(t)
	defer cleanup()

	f := NewEnvAWSFingerprint(testlog.HCLogger(t))
	f.(*EnvAWSFingerprint).endpoint = endpoint

	{
		node := &structs.Node{
			Attributes: make(map[string]string),
		}

		request := &FingerprintRequest{Config: &config.Config{}, Node: node}
		var response FingerprintResponse
		err := f.Fingerprint(request, &response)
		require.NoError(t, err)

		require.True(t, response.Detected, "expected response to be applicable")

		assertNodeAttributeContains(t, response.Attributes, "unique.network.ip-address")

		require.NotNil(t, response.NodeResources)
		require.Len(t, response.NodeResources.Networks, 1)

		// Test at least the first Network Resource
		net := response.NodeResources.Networks[0]
		require.NotEmpty(t, net.IP, "Expected Network Resource to have an IP")
		require.NotEmpty(t, net.CIDR, "Expected Network Resource to have a CIDR")
		require.NotEmpty(t, net.Device, "Expected Network Resource to have a Device Name")
		require.Equal(t, 1000, net.MBits)
	}

	// Try again this time setting a network speed in the config
	{
		node := &structs.Node{
			Attributes: make(map[string]string),
		}

		cfg := &config.Config{
			NetworkSpeed: 10,
		}

		request := &FingerprintRequest{Config: cfg, Node: node}
		var response FingerprintResponse
		err := f.Fingerprint(request, &response)
		require.NoError(t, err)

		assertNodeAttributeContains(t, response.Attributes, "unique.network.ip-address")

		require.NotNil(t, response.NodeResources)
		require.Len(t, response.NodeResources.Networks, 1)

		// Test at least the first Network Resource
		net := response.NodeResources.Networks[0]
		require.NotEmpty(t, net.IP, "Expected Network Resource to have an IP")
		require.NotEmpty(t, net.CIDR, "Expected Network Resource to have a CIDR")
		require.NotEmpty(t, net.Device, "Expected Network Resource to have a Device Name")
		require.Equal(t, 10, net.MBits)
	}
}

/// Utility functions for tests

func startFakeEC2Metadata(t *testing.T) (endpoint string, cleanup func()) {
	routes := routes{}
	if err := json.Unmarshal([]byte(aws_routes), &routes); err != nil {
		t.Fatalf("Failed to unmarshal JSON in AWS ENV test: %s", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, e := range routes.Endpoints {
			if r.RequestURI == e.Uri {
				w.Header().Set("Content-Type", e.ContentType)
				fmt.Fprintln(w, e.Body)
			}
		}
	}))

	return ts.URL + "/latest", ts.Close
}

type routes struct {
	Endpoints []*endpoint `json:"endpoints"`
}
type endpoint struct {
	Uri         string `json:"uri"`
	ContentType string `json:"content-type"`
	Body        string `json:"body"`
}

const aws_routes = `
{
  "endpoints": [
    {
      "uri": "/latest/meta-data/ami-id",
      "content-type": "text/plain",
      "body": "ami-1234"
    },
    {
      "uri": "/latest/meta-data/hostname",
      "content-type": "text/plain",
      "body": "ip-10-0-0-207.us-west-2.compute.internal"
    },
    {
      "uri": "/latest/meta-data/placement/availability-zone",
      "content-type": "text/plain",
      "body": "us-west-2a"
    },
    {
      "uri": "/latest/meta-data/instance-id",
      "content-type": "text/plain",
      "body": "i-b3ba3875"
    },
    {
      "uri": "/latest/meta-data/instance-type",
      "content-type": "text/plain",
      "body": "m3.2xlarge"
    },
    {
      "uri": "/latest/meta-data/local-hostname",
      "content-type": "text/plain",
      "body": "ip-10-0-0-207.us-west-2.compute.internal"
    },
    {
      "uri": "/latest/meta-data/local-ipv4",
      "content-type": "text/plain",
      "body": "10.0.0.207"
    },
    {
      "uri": "/latest/meta-data/public-hostname",
      "content-type": "text/plain",
      "body": "ec2-54-191-117-175.us-west-2.compute.amazonaws.com"
    },
    {
      "uri": "/latest/meta-data/public-ipv4",
      "content-type": "text/plain",
      "body": "54.191.117.175"
    }
  ]
}
`
