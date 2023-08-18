// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	agentconsul "github.com/hashicorp/nomad/command/agent/consul"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// fakeConsul creates an HTTP server mimicking Consul /v1/agent/self endpoint on
// the first request, and alternates between success and failure responses on
// subsequent requests
func fakeConsul(payload string) (*httptest.Server, *config.Config) {
	working := true

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if working {
			_, _ = io.WriteString(w, payload)
			working = false
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			working = true
		}
	}))

	cfg := config.DefaultConfig()
	cfg.ConsulConfig.Addr = strings.TrimPrefix(ts.URL, `http://`)
	return ts, cfg
}

func fakeConsulPayload(t *testing.T, filename string) string {
	b, err := os.ReadFile(filename)
	require.NoError(t, err)
	return string(b)
}

func newConsulFingerPrint(t *testing.T) *ConsulFingerprint {
	return NewConsulFingerprint(testlog.HCLogger(t)).(*ConsulFingerprint)
}

func TestConsulFingerprint_server(t *testing.T) {
	ci.Parallel(t)

	fp := newConsulFingerPrint(t)

	t.Run("is server", func(t *testing.T) {
		s, ok := fp.server(agentconsul.Self{
			"Config": {"Server": true},
		})
		require.True(t, ok)
		require.Equal(t, "true", s)
	})

	t.Run("is not server", func(t *testing.T) {
		s, ok := fp.server(agentconsul.Self{
			"Config": {"Server": false},
		})
		require.True(t, ok)
		require.Equal(t, "false", s)
	})

	t.Run("missing", func(t *testing.T) {
		_, ok := fp.server(agentconsul.Self{
			"Config": {},
		})
		require.False(t, ok)
	})

	t.Run("malformed", func(t *testing.T) {
		_, ok := fp.server(agentconsul.Self{
			"Config": {"Server": 9000},
		})
		require.False(t, ok)
	})
}

func TestConsulFingerprint_version(t *testing.T) {
	ci.Parallel(t)

	fp := newConsulFingerPrint(t)

	t.Run("oss", func(t *testing.T) {
		v, ok := fp.version(agentconsul.Self{
			"Config": {"Version": "v1.9.5"},
		})
		require.True(t, ok)
		require.Equal(t, "v1.9.5", v)
	})

	t.Run("ent", func(t *testing.T) {
		v, ok := fp.version(agentconsul.Self{
			"Config": {"Version": "v1.9.5+ent"},
		})
		require.True(t, ok)
		require.Equal(t, "v1.9.5+ent", v)
	})

	t.Run("missing", func(t *testing.T) {
		_, ok := fp.version(agentconsul.Self{
			"Config": {},
		})
		require.False(t, ok)
	})

	t.Run("malformed", func(t *testing.T) {
		_, ok := fp.version(agentconsul.Self{
			"Config": {"Version": 9000},
		})
		require.False(t, ok)
	})
}

func TestConsulFingerprint_sku(t *testing.T) {
	ci.Parallel(t)

	fp := newConsulFingerPrint(t)

	t.Run("oss", func(t *testing.T) {
		s, ok := fp.sku(agentconsul.Self{
			"Config": {"Version": "v1.9.5"},
		})
		require.True(t, ok)
		require.Equal(t, "oss", s)
	})

	t.Run("oss dev", func(t *testing.T) {
		s, ok := fp.sku(agentconsul.Self{
			"Config": {"Version": "v1.9.5-dev"},
		})
		require.True(t, ok)
		require.Equal(t, "oss", s)
	})

	t.Run("ent", func(t *testing.T) {
		s, ok := fp.sku(agentconsul.Self{
			"Config": {"Version": "v1.9.5+ent"},
		})
		require.True(t, ok)
		require.Equal(t, "ent", s)
	})

	t.Run("ent dev", func(t *testing.T) {
		s, ok := fp.sku(agentconsul.Self{
			"Config": {"Version": "v1.9.5+ent-dev"},
		})
		require.True(t, ok)
		require.Equal(t, "ent", s)
	})

	t.Run("extra spaces", func(t *testing.T) {
		v, ok := fp.sku(agentconsul.Self{
			"Config": {"Version": "   v1.9.5\n"},
		})
		require.True(t, ok)
		require.Equal(t, "oss", v)
	})

	t.Run("missing", func(t *testing.T) {
		_, ok := fp.sku(agentconsul.Self{
			"Config": {},
		})
		require.False(t, ok)
	})

	t.Run("malformed", func(t *testing.T) {
		_, ok := fp.sku(agentconsul.Self{
			"Config": {"Version": "***"},
		})
		require.False(t, ok)
	})
}

