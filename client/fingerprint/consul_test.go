package fingerprint

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestConsulFingerprint(t *testing.T) {
	fp := NewConsulFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, mockConsulResponse)
	}))
	defer ts.Close()

	conf := config.DefaultConfig()
	conf.ConsulConfig.Addr = strings.TrimPrefix(ts.URL, "http://")

	request := &cstructs.FingerprintRequest{Config: conf, Node: node}
	var response cstructs.FingerprintResponse
	err := fp.Fingerprint(request, &response)
	if err != nil {
		t.Fatalf("Failed to fingerprint: %s", err)
	}

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	assertNodeAttributeContains(t, response.Attributes, "consul.server")
	assertNodeAttributeContains(t, response.Attributes, "consul.version")
	assertNodeAttributeContains(t, response.Attributes, "consul.revision")
	assertNodeAttributeContains(t, response.Attributes, "unique.consul.name")
	assertNodeAttributeContains(t, response.Attributes, "consul.datacenter")

	if _, ok := response.Links["consul"]; !ok {
		t.Errorf("Expected a link to consul, none found")
	}
}

// Taken from tryconsul using consul release 0.5.2
const mockConsulResponse = `
{
  "Config": {
    "Bootstrap": false,
    "BootstrapExpect": 3,
    "Server": true,
    "Datacenter": "vagrant",
    "DataDir": "/var/lib/consul",
    "DNSRecursor": "",
    "DNSRecursors": [],
    "DNSConfig": {
      "NodeTTL": 0,
      "ServiceTTL": null,
      "AllowStale": false,
      "EnableTruncate": false,
      "MaxStale": 5000000000,
      "OnlyPassing": false
    },
    "Domain": "consul.",
    "LogLevel": "INFO",
    "NodeName": "consul2",
    "ClientAddr": "0.0.0.0",
    "BindAddr": "0.0.0.0",
    "AdvertiseAddr": "172.16.59.133",
    "AdvertiseAddrWan": "172.16.59.133",
    "Ports": {
      "DNS": 8600,
      "HTTP": 8500,
      "HTTPS": -1,
      "RPC": 8400,
      "SerfLan": 8301,
      "SerfWan": 8302,
      "Server": 8300
    },
    "Addresses": {
      "DNS": "",
      "HTTP": "",
      "HTTPS": "",
      "RPC": ""
    },
    "LeaveOnTerm": false,
    "SkipLeaveOnInt": false,
    "StatsiteAddr": "",
    "StatsitePrefix": "consul",
    "StatsdAddr": "",
    "Protocol": 2,
    "EnableDebug": false,
    "VerifyIncoming": false,
    "VerifyOutgoing": false,
    "VerifyServerHostname": false,
    "CAFile": "",
    "CertFile": "",
    "KeyFile": "",
    "ServerName": "",
    "StartJoin": [],
    "StartJoinWan": [],
    "RetryJoin": [],
    "RetryMaxAttempts": 0,
    "RetryIntervalRaw": "",
    "RetryJoinWan": [],
    "RetryMaxAttemptsWan": 0,
    "RetryIntervalWanRaw": "",
    "UiDir": "/opt/consul-ui",
    "PidFile": "",
    "EnableSyslog": true,
    "SyslogFacility": "LOCAL0",
    "RejoinAfterLeave": false,
    "CheckUpdateInterval": 300000000000,
    "ACLDatacenter": "",
    "ACLTTL": 30000000000,
    "ACLTTLRaw": "",
    "ACLDefaultPolicy": "allow",
    "ACLDownPolicy": "extend-cache",
    "Watches": null,
    "DisableRemoteExec": false,
    "DisableUpdateCheck": false,
    "DisableAnonymousSignature": false,
    "HTTPAPIResponseHeaders": null,
    "AtlasInfrastructure": "",
    "AtlasJoin": false,
    "Revision": "9a9cc9341bb487651a0399e3fc5e1e8a42e62dd9+CHANGES",
    "Version": "0.5.2",
    "VersionPrerelease": "",
    "UnixSockets": {
      "Usr": "",
      "Grp": "",
      "Perms": ""
    },
    "SessionTTLMin": 0,
    "SessionTTLMinRaw": ""
  },
  "Member": {
    "Name": "consul2",
    "Addr": "172.16.59.133",
    "Port": 8301,
    "Tags": {
      "build": "0.5.2:9a9cc934",
      "dc": "vagrant",
      "expect": "3",
      "port": "8300",
      "role": "consul",
      "vsn": "2"
    },
    "Status": 1,
    "ProtocolMin": 1,
    "ProtocolMax": 2,
    "ProtocolCur": 2,
    "DelegateMin": 2,
    "DelegateMax": 4,
    "DelegateCur": 4
  }
}
`

// TestConsulFingerprint_UnexpectedResponse asserts that the Consul
// fingerprinter does not panic when it encounters an unexpected response.
// See https://github.com/hashicorp/nomad/issues/3326
func TestConsulFingerprint_UnexpectedResponse(t *testing.T) {
	assert := assert.New(t)
	fp := NewConsulFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, "{}")
	}))
	defer ts.Close()

	conf := config.DefaultConfig()
	conf.ConsulConfig.Addr = strings.TrimPrefix(ts.URL, "http://")

	request := &cstructs.FingerprintRequest{Config: conf, Node: node}
	var response cstructs.FingerprintResponse
	err := fp.Fingerprint(request, &response)
	assert.Nil(err)

	if !response.Detected {
		t.Fatalf("expected response to be applicable")
	}

	attrs := []string{
		"consul.server",
		"consul.version",
		"consul.revision",
		"unique.consul.name",
		"consul.datacenter",
	}

	for _, attr := range attrs {
		if v, ok := response.Attributes[attr]; ok {
			t.Errorf("unexpected node attribute %q with vlaue %q", attr, v)
		}
	}

	if v, ok := response.Links["consul"]; ok {
		t.Errorf("Unexpected link to consul: %v", v)
	}
}
