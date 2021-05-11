package consul

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
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
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		asr, err := newConnect("", "", nil, nil, nil)
		require.NoError(t, err)
		require.Nil(t, asr)
	})

	t.Run("native", func(t *testing.T) {
		asr, err := newConnect("", "", &structs.ConsulConnect{
			Native: true,
		}, nil, nil)
		require.NoError(t, err)
		require.True(t, asr.Native)
		require.Nil(t, asr.SidecarService)
	})

	t.Run("with sidecar", func(t *testing.T) {
		asr, err := newConnect("redis-service-id", "redis", &structs.ConsulConnect{
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
					"bind_address": "0.0.0.0",
					"bind_port":    3000,
				},
			},
			Checks: api.AgentServiceChecks{
				{
					Name:         "Connect Sidecar Aliasing redis-service-id",
					AliasService: "redis-service-id",
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
		asr, err := newConnect("redis-service-id", "redis", &structs.ConsulConnect{
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
					"bind_address": "0.0.0.0",
					"bind_port":    3000,
				},
			},
			Checks: api.AgentServiceChecks{
				{
					Name:         "Connect Sidecar Aliasing redis-service-id",
					AliasService: "redis-service-id",
				},
			},
		}, asr.SidecarService)
	})
}

func TestConnect_connectSidecarRegistration(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		sidecarReg, err := connectSidecarRegistration("", nil, testConnectNetwork, testConnectPorts)
		require.NoError(t, err)
		require.Nil(t, sidecarReg)
	})

	t.Run("no service port", func(t *testing.T) {
		_, err := connectSidecarRegistration("unknown-id", &structs.ConsulSidecarService{
			Port: "unknown-label",
		}, testConnectNetwork, testConnectPorts)
		require.EqualError(t, err, `No port of label "unknown-label" defined`)
	})

	t.Run("bad proxy", func(t *testing.T) {
		_, err := connectSidecarRegistration("redis-service-id", &structs.ConsulSidecarService{
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
		proxy, err := connectSidecarRegistration("redis-service-id", &structs.ConsulSidecarService{
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
					"bind_address": "0.0.0.0",
					"bind_port":    3000,
				},
			},
			Checks: api.AgentServiceChecks{
				{
					Name:         "Connect Sidecar Aliasing redis-service-id",
					AliasService: "redis-service-id",
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
	t.Parallel()

	// If the input proxy is nil, we expect the output to be a proxy with its
	// config set to default values.
	t.Run("nil proxy", func(t *testing.T) {
		proxy, err := connectSidecarProxy(nil, 2000, testConnectNetwork)
		require.NoError(t, err)
		require.Equal(t, &api.AgentServiceConnectProxyConfig{
			LocalServiceAddress: "",
			LocalServicePort:    0,
			Upstreams:           nil,
			Expose:              api.ExposeConfig{},
			Config: map[string]interface{}{
				"bind_address": "0.0.0.0",
				"bind_port":    2000,
			},
		}, proxy)
	})

	t.Run("bad proxy", func(t *testing.T) {
		_, err := connectSidecarProxy(&structs.ConsulProxy{
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
		proxy, err := connectSidecarProxy(&structs.ConsulProxy{
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
				"bind_address": "0.0.0.0",
				"bind_port":    2000,
			},
		}, proxy)
	})
}

func TestConnect_connectProxyExpose(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		require.Nil(t, connectUpstreams(nil))
	})

	t.Run("not empty", func(t *testing.T) {
		require.Equal(t,
			[]api.Upstream{{
				DestinationName: "foo",
				LocalBindPort:   8000,
			}, {
				DestinationName:  "bar",
				LocalBindPort:    9000,
				Datacenter:       "dc2",
				LocalBindAddress: "127.0.0.2",
			}},
			connectUpstreams([]structs.ConsulUpstream{{
				DestinationName: "foo",
				LocalBindPort:   8000,
			}, {
				DestinationName:  "bar",
				LocalBindPort:    9000,
				Datacenter:       "dc2",
				LocalBindAddress: "127.0.0.2",
			}}),
		)
	})
}

func TestConnect_connectProxyConfig(t *testing.T) {
	t.Parallel()

	t.Run("nil map", func(t *testing.T) {
		require.Equal(t, map[string]interface{}{
			"bind_address": "0.0.0.0",
			"bind_port":    42,
		}, connectProxyConfig(nil, 42))
	})

	t.Run("pre-existing map", func(t *testing.T) {
		require.Equal(t, map[string]interface{}{
			"bind_address": "0.0.0.0",
			"bind_port":    42,
			"foo":          "bar",
		}, connectProxyConfig(map[string]interface{}{
			"foo": "bar",
		}, 42))
	})
}

func TestConnect_getConnectPort(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	t.Run("not a gateway", func(t *testing.T) {
		result := newConnectGateway("s1", &structs.ConsulConnect{Native: true})
		require.Nil(t, result)
	})

	t.Run("canonical empty", func(t *testing.T) {
		result := newConnectGateway("s1", &structs.ConsulConnect{
			Gateway: &structs.ConsulGateway{
				Proxy: &structs.ConsulGatewayProxy{
					ConnectTimeout:                  helper.TimeToPtr(1 * time.Second),
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
		result := newConnectGateway("s1", &structs.ConsulConnect{
			Gateway: &structs.ConsulGateway{
				Proxy: nil,
			},
		})
		require.Equal(t, &api.AgentServiceConnectProxyConfig{
			Config: nil,
		}, result)
	})

	t.Run("full", func(t *testing.T) {
		result := newConnectGateway("s1", &structs.ConsulConnect{
			Gateway: &structs.ConsulGateway{
				Proxy: &structs.ConsulGatewayProxy{
					ConnectTimeout:                  helper.TimeToPtr(1 * time.Second),
					EnvoyGatewayBindTaggedAddresses: true,
					EnvoyGatewayBindAddresses: map[string]*structs.ConsulGatewayBindAddress{
						"service1": &structs.ConsulGatewayBindAddress{
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
					"service1": &structs.ConsulGatewayBindAddress{
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