func TestConsulFingerprint_revision(t *testing.T) {
	ci.Parallel(t)

	fp := newConsulFingerPrint(t)

	t.Run("ok", func(t *testing.T) {
		r, ok := fp.revision(agentconsul.Self{
			"Config": {"Revision": "3c1c22679"},
		})
		require.True(t, ok)
		require.Equal(t, "3c1c22679", r)
	})

	t.Run("malformed", func(t *testing.T) {
		_, ok := fp.revision(agentconsul.Self{
			"Config": {"Revision": 9000},
		})
		require.False(t, ok)
	})

	t.Run("missing", func(t *testing.T) {
		_, ok := fp.revision(agentconsul.Self{
			"Config": {},
		})
		require.False(t, ok)
	})
}

func TestConsulFingerprint_dc(t *testing.T) {
	ci.Parallel(t)

	fp := newConsulFingerPrint(t)

	t.Run("ok", func(t *testing.T) {
		dc, ok := fp.dc(agentconsul.Self{
			"Config": {"Datacenter": "dc1"},
		})
		require.True(t, ok)
		require.Equal(t, "dc1", dc)
	})

	t.Run("malformed", func(t *testing.T) {
		_, ok := fp.dc(agentconsul.Self{
			"Config": {"Datacenter": 9000},
		})
		require.False(t, ok)
	})

	t.Run("missing", func(t *testing.T) {
		_, ok := fp.dc(agentconsul.Self{
			"Config": {},
		})
		require.False(t, ok)
	})
}

func TestConsulFingerprint_segment(t *testing.T) {
	ci.Parallel(t)

	fp := newConsulFingerPrint(t)

	t.Run("ok", func(t *testing.T) {
		s, ok := fp.segment(agentconsul.Self{
			"Member": {"Tags": map[string]interface{}{"segment": "seg1"}},
		})
		require.True(t, ok)
		require.Equal(t, "seg1", s)
	})

	t.Run("segment missing", func(t *testing.T) {
		_, ok := fp.segment(agentconsul.Self{
			"Member": {"Tags": map[string]interface{}{}},
		})
		require.False(t, ok)
	})

	t.Run("tags missing", func(t *testing.T) {
		_, ok := fp.segment(agentconsul.Self{
			"Member": {},
		})
		require.False(t, ok)
	})

	t.Run("malformed", func(t *testing.T) {
		_, ok := fp.segment(agentconsul.Self{
			"Member": {"Tags": map[string]interface{}{"segment": 9000}},
		})
		require.False(t, ok)
	})
}

func TestConsulFingerprint_connect(t *testing.T) {
	ci.Parallel(t)

	fp := newConsulFingerPrint(t)

	t.Run("connect enabled", func(t *testing.T) {
		s, ok := fp.connect(agentconsul.Self{
			"DebugConfig": {"ConnectEnabled": true},
		})
		require.True(t, ok)
		require.Equal(t, "true", s)
	})

	t.Run("connect not enabled", func(t *testing.T) {
		s, ok := fp.connect(agentconsul.Self{
			"DebugConfig": {"ConnectEnabled": false},
		})
		require.True(t, ok)
		require.Equal(t, "false", s)
	})

	t.Run("connect missing", func(t *testing.T) {
		_, ok := fp.connect(agentconsul.Self{
			"DebugConfig": {},
		})
		require.False(t, ok)
	})
}

