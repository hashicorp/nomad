// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestEnvAWSFingerprint_nonAws(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)

	endpoint, cleanup := startFakeEC2Metadata(t, awsStubs)
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
		"platform.aws.instance-life-cycle",
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
	ci.Parallel(t)

	endpoint, cleanup := startFakeEC2Metadata(t, awsStubs)
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
	ci.Parallel(t)

	endpoint, cleanup := startFakeEC2Metadata(t, awsStubs)
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

func TestNetworkFingerprint_AWS_NoNetwork(t *testing.T) {
	ci.Parallel(t)

	endpoint, cleanup := startFakeEC2Metadata(t, noNetworkAWSStubs)
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

	require.True(t, response.Detected, "expected response to be applicable")

	require.Equal(t, "ami-1234", response.Attributes["platform.aws.ami-id"])

	require.Nil(t, response.NodeResources.Networks)
}

func TestNetworkFingerprint_AWS_IncompleteImitation(t *testing.T) {
	ci.Parallel(t)

	endpoint, cleanup := startFakeEC2Metadata(t, incompleteAWSImitationStubs)
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

	require.False(t, response.Detected, "expected response not to be applicable")

	require.NotContains(t, response.Attributes, "platform.aws.ami-id")
	require.Nil(t, response.NodeResources)
}

func TestCPUFingerprint_AWS_InstanceFound(t *testing.T) {
	ci.Parallel(t)

	endpoint, cleanup := startFakeEC2Metadata(t, awsStubs)
	defer cleanup()

	f := NewEnvAWSFingerprint(testlog.HCLogger(t))
	f.(*EnvAWSFingerprint).endpoint = endpoint

	node := &structs.Node{Attributes: make(map[string]string)}

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	require.NoError(t, err)
	require.True(t, response.Detected)
}

func TestCPUFingerprint_AWS_InstanceNotFound(t *testing.T) {
	ci.Parallel(t)

	endpoint, cleanup := startFakeEC2Metadata(t, unknownInstanceType)
	defer cleanup()

	f := NewEnvAWSFingerprint(testlog.HCLogger(t))
	f.(*EnvAWSFingerprint).endpoint = endpoint

	node := &structs.Node{Attributes: make(map[string]string)}

	request := &FingerprintRequest{Config: &config.Config{}, Node: node}
	var response FingerprintResponse
	err := f.Fingerprint(request, &response)
	require.NoError(t, err)
	require.True(t, response.Detected)
}

/// Utility functions for tests

