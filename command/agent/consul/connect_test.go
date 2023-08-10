// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
)

var (
	testConnectNetwork = structs.Networks{{
		Mode:   "bridge",
		Device: "eth0",
		IP:     "192.168.30.1",
		DynamicPorts: []structs.Port{
			{Label: "healthPort", Value: 23100, To: 23100},
			{Label: "metricsPort", Value: 23200, To: 23200},
			{Label: "connect-proxy-redis", Value: 3000, To: 3000},
		},
	}}
	testConnectPorts = structs.AllocatedPorts{{
		Label:  "connect-proxy-redis",
		Value:  3000,
		To:     3000,
		HostIP: "192.168.30.1",
	}}
)

func TestConnect_newConnect(t *testing.T) {
	ci.Parallel(t)

	service := "redis"
	redisID := uuid.Generate()
	allocID := uuid.Generate()
	info := structs.AllocInfo{
		AllocID: allocID,
	}

	t.Run("nil", func(t *testing.T) {
		asr, err := newConnect("", structs.AllocInfo{}, "", nil, nil, nil)
		require.NoError(t, err)
		require.Nil(t, asr)
	})

	t.Run("native", func(t *testing.T) {
		asr, err := newConnect(redisID, info, service, &structs.ConsulConnect{
			Native: true,
		}, nil, nil)
		require.NoError(t, err)
		require.True(t, asr.Native)
		require.Nil(t, asr.SidecarService)
	})

	t.Run("with sidecar", func(t *testing.T) {
		asr, err := newConnect(redisID, info, service, &structs.ConsulConnect{
			Native: false,
			SidecarService: &structs.ConsulSidecarService{
				Tags: []string{"foo", "bar"},
				Port: "connect-proxy-redis",
			},
		}, testConnectNetwork, testConnectPorts)
		require.NoError(t, err)
		require.Equal(t, &api.AgentServiceRegistration{
			Tags:    []string{"foo", "bar"},
			Port:    3000,
			Address: "192.168.30.1",
			Proxy: &api.AgentServiceConnectProxyConfig{
				Config: map[string]interface{}{
					"bind_address":     "0.0.0.0",
					"bind_port":        3000,
					"envoy_stats_tags": []string{"nomad.alloc_id=" + allocID},
				},
			},
			Checks: api.AgentServiceChecks{
				{
					Name:         "Connect Sidecar Aliasing " + redisID,
					AliasService: redisID,
				},
				{
					Name:     "Connect Sidecar Listening",
					TCP:      "192.168.30.1:3000",
					Interval: "10s",
				},
			},
		}, asr.SidecarService)
	})

	t.Run("with sidecar without TCP checks", func(t *testing.T) {
		asr, err := newConnect(redisID, info, service, &structs.ConsulConnect{
			Native: false,
			SidecarService: &structs.ConsulSidecarService{
				Tags:                   []string{"foo", "bar"},
				Port:                   "connect-proxy-redis",
				DisableDefaultTCPCheck: true,
			},
		}, testConnectNetwork, testConnectPorts)
		require.NoError(t, err)
		require.Equal(t, &api.AgentServiceRegistration{
			Tags:    []string{"foo", "bar"},
			Port:    3000,
			Address: "192.168.30.1",
			Proxy: &api.AgentServiceConnectProxyConfig{
				Config: map[string]interface{}{
					"bind_address":     "0.0.0.0",
					"bind_port":        3000,
					"envoy_stats_tags": []string{"nomad.alloc_id=" + allocID},
				},
			},
			Checks: api.AgentServiceChecks{
				{
					Name:         "Connect Sidecar Aliasing " + redisID,
					AliasService: redisID,
				},
			},
		}, asr.SidecarService)
	})
}