func TestConsulFingerprint_grpc(t *testing.T) {
	ci.Parallel(t)

	fp := newConsulFingerPrint(t)

	t.Run("grpc set pre-1.14 http", func(t *testing.T) {
		s, ok := fp.grpc("http")(agentconsul.Self{
			"Config":      {"Version": "1.13.3"},
			"DebugConfig": {"GRPCPort": 8502.0}, // JSON numbers are floats
		})
		require.True(t, ok)
		require.Equal(t, "8502", s)
	})

	t.Run("grpc disabled pre-1.14 http", func(t *testing.T) {
		s, ok := fp.grpc("http")(agentconsul.Self{
			"Config":      {"Version": "1.13.3"},
			"DebugConfig": {"GRPCPort": -1.0}, // JSON numbers are floats
		})
		require.True(t, ok)
		require.Equal(t, "-1", s)
	})

	t.Run("grpc set pre-1.14 https", func(t *testing.T) {
		s, ok := fp.grpc("https")(agentconsul.Self{
			"Config":      {"Version": "1.13.3"},
			"DebugConfig": {"GRPCPort": 8502.0}, // JSON numbers are floats
		})
		require.True(t, ok)
		require.Equal(t, "8502", s)
	})

	t.Run("grpc disabled pre-1.14 https", func(t *testing.T) {
		s, ok := fp.grpc("https")(agentconsul.Self{
			"Config":      {"Version": "1.13.3"},
			"DebugConfig": {"GRPCPort": -1.0}, // JSON numbers are floats
		})
		require.True(t, ok)
		require.Equal(t, "-1", s)
	})

	t.Run("grpc set post-1.14 http", func(t *testing.T) {
		s, ok := fp.grpc("http")(agentconsul.Self{
			"Config":      {"Version": "1.14.0"},
			"DebugConfig": {"GRPCPort": 8502.0}, // JSON numbers are floats
		})
		require.True(t, ok)
		require.Equal(t, "8502", s)
	})

	t.Run("grpc disabled post-1.14 http", func(t *testing.T) {
		s, ok := fp.grpc("http")(agentconsul.Self{
			"Config":      {"Version": "1.14.0"},
			"DebugConfig": {"GRPCPort": -1.0}, // JSON numbers are floats
		})
		require.True(t, ok)
		require.Equal(t, "-1", s)
	})

	t.Run("grpc disabled post-1.14 https", func(t *testing.T) {
		s, ok := fp.grpc("https")(agentconsul.Self{
			"Config":      {"Version": "1.14.0"},
			"DebugConfig": {"GRPCTLSPort": -1.0}, // JSON numbers are floats
		})
		require.True(t, ok)
		require.Equal(t, "-1", s)
	})

	t.Run("grpc set post-1.14 https", func(t *testing.T) {
		s, ok := fp.grpc("https")(agentconsul.Self{
			"Config":      {"Version": "1.14.0"},
			"DebugConfig": {"GRPCTLSPort": 8503.0}, // JSON numbers are floats
		})
		require.True(t, ok)
		require.Equal(t, "8503", s)
	})

	t.Run("version with extra spaces", func(t *testing.T) {
		s, ok := fp.grpc("https")(agentconsul.Self{
			"Config":      {"Version": "  1.14.0\n"},
			"DebugConfig": {"GRPCTLSPort": 8503.0}, // JSON numbers are floats
		})
		require.True(t, ok)
		require.Equal(t, "8503", s)
	})

	t.Run("grpc missing http", func(t *testing.T) {
		_, ok := fp.grpc("http")(agentconsul.Self{
			"DebugConfig": {},
		})
		require.False(t, ok)
	})

	t.Run("grpc missing https", func(t *testing.T) {
		_, ok := fp.grpc("https")(agentconsul.Self{
			"DebugConfig": {},
		})
		require.False(t, ok)
	})
}

