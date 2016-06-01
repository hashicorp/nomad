package fingerprint

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

func TestConsulFingerprint(t *testing.T) {
	addr := os.Getenv("CONSUL_HTTP_ADDR")
	if addr == "" {
		t.Skipf("No consul process running, skipping test")
	}

	fp := NewConsulFingerprint(testLogger())
	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, mockConsulResponse)
	}))
	defer ts.Close()

	config := config.DefaultConfig()

	ok, err := fp.Fingerprint(config, node)
	if err != nil {
		t.Fatalf("Failed to fingerprint: %s", err)
	}
	if !ok {
		t.Fatalf("Failed to apply node attributes")
	}

	assertNodeAttributeContains(t, node, "consul.server")
	assertNodeAttributeContains(t, node, "consul.version")
	assertNodeAttributeContains(t, node, "consul.revision")
	assertNodeAttributeContains(t, node, "unique.consul.name")
	assertNodeAttributeContains(t, node, "consul.datacenter")

	if _, ok := node.Links["consul"]; !ok {
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