func TestConnect_connectSidecarRegistration(t *testing.T) {
	ci.Parallel(t)

	redisID := uuid.Generate()
	allocID := uuid.Generate()
	info := structs.AllocInfo{
		AllocID: allocID,
	}

	t.Run("nil", func(t *testing.T) {
		sidecarReg, err := connectSidecarRegistration(redisID, info, nil, testConnectNetwork, testConnectPorts)
		require.NoError(t, err)
		require.Nil(t, sidecarReg)
	})

	t.Run("no service port", func(t *testing.T) {
		_, err := connectSidecarRegistration("unknown-id", info, &structs.ConsulSidecarService{
			Port: "unknown-label",
		}, testConnectNetwork, testConnectPorts)
		require.EqualError(t, err, `No port of label "unknown-label" defined`)
	})

	t.Run("bad proxy", func(t *testing.T) {
		_, err := connectSidecarRegistration(redisID, info, &structs.ConsulSidecarService{
			Port: "connect-proxy-redis",
			Proxy: &structs.ConsulProxy{
				Expose: &structs.ConsulExposeConfig{
					Paths: []structs.ConsulExposePath{{
						ListenerPort: "badPort",
					}},
				},
			},
		}, testConnectNetwork, testConnectPorts)
		require.EqualError(t, err, `No port of label "badPort" defined`)
	})

	t.Run("normal", func(t *testing.T) {
		proxy, err := connectSidecarRegistration(redisID, info, &structs.ConsulSidecarService{
			Tags: []string{"foo", "bar"},
			Port: "connect-proxy-redis",
		}, testConnectNetwork, testConnectPorts)
		require.NoError(t, err)
		require.Equal(t, &api.AgentServiceRegistration{
			Tags:    []string{"foo", "bar"},
			Port:    3000,
			Address: "192.168.30.1",
			Proxy: &api.AgentServiceConnectProxyConfig{
				Config: map[string]interface{}{
					"bind_address":     "0.0.0.0",
					"bind_port":        3000,
					"envoy_stats_tags": []string{"nomad.alloc_id=" + allocID},
				},
			},
			Checks: api.AgentServiceChecks{
				{
					Name:         "Connect Sidecar Aliasing " + redisID,
					AliasService: redisID,
				},
				{
					Name:     "Connect Sidecar Listening",
					TCP:      "192.168.30.1:3000",
					Interval: "10s",
				},
			},
		}, proxy)
	})
}

func TestConnect_connectProxy(t *testing.T) {
	ci.Parallel(t)

	allocID := uuid.Generate()
	info := structs.AllocInfo{
		AllocID: allocID,
	}

	// If the input proxy is nil, we expect the output to be a proxy with its
	// config set to default values.
	t.Run("nil proxy", func(t *testing.T) {
		proxy, err := connectSidecarProxy(info, nil, 2000, testConnectNetwork)
		require.NoError(t, err)
		require.Equal(t, &api.AgentServiceConnectProxyConfig{
			LocalServiceAddress: "",
			LocalServicePort:    0,
			Upstreams:           nil,
			Expose:              api.ExposeConfig{},
			Config: map[string]interface{}{
				"bind_address":     "0.0.0.0",
				"bind_port":        2000,
				"envoy_stats_tags": []string{"nomad.alloc_id=" + allocID},
			},
		}, proxy)
	})

	t.Run("bad proxy", func(t *testing.T) {
		_, err := connectSidecarProxy(info, &structs.ConsulProxy{
			LocalServiceAddress: "0.0.0.0",
			LocalServicePort:    2000,
			Upstreams:           nil,
			Expose: &structs.ConsulExposeConfig{
				Paths: []structs.ConsulExposePath{{
					ListenerPort: "badPort",
				}},
			},
			Config: nil,
		}, 2000, testConnectNetwork)
		require.EqualError(t, err, `No port of label "badPort" defined`)
	})

	t.Run("normal", func(t *testing.T) {
		proxy, err := connectSidecarProxy(info, &structs.ConsulProxy{
			LocalServiceAddress: "0.0.0.0",
			LocalServicePort:    2000,
			Upstreams:           nil,
			Expose: &structs.ConsulExposeConfig{
				Paths: []structs.ConsulExposePath{{
					Path:          "/health",
					Protocol:      "http",
					LocalPathPort: 8000,
					ListenerPort:  "healthPort",
				}},
			},
			Config: nil,
		}, 2000, testConnectNetwork)
		require.NoError(t, err)
		require.Equal(t, &api.AgentServiceConnectProxyConfig{
			LocalServiceAddress: "0.0.0.0",
			LocalServicePort:    2000,
			Upstreams:           nil,
			Expose: api.ExposeConfig{
				Paths: []api.ExposePath{{
					Path:          "/health",
					Protocol:      "http",
					LocalPathPort: 8000,
					ListenerPort:  23100,
				}},
			},
			Config: map[string]interface{}{
				"bind_address":     "0.0.0.0",
				"bind_port":        2000,
				"envoy_stats_tags": []string{"nomad.alloc_id=" + allocID},
			},
		}, proxy)
	})
}