func TestConsulFingerprint_namespaces(t *testing.T) {
	ci.Parallel(t)

	fp := newConsulFingerPrint(t)

	t.Run("supports namespaces", func(t *testing.T) {
		value, ok := fp.namespaces(agentconsul.Self{
			"Stats": {"license": map[string]interface{}{"features": "Automated Backups, Automated Upgrades, Enhanced Read Scalability, Network Segments, Redundancy Zone, Advanced Network Federation, Namespaces, SSO, Audit Logging"}},
		})
		require.True(t, ok)
		require.Equal(t, "true", value)
	})

	t.Run("no namespaces", func(t *testing.T) {
		value, ok := fp.namespaces(agentconsul.Self{
			"Stats": {"license": map[string]interface{}{"features": "Automated Backups, Automated Upgrades, Enhanced Read Scalability, Network Segments, Redundancy Zone, Advanced Network Federation, SSO, Audit Logging"}},
		})
		require.True(t, ok)
		require.Equal(t, "false", value)

	})

	t.Run("stats missing", func(t *testing.T) {
		value, ok := fp.namespaces(agentconsul.Self{})
		require.True(t, ok)
		require.Equal(t, "false", value)
	})

	t.Run("license missing", func(t *testing.T) {
		value, ok := fp.namespaces(agentconsul.Self{"Stats": {}})
		require.True(t, ok)
		require.Equal(t, "false", value)
	})

	t.Run("features missing", func(t *testing.T) {
		value, ok := fp.namespaces(agentconsul.Self{"Stats": {"license": map[string]interface{}{}}})
		require.True(t, ok)
		require.Equal(t, "false", value)
	})
}

func TestConsulFingerprint_Fingerprint_oss(t *testing.T) {
	ci.Parallel(t)

	cf := newConsulFingerPrint(t)

	ts, cfg := fakeConsul(fakeConsulPayload(t, "test_fixtures/consul/agent_self_ce.json"))
	defer ts.Close()

	node := &structs.Node{Attributes: make(map[string]string)}

	// consul not available before first run
	require.Equal(t, consulUnavailable, cf.lastState)

	// execute first query with good response
	var resp FingerprintResponse
	err := cf.Fingerprint(&FingerprintRequest{Config: cfg, Node: node}, &resp)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"consul.datacenter":    "dc1",
		"consul.revision":      "3c1c22679",
		"consul.segment":       "seg1",
		"consul.server":        "true",
		"consul.sku":           "oss",
		"consul.version":       "1.9.5",
		"consul.connect":       "true",
		"consul.grpc":          "8502",
		"consul.ft.namespaces": "false",
		"unique.consul.name":   "HAL9000",
	}, resp.Attributes)
	require.True(t, resp.Detected)

	// consul now available
	require.Equal(t, consulAvailable, cf.lastState)

	var resp2 FingerprintResponse

	// pretend attributes set for failing request
	node.Attributes["consul.datacenter"] = "foo"
	node.Attributes["consul.revision"] = "foo"
	node.Attributes["consul.segment"] = "foo"
	node.Attributes["consul.server"] = "foo"
	node.Attributes["consul.sku"] = "foo"
	node.Attributes["consul.version"] = "foo"
	node.Attributes["consul.connect"] = "foo"
	node.Attributes["connect.grpc"] = "foo"
	node.Attributes["unique.consul.name"] = "foo"

	// execute second query with error
	err2 := cf.Fingerprint(&FingerprintRequest{Config: cfg, Node: node}, &resp2)
	require.NoError(t, err2)         // does not return error
	require.Nil(t, resp2.Attributes) // attributes unset so they don't change
	require.True(t, resp.Detected)   // never downgrade

	// consul no longer available
	require.Equal(t, consulUnavailable, cf.lastState)

	// execute third query no error
	var resp3 FingerprintResponse
	err3 := cf.Fingerprint(&FingerprintRequest{Config: cfg, Node: node}, &resp3)
	require.NoError(t, err3)
	require.Equal(t, map[string]string{
		"consul.datacenter":    "dc1",
		"consul.revision":      "3c1c22679",
		"consul.segment":       "seg1",
		"consul.server":        "true",
		"consul.sku":           "oss",
		"consul.version":       "1.9.5",
		"consul.connect":       "true",
		"consul.grpc":          "8502",
		"consul.ft.namespaces": "false",
		"unique.consul.name":   "HAL9000",
	}, resp3.Attributes)

	// consul now available again
	require.Equal(t, consulAvailable, cf.lastState)
	require.True(t, resp.Detected)
}