func startFakeEC2Metadata(t *testing.T, endpoints []endpoint) (endpoint string, cleanup func()) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, e := range endpoints {
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

// awsStubs mimics normal EC2 instance metadata
var awsStubs = []endpoint{
	{
		Uri:         "/latest/meta-data/ami-id",
		ContentType: "text/plain",
		Body:        "ami-1234",
	},
	{
		Uri:         "/latest/meta-data/hostname",
		ContentType: "text/plain",
		Body:        "ip-10-0-0-207.us-west-2.compute.internal",
	},
	{
		Uri:         "/latest/meta-data/placement/availability-zone",
		ContentType: "text/plain",
		Body:        "us-west-2a",
	},
	{
		Uri:         "/latest/meta-data/instance-id",
		ContentType: "text/plain",
		Body:        "i-b3ba3875",
	},
	{
		Uri:         "/latest/meta-data/instance-life-cycle",
		ContentType: "text/plain",
		Body:        "on-demand",
	},
	{
		Uri:         "/latest/meta-data/instance-type",
		ContentType: "text/plain",
		Body:        "t3a.2xlarge",
	},
	{
		Uri:         "/latest/meta-data/local-hostname",
		ContentType: "text/plain",
		Body:        "ip-10-0-0-207.us-west-2.compute.internal",
	},
	{
		Uri:         "/latest/meta-data/local-ipv4",
		ContentType: "text/plain",
		Body:        "10.0.0.207",
	},
	{
		Uri:         "/latest/meta-data/public-hostname",
		ContentType: "text/plain",
		Body:        "ec2-54-191-117-175.us-west-2.compute.amazonaws.com",
	},
	{
		Uri:         "/latest/meta-data/public-ipv4",
		ContentType: "text/plain",
		Body:        "54.191.117.175",
	},
	{
		Uri:         "/latest/meta-data/mac",
		ContentType: "text/plain",
		Body:        "0a:20:d2:42:b3:55",
	},
}

var unknownInstanceType = []endpoint{
	{
		Uri:         "/latest/meta-data/ami-id",
		ContentType: "text/plain",
		Body:        "ami-1234",
	},
	{
		Uri:         "/latest/meta-data/hostname",
		ContentType: "text/plain",
		Body:        "ip-10-0-0-207.us-west-2.compute.internal",
	},
	{
		Uri:         "/latest/meta-data/placement/availability-zone",
		ContentType: "text/plain",
		Body:        "us-west-2a",
	},
	{
		Uri:         "/latest/meta-data/instance-id",
		ContentType: "text/plain",
		Body:        "i-b3ba3875",
	},
	{
		Uri:         "/latest/meta-data/instance-life-cycle",
		ContentType: "text/plain",
		Body:        "on-demand",
	},
	{
		Uri:         "/latest/meta-data/instance-type",
		ContentType: "text/plain",
		Body:        "xyz123.uber",
	},
}

// noNetworkAWSStubs mimics an EC2 instance but without local ip address
// may happen in environments with odd EC2 Metadata emulation
var noNetworkAWSStubs = []endpoint{
	{
		Uri:         "/latest/meta-data/ami-id",
		ContentType: "text/plain",
		Body:        "ami-1234",
	},
	{
		Uri:         "/latest/meta-data/hostname",
		ContentType: "text/plain",
		Body:        "ip-10-0-0-207.us-west-2.compute.internal",
	},
	{
		Uri:         "/latest/meta-data/placement/availability-zone",
		ContentType: "text/plain",
		Body:        "us-west-2a",
	},
	{
		Uri:         "/latest/meta-data/instance-id",
		ContentType: "text/plain",
		Body:        "i-b3ba3875",
	},
	{
		Uri:         "/latest/meta-data/instance-life-cycle",
		ContentType: "text/plain",
		Body:        "on-demand",
	},
	{
		Uri:         "/latest/meta-data/instance-type",
		ContentType: "text/plain",
		Body:        "m3.2xlarge",
	},
	{
		Uri:         "/latest/meta-data/local-hostname",
		ContentType: "text/plain",
		Body:        "ip-10-0-0-207.us-west-2.compute.internal",
	},
	{
		Uri:         "/latest/meta-data/local-ipv4",
		ContentType: "text/plain",
		Body:        "",
	},
	{
		Uri:         "/latest/meta-data/public-hostname",
		ContentType: "text/plain",
		Body:        "ec2-54-191-117-175.us-west-2.compute.amazonaws.com",
	},
	{
		Uri:         "/latest/meta-data/public-ipv4",
		ContentType: "text/plain",
		Body:        "54.191.117.175",
	},
}

// incompleteAWSImitationsStub mimics environments where some AWS endpoints
// return empty, namely Hetzner
var incompleteAWSImitationStubs = []endpoint{
	{
		Uri:         "/latest/meta-data/hostname",
		ContentType: "text/plain",
		Body:        "ip-10-0-0-207.us-west-2.compute.internal",
	},
	{
		Uri:         "/latest/meta-data/instance-id",
		ContentType: "text/plain",
		Body:        "i-b3ba3875",
	},
	{
		Uri:         "/latest/meta-data/local-ipv4",
		ContentType: "text/plain",
		Body:        "",
	},
	{
		Uri:         "/latest/meta-data/public-ipv4",
		ContentType: "text/plain",
		Body:        "54.191.117.175",
	},
}