func TestConnect_connectProxyExpose(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		exposeConfig, err := connectProxyExpose(nil, nil)
		require.NoError(t, err)
		require.Equal(t, api.ExposeConfig{}, exposeConfig)
	})

	t.Run("bad port", func(t *testing.T) {
		_, err := connectProxyExpose(&structs.ConsulExposeConfig{
			Paths: []structs.ConsulExposePath{{
				ListenerPort: "badPort",
			}},
		}, testConnectNetwork)
		require.EqualError(t, err, `No port of label "badPort" defined`)
	})

	t.Run("normal", func(t *testing.T) {
		expose, err := connectProxyExpose(&structs.ConsulExposeConfig{
			Paths: []structs.ConsulExposePath{{
				Path:          "/health",
				Protocol:      "http",
				LocalPathPort: 8000,
				ListenerPort:  "healthPort",
			}},
		}, testConnectNetwork)
		require.NoError(t, err)
		require.Equal(t, api.ExposeConfig{
			Checks: false,
			Paths: []api.ExposePath{{
				Path:            "/health",
				ListenerPort:    23100,
				LocalPathPort:   8000,
				Protocol:        "http",
				ParsedFromCheck: false,
			}},
		}, expose)
	})
}

func TestConnect_connectProxyExposePaths(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		upstreams, err := connectProxyExposePaths(nil, nil)
		require.NoError(t, err)
		require.Empty(t, upstreams)
	})

	t.Run("no network", func(t *testing.T) {
		original := []structs.ConsulExposePath{{Path: "/path"}}
		_, err := connectProxyExposePaths(original, nil)
		require.EqualError(t, err, `Connect only supported with exactly 1 network (found 0)`)
	})

	t.Run("normal", func(t *testing.T) {
		original := []structs.ConsulExposePath{{
			Path:          "/health",
			Protocol:      "http",
			LocalPathPort: 8000,
			ListenerPort:  "healthPort",
		}, {
			Path:          "/metrics",
			Protocol:      "grpc",
			LocalPathPort: 9500,
			ListenerPort:  "metricsPort",
		}}
		exposePaths, err := connectProxyExposePaths(original, testConnectNetwork)
		require.NoError(t, err)
		require.Equal(t, []api.ExposePath{
			{
				Path:            "/health",
				Protocol:        "http",
				LocalPathPort:   8000,
				ListenerPort:    23100,
				ParsedFromCheck: false,
			},
			{
				Path:            "/metrics",
				Protocol:        "grpc",
				LocalPathPort:   9500,
				ListenerPort:    23200,
				ParsedFromCheck: false,
			},
		}, exposePaths)
	})
}

func TestConnect_connectUpstreams(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		must.Nil(t, connectUpstreams(nil))
	})

	t.Run("not empty", func(t *testing.T) {
		must.Eq(t,
			[]api.Upstream{{
				DestinationName: "foo",
				LocalBindPort:   8000,
			}, {
				DestinationName:      "bar",
				DestinationNamespace: "ns2",
				LocalBindPort:        9000,
				Datacenter:           "dc2",
				LocalBindAddress:     "127.0.0.2",
				Config:               map[string]any{"connect_timeout_ms": 5000},
			}},
			connectUpstreams([]structs.ConsulUpstream{{
				DestinationName: "foo",
				LocalBindPort:   8000,
			}, {
				DestinationName:      "bar",
				DestinationNamespace: "ns2",
				LocalBindPort:        9000,
				Datacenter:           "dc2",
				LocalBindAddress:     "127.0.0.2",
				Config:               map[string]any{"connect_timeout_ms": 5000},
			}}),
		)
	})
}

func TestConnect_connectProxyConfig(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil map", func(t *testing.T) {
		require.Equal(t, map[string]interface{}{
			"bind_address":     "0.0.0.0",
			"bind_port":        42,
			"envoy_stats_tags": []string{"nomad.alloc_id=test_alloc1"},
		}, connectProxyConfig(nil, 42, structs.AllocInfo{AllocID: "test_alloc1"}))
	})

	t.Run("pre-existing map", func(t *testing.T) {
		require.Equal(t, map[string]interface{}{
			"bind_address":     "0.0.0.0",
			"bind_port":        42,
			"foo":              "bar",
			"envoy_stats_tags": []string{"nomad.alloc_id=test_alloc2"},
		}, connectProxyConfig(map[string]interface{}{
			"foo": "bar",
		}, 42, structs.AllocInfo{AllocID: "test_alloc2"}))
	})
}