func TestConsulFingerprint_Fingerprint_ent(t *testing.T) {
	ci.Parallel(t)

	cf := newConsulFingerPrint(t)

	ts, cfg := fakeConsul(fakeConsulPayload(t, "test_fixtures/consul/agent_self_ent.json"))
	defer ts.Close()

	node := &structs.Node{Attributes: make(map[string]string)}

	// consul not available before first run
	require.Equal(t, consulUnavailable, cf.lastState)

	// execute first query with good response
	var resp FingerprintResponse
	err := cf.Fingerprint(&FingerprintRequest{Config: cfg, Node: node}, &resp)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"consul.datacenter":    "dc1",
		"consul.revision":      "22ce6c6ad",
		"consul.segment":       "seg1",
		"consul.server":        "true",
		"consul.sku":           "ent",
		"consul.version":       "1.9.5+ent",
		"consul.ft.namespaces": "true",
		"consul.connect":       "true",
		"consul.grpc":          "8502",
		"unique.consul.name":   "HAL9000",
	}, resp.Attributes)
	require.True(t, resp.Detected)

	// consul now available
	require.Equal(t, consulAvailable, cf.lastState)

	var resp2 FingerprintResponse

	// pretend attributes set for failing request
	node.Attributes["consul.datacenter"] = "foo"
	node.Attributes["consul.revision"] = "foo"
	node.Attributes["consul.segment"] = "foo"
	node.Attributes["consul.server"] = "foo"
	node.Attributes["consul.sku"] = "foo"
	node.Attributes["consul.version"] = "foo"
	node.Attributes["consul.ft.namespaces"] = "foo"
	node.Attributes["consul.connect"] = "foo"
	node.Attributes["connect.grpc"] = "foo"
	node.Attributes["unique.consul.name"] = "foo"

	// execute second query with error
	err2 := cf.Fingerprint(&FingerprintRequest{Config: cfg, Node: node}, &resp2)
	require.NoError(t, err2)         // does not return error
	require.Nil(t, resp2.Attributes) // attributes unset so they don't change
	require.True(t, resp.Detected)   // never downgrade

	// consul no longer available
	require.Equal(t, consulUnavailable, cf.lastState)

	// execute third query no error
	var resp3 FingerprintResponse
	err3 := cf.Fingerprint(&FingerprintRequest{Config: cfg, Node: node}, &resp3)
	require.NoError(t, err3)
	require.Equal(t, map[string]string{
		"consul.datacenter":    "dc1",
		"consul.revision":      "22ce6c6ad",
		"consul.segment":       "seg1",
		"consul.server":        "true",
		"consul.sku":           "ent",
		"consul.version":       "1.9.5+ent",
		"consul.ft.namespaces": "true",
		"consul.connect":       "true",
		"consul.grpc":          "8502",
		"unique.consul.name":   "HAL9000",
	}, resp3.Attributes)

	// consul now available again
	require.Equal(t, consulAvailable, cf.lastState)
	require.True(t, resp.Detected)
}