func TestConnect_getConnectPort(t *testing.T) {
	ci.Parallel(t)

	networks := structs.Networks{{
		IP: "192.168.30.1",
		DynamicPorts: []structs.Port{{
			Label: "connect-proxy-foo",
			Value: 23456,
			To:    23456,
		}}}}

	ports := structs.AllocatedPorts{{
		Label:  "foo",
		Value:  23456,
		To:     23456,
		HostIP: "192.168.30.1",
	}}

	t.Run("normal", func(t *testing.T) {
		nr, err := connectPort("foo", networks, ports)
		require.NoError(t, err)
		require.Equal(t, structs.AllocatedPortMapping{
			Label:  "foo",
			Value:  23456,
			To:     23456,
			HostIP: "192.168.30.1",
		}, nr)
	})

	t.Run("no such service", func(t *testing.T) {
		_, err := connectPort("other", networks, ports)
		require.EqualError(t, err, `No port of label "other" defined`)
	})

	t.Run("no network", func(t *testing.T) {
		_, err := connectPort("foo", nil, nil)
		require.EqualError(t, err, "Connect only supported with exactly 1 network (found 0)")
	})

	t.Run("multi network", func(t *testing.T) {
		_, err := connectPort("foo", append(networks, &structs.NetworkResource{
			Device: "eth1",
			IP:     "10.0.10.0",
		}), nil)
		require.EqualError(t, err, "Connect only supported with exactly 1 network (found 2)")
	})
}

func TestConnect_getExposePathPort(t *testing.T) {
	ci.Parallel(t)

	networks := structs.Networks{{
		Device: "eth0",
		IP:     "192.168.30.1",
		DynamicPorts: []structs.Port{{
			Label: "myPort",
			Value: 23456,
			To:    23456,
		}}}}

	t.Run("normal", func(t *testing.T) {
		ip, port, err := connectExposePathPort("myPort", networks)
		require.NoError(t, err)
		require.Equal(t, ip, "192.168.30.1")
		require.Equal(t, 23456, port)
	})

	t.Run("no such port label", func(t *testing.T) {
		_, _, err := connectExposePathPort("otherPort", networks)
		require.EqualError(t, err, `No port of label "otherPort" defined`)
	})

	t.Run("no network", func(t *testing.T) {
		_, _, err := connectExposePathPort("myPort", nil)
		require.EqualError(t, err, "Connect only supported with exactly 1 network (found 0)")
	})

	t.Run("multi network", func(t *testing.T) {
		_, _, err := connectExposePathPort("myPort", append(networks, &structs.NetworkResource{
			Device: "eth1",
			IP:     "10.0.10.0",
		}))
		require.EqualError(t, err, "Connect only supported with exactly 1 network (found 2)")
	})
}

func TestConnect_newConnectGateway(t *testing.T) {
	ci.Parallel(t)

	t.Run("not a gateway", func(t *testing.T) {
		result := newConnectGateway(&structs.ConsulConnect{Native: true})
		require.Nil(t, result)
	})

	t.Run("canonical empty", func(t *testing.T) {
		result := newConnectGateway(&structs.ConsulConnect{
			Gateway: &structs.ConsulGateway{
				Proxy: &structs.ConsulGatewayProxy{
					ConnectTimeout:                  pointer.Of(1 * time.Second),
					EnvoyGatewayBindTaggedAddresses: false,
					EnvoyGatewayBindAddresses:       nil,
					EnvoyGatewayNoDefaultBind:       false,
					Config:                          nil,
				},
			},
		})
		require.Equal(t, &api.AgentServiceConnectProxyConfig{
			Config: map[string]interface{}{
				"connect_timeout_ms": int64(1000),
			},
		}, result)
	})

	t.Run("proxy undefined", func(t *testing.T) {
		result := newConnectGateway(&structs.ConsulConnect{
			Gateway: &structs.ConsulGateway{
				Proxy: nil,
			},
		})
		require.Equal(t, &api.AgentServiceConnectProxyConfig{
			Config: nil,
		}, result)
	})

	t.Run("full", func(t *testing.T) {
		result := newConnectGateway(&structs.ConsulConnect{
			Gateway: &structs.ConsulGateway{
				Proxy: &structs.ConsulGatewayProxy{
					ConnectTimeout:                  pointer.Of(1 * time.Second),
					EnvoyGatewayBindTaggedAddresses: true,
					EnvoyGatewayBindAddresses: map[string]*structs.ConsulGatewayBindAddress{
						"service1": {
							Address: "10.0.0.1",
							Port:    2000,
						},
					},
					EnvoyGatewayNoDefaultBind: true,
					EnvoyDNSDiscoveryType:     "STRICT_DNS",
					Config: map[string]interface{}{
						"foo": 1,
					},
				},
			},
		})
		require.Equal(t, &api.AgentServiceConnectProxyConfig{
			Config: map[string]interface{}{
				"connect_timeout_ms":                  int64(1000),
				"envoy_gateway_bind_tagged_addresses": true,
				"envoy_gateway_bind_addresses": map[string]*structs.ConsulGatewayBindAddress{
					"service1": {
						Address: "10.0.0.1",
						Port:    2000,
					},
				},
				"envoy_gateway_no_default_bind": true,
				"envoy_dns_discovery_type":      "STRICT_DNS",
				"foo":                           1,
			},
		}, result)
	})
}

func Test_connectMeshGateway(t *testing.T) {
	ci.Parallel(t)

	t.Run("empty", func(t *testing.T) {
		result := connectMeshGateway(structs.ConsulMeshGateway{})
		require.Equal(t, api.MeshGatewayConfig{Mode: api.MeshGatewayModeDefault}, result)
	})

	t.Run("local", func(t *testing.T) {
		result := connectMeshGateway(structs.ConsulMeshGateway{Mode: "local"})
		require.Equal(t, api.MeshGatewayConfig{Mode: api.MeshGatewayModeLocal}, result)
	})

	t.Run("remote", func(t *testing.T) {
		result := connectMeshGateway(structs.ConsulMeshGateway{Mode: "remote"})
		require.Equal(t, api.MeshGatewayConfig{Mode: api.MeshGatewayModeRemote}, result)
	})

	t.Run("none", func(t *testing.T) {
		result := connectMeshGateway(structs.ConsulMeshGateway{Mode: "none"})
		require.Equal(t, api.MeshGatewayConfig{Mode: api.MeshGatewayModeNone}, result)
	})

	t.Run("nonsense", func(t *testing.T) {
		result := connectMeshGateway(structs.ConsulMeshGateway{})
		require.Equal(t, api.MeshGatewayConfig{Mode: api.MeshGatewayModeDefault}, result)
	})
}

func Test_injectNomadInfo(t *testing.T) {
	ci.Parallel(t)

	info1 := func() map[string]string {
		return map[string]string{
			"nomad.alloc_id=": "abc123",
		}
	}
	info2 := func() map[string]string {
		return map[string]string{
			"nomad.alloc_id=":  "abc123",
			"nomad.namespace=": "testns",
		}
	}

	try := func(defaultTags map[string]string, cfg, exp map[string]interface{}) {
		// TODO: defaultTags get modified over the execution
		injectNomadInfo(cfg, defaultTags)
		cfgTags, expTags := cfg["envoy_stats_tags"], exp["envoy_stats_tags"]
		delete(cfg, "envoy_stats_tags")
		delete(exp, "envoy_stats_tags")
		require.Equal(t, exp, cfg)
		require.ElementsMatch(t, expTags, cfgTags, "")
	}

	// empty
	try(
		info1(),
		make(map[string]interface{}),
		map[string]interface{}{
			"envoy_stats_tags": []string{"nomad.alloc_id=abc123"},
		},
	)

	// merge fresh
	try(
		info1(),
		map[string]interface{}{"foo": "bar"},
		map[string]interface{}{
			"foo":              "bar",
			"envoy_stats_tags": []string{"nomad.alloc_id=abc123"},
		},
	)

	// merge append
	try(
		info1(),
		map[string]interface{}{
			"foo":              "bar",
			"envoy_stats_tags": []string{"k1=v1", "k2=v2"},
		},
		map[string]interface{}{
			"foo":              "bar",
			"envoy_stats_tags": []string{"k1=v1", "k2=v2", "nomad.alloc_id=abc123"},
		},
	)

	// merge exists
	try(
		info2(),
		map[string]interface{}{
			"foo":              "bar",
			"envoy_stats_tags": []string{"k1=v1", "k2=v2", "nomad.alloc_id=xyz789"},
		},
		map[string]interface{}{
			"foo":              "bar",
			"envoy_stats_tags": []string{"k1=v1", "k2=v2", "nomad.alloc_id=xyz789", "nomad.namespace=testns"},
		},
	)

	// merge wrong type
	try(
		info1(),
		map[string]interface{}{
			"envoy_stats_tags": "not a slice of string",
		},
		map[string]interface{}{
			"envoy_stats_tags": []string{"nomad.alloc_id=abc123"},
		},
	)
}
