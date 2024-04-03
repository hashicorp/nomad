// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/shoenig/test/must"
)

func TestConsul_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("missing ns", func(t *testing.T) {
		c := new(Consul)
		c.Canonicalize()
		must.Eq(t, "", c.Namespace)
	})

	t.Run("complete", func(t *testing.T) {
		c := &Consul{Namespace: "foo"}
		c.Canonicalize()
		must.Eq(t, "foo", c.Namespace)
	})
}

func TestConsul_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("complete", func(t *testing.T) {
		result := (&Consul{
			Namespace: "foo",
		}).Copy()
		must.Eq(t, &Consul{
			Namespace: "foo",
		}, result)
	})
}

func TestConsul_MergeNamespace(t *testing.T) {
	testutil.Parallel(t)

	t.Run("already set", func(t *testing.T) {
		a := &Consul{Namespace: "foo"}
		ns := pointerOf("bar")
		a.MergeNamespace(ns)
		must.Eq(t, "foo", a.Namespace)
		must.Eq(t, "bar", *ns)
	})

	t.Run("inherit", func(t *testing.T) {
		a := &Consul{Namespace: ""}
		ns := pointerOf("bar")
		a.MergeNamespace(ns)
		must.Eq(t, "bar", a.Namespace)
		must.Eq(t, "bar", *ns)
	})

	t.Run("parent is nil", func(t *testing.T) {
		a := &Consul{Namespace: "foo"}
		ns := (*string)(nil)
		a.MergeNamespace(ns)
		must.Eq(t, "foo", a.Namespace)
		must.Nil(t, ns)
	})
}

func TestConsulConnect_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil connect", func(t *testing.T) {
		cc := (*ConsulConnect)(nil)
		cc.Canonicalize()
		must.Nil(t, cc)
	})

	t.Run("empty connect", func(t *testing.T) {
		cc := new(ConsulConnect)
		cc.Canonicalize()
		must.False(t, cc.Native)
		must.Nil(t, cc.SidecarService)
		must.Nil(t, cc.SidecarTask)
	})
}

func TestConsulSidecarService_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil sidecar_service", func(t *testing.T) {
		css := (*ConsulSidecarService)(nil)
		css.Canonicalize()
		must.Nil(t, css)
	})

	t.Run("empty sidecar_service", func(t *testing.T) {
		css := new(ConsulSidecarService)
		css.Canonicalize()
		must.SliceEmpty(t, css.Tags)
		must.Nil(t, css.Proxy)
	})

	t.Run("non-empty sidecar_service", func(t *testing.T) {
		css := &ConsulSidecarService{
			Tags: make([]string, 0),
			Port: "port",
			Proxy: &ConsulProxy{
				LocalServiceAddress: "lsa",
				LocalServicePort:    80,
			},
			Meta: map[string]string{
				"test-key": "test-value",
			},
		}
		css.Canonicalize()
		must.Eq(t, &ConsulSidecarService{
			Tags: nil,
			Port: "port",
			Proxy: &ConsulProxy{
				LocalServiceAddress: "lsa",
				LocalServicePort:    80},
			Meta: map[string]string{
				"test-key": "test-value",
			},
		}, css)
	})
}

func TestConsulProxy_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil proxy", func(t *testing.T) {
		cp := (*ConsulProxy)(nil)
		cp.Canonicalize()
		must.Nil(t, cp)
	})

	t.Run("empty proxy", func(t *testing.T) {
		cp := new(ConsulProxy)
		cp.Canonicalize()
		must.Eq(t, "", cp.LocalServiceAddress)
		must.Zero(t, cp.LocalServicePort)
		must.Nil(t, cp.Expose)
		must.Nil(t, cp.Upstreams)
		must.MapEmpty(t, cp.Config)
	})

	t.Run("non empty proxy", func(t *testing.T) {
		cp := &ConsulProxy{
			LocalServiceAddress: "127.0.0.1",
			LocalServicePort:    80,
			Expose:              new(ConsulExposeConfig),
			Upstreams:           make([]*ConsulUpstream, 0),
			Config:              make(map[string]interface{}),
		}
		cp.Canonicalize()
		must.Eq(t, "127.0.0.1", cp.LocalServiceAddress)
		must.Eq(t, 80, cp.LocalServicePort)
		must.Eq(t, &ConsulExposeConfig{}, cp.Expose)
		must.Nil(t, cp.Upstreams)
		must.Nil(t, cp.Config)
	})
}

func TestConsulUpstream_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil upstream", func(t *testing.T) {
		cu := (*ConsulUpstream)(nil)
		result := cu.Copy()
		must.Nil(t, result)
	})

	t.Run("complete upstream", func(t *testing.T) {
		cu := &ConsulUpstream{
			DestinationName:      "dest1",
			DestinationNamespace: "ns2",
			DestinationPeer:      "10.0.0.1:6379",
			DestinationPartition: "infra",
			DestinationType:      "tcp",
			Datacenter:           "dc2",
			LocalBindPort:        2000,
			LocalBindAddress:     "10.0.0.1",
			LocalBindSocketPath:  "/var/run/testsocket.sock",
			LocalBindSocketMode:  "0666",
			MeshGateway:          &ConsulMeshGateway{Mode: "remote"},
			Config:               map[string]any{"connect_timeout_ms": 5000},
		}
		result := cu.Copy()
		must.Eq(t, cu, result)
	})
}

func TestConsulUpstream_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil upstream", func(t *testing.T) {
		cu := (*ConsulUpstream)(nil)
		cu.Canonicalize()
		must.Nil(t, cu)
	})

	t.Run("complete", func(t *testing.T) {
		cu := &ConsulUpstream{
			DestinationName:      "dest1",
			DestinationNamespace: "ns2",
			DestinationPeer:      "10.0.0.1:6379",
			DestinationType:      "tcp",
			Datacenter:           "dc2",
			LocalBindPort:        2000,
			LocalBindAddress:     "10.0.0.1",
			LocalBindSocketPath:  "/var/run/testsocket.sock",
			LocalBindSocketMode:  "0666",
			MeshGateway:          &ConsulMeshGateway{Mode: ""},
			Config:               make(map[string]any),
		}
		cu.Canonicalize()
		must.Eq(t, &ConsulUpstream{
			DestinationName:      "dest1",
			DestinationNamespace: "ns2",
			DestinationPeer:      "10.0.0.1:6379",
			DestinationType:      "tcp",
			Datacenter:           "dc2",
			LocalBindPort:        2000,
			LocalBindAddress:     "10.0.0.1",
			LocalBindSocketPath:  "/var/run/testsocket.sock",
			LocalBindSocketMode:  "0666",
			MeshGateway:          &ConsulMeshGateway{Mode: ""},
			Config:               nil,
		}, cu)
	})
}

func TestSidecarTask_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil sidecar_task", func(t *testing.T) {
		st := (*SidecarTask)(nil)
		st.Canonicalize()
		must.Nil(t, st)
	})

	t.Run("empty sidecar_task", func(t *testing.T) {
		st := new(SidecarTask)
		st.Canonicalize()
		must.Nil(t, st.Config)
		must.Nil(t, st.Env)
		must.Eq(t, DefaultResources(), st.Resources)
		must.Eq(t, DefaultLogConfig(), st.LogConfig)
		must.Nil(t, st.Meta)
		must.Eq(t, 5*time.Second, *st.KillTimeout)
		must.Eq(t, 0*time.Second, *st.ShutdownDelay)
	})

	t.Run("non empty sidecar_task resources", func(t *testing.T) {
		exp := DefaultResources()
		exp.MemoryMB = pointerOf(333)
		st := &SidecarTask{
			Resources: &Resources{MemoryMB: pointerOf(333)},
		}
		st.Canonicalize()
		must.Eq(t, exp, st.Resources)
	})
}

func TestConsulGateway_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		cg := (*ConsulGateway)(nil)
		cg.Canonicalize()
		must.Nil(t, cg)
	})

	t.Run("set defaults", func(t *testing.T) {
		cg := &ConsulGateway{
			Proxy: &ConsulGatewayProxy{
				ConnectTimeout:                  nil,
				EnvoyGatewayBindTaggedAddresses: true,
				EnvoyGatewayBindAddresses:       make(map[string]*ConsulGatewayBindAddress, 0),
				EnvoyGatewayNoDefaultBind:       true,
				Config:                          make(map[string]interface{}, 0),
			},
			Ingress: &ConsulIngressConfigEntry{
				TLS: &ConsulGatewayTLSConfig{
					Enabled: false,
				},
				Listeners: make([]*ConsulIngressListener, 0),
			},
		}
		cg.Canonicalize()
		must.Eq(t, pointerOf(5*time.Second), cg.Proxy.ConnectTimeout)
		must.True(t, cg.Proxy.EnvoyGatewayBindTaggedAddresses)
		must.Nil(t, cg.Proxy.EnvoyGatewayBindAddresses)
		must.True(t, cg.Proxy.EnvoyGatewayNoDefaultBind)
		must.Eq(t, "", cg.Proxy.EnvoyDNSDiscoveryType)
		must.Nil(t, cg.Proxy.Config)
		must.Nil(t, cg.Ingress.Listeners)
	})
}

func TestConsulGateway_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		result := (*ConsulGateway)(nil).Copy()
		must.Nil(t, result)
	})

	gateway := &ConsulGateway{
		Proxy: &ConsulGatewayProxy{
			ConnectTimeout:                  pointerOf(3 * time.Second),
			EnvoyGatewayBindTaggedAddresses: true,
			EnvoyGatewayBindAddresses: map[string]*ConsulGatewayBindAddress{
				"listener1": {Address: "10.0.0.1", Port: 2000},
				"listener2": {Address: "10.0.0.1", Port: 2001},
			},
			EnvoyGatewayNoDefaultBind: true,
			EnvoyDNSDiscoveryType:     "STRICT_DNS",
			Config: map[string]interface{}{
				"foo": "bar",
				"baz": 3,
			},
		},
		Ingress: &ConsulIngressConfigEntry{
			TLS: &ConsulGatewayTLSConfig{
				Enabled: true,
			},
			Listeners: []*ConsulIngressListener{{
				Port:     3333,
				Protocol: "tcp",
				Services: []*ConsulIngressService{{
					Name: "service1",
					Hosts: []string{
						"127.0.0.1", "127.0.0.1:3333",
					}},
				}},
			},
		},
		Terminating: &ConsulTerminatingConfigEntry{
			Services: []*ConsulLinkedService{{
				Name: "linked-service1",
			}},
		},
	}

	t.Run("complete", func(t *testing.T) {
		result := gateway.Copy()
		must.Eq(t, gateway, result)
	})
}

func TestConsulIngressConfigEntry_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		c := (*ConsulIngressConfigEntry)(nil)
		c.Canonicalize()
		must.Nil(t, c)
	})

	t.Run("empty fields", func(t *testing.T) {
		c := &ConsulIngressConfigEntry{
			TLS:       nil,
			Listeners: []*ConsulIngressListener{},
		}
		c.Canonicalize()
		must.Nil(t, c.TLS)
		must.Nil(t, c.Listeners)
	})

	t.Run("complete", func(t *testing.T) {
		c := &ConsulIngressConfigEntry{
			TLS: &ConsulGatewayTLSConfig{Enabled: true},
			Listeners: []*ConsulIngressListener{{
				Port:     9090,
				Protocol: "http",
				Services: []*ConsulIngressService{{
					Name:  "service1",
					Hosts: []string{"1.1.1.1"},
				}},
			}},
		}
		c.Canonicalize()
		must.Eq(t, &ConsulIngressConfigEntry{
			TLS: &ConsulGatewayTLSConfig{Enabled: true},
			Listeners: []*ConsulIngressListener{{
				Port:     9090,
				Protocol: "http",
				Services: []*ConsulIngressService{{
					Name:  "service1",
					Hosts: []string{"1.1.1.1"},
				}},
			}},
		}, c)
	})
}

func TestConsulIngressConfigEntry_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		result := (*ConsulIngressConfigEntry)(nil).Copy()
		must.Nil(t, result)
	})

	entry := &ConsulIngressConfigEntry{
		TLS: &ConsulGatewayTLSConfig{
			Enabled: true,
		},
		Listeners: []*ConsulIngressListener{{
			Port:     1111,
			Protocol: "http",
			Services: []*ConsulIngressService{{
				Name:  "service1",
				Hosts: []string{"1.1.1.1", "1.1.1.1:9000"},
				TLS: &ConsulGatewayTLSConfig{
					SDS: &ConsulGatewayTLSSDSConfig{
						ClusterName:  "foo",
						CertResource: "bar",
					},
				},
				RequestHeaders: &ConsulHTTPHeaderModifiers{
					Add: map[string]string{
						"test": "testvalue",
					},
					Set: map[string]string{
						"test1": "testvalue1",
					},
					Remove: []string{"test2"},
				},
				ResponseHeaders: &ConsulHTTPHeaderModifiers{
					Add: map[string]string{
						"test": "testvalue",
					},
					Set: map[string]string{
						"test1": "testvalue1",
					},
					Remove: []string{"test2"},
				},
				MaxConnections:        pointerOf(uint32(5120)),
				MaxPendingRequests:    pointerOf(uint32(512)),
				MaxConcurrentRequests: pointerOf(uint32(2048)),
			}, {
				Name:  "service2",
				Hosts: []string{"2.2.2.2"},
			}},
		}},
	}

	t.Run("complete", func(t *testing.T) {
		result := entry.Copy()
		must.Eq(t, entry, result)
	})
}

func TestConsulTerminatingConfigEntry_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		c := (*ConsulTerminatingConfigEntry)(nil)
		c.Canonicalize()
		must.Nil(t, c)
	})

	t.Run("empty services", func(t *testing.T) {
		c := &ConsulTerminatingConfigEntry{
			Services: []*ConsulLinkedService{},
		}
		c.Canonicalize()
		must.Nil(t, c.Services)
	})
}

func TestConsulTerminatingConfigEntry_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		result := (*ConsulIngressConfigEntry)(nil).Copy()
		must.Nil(t, result)
	})

	entry := &ConsulTerminatingConfigEntry{
		Services: []*ConsulLinkedService{{
			Name: "servic1",
		}, {
			Name:     "service2",
			CAFile:   "ca_file.pem",
			CertFile: "cert_file.pem",
			KeyFile:  "key_file.pem",
			SNI:      "sni.terminating.consul",
		}},
	}

	t.Run("complete", func(t *testing.T) {
		result := entry.Copy()
		must.Eq(t, entry, result)
	})
}

func TestConsulMeshConfigEntry_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		ce := (*ConsulMeshConfigEntry)(nil)
		ce.Canonicalize()
		must.Nil(t, ce)
	})

	t.Run("instantiated", func(t *testing.T) {
		ce := new(ConsulMeshConfigEntry)
		ce.Canonicalize()
		must.NotNil(t, ce)
	})
}

func TestConsulMeshConfigEntry_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		ce := (*ConsulMeshConfigEntry)(nil)
		ce2 := ce.Copy()
		must.Nil(t, ce2)
	})

	t.Run("instantiated", func(t *testing.T) {
		ce := new(ConsulMeshConfigEntry)
		ce2 := ce.Copy()
		must.NotNil(t, ce2)
	})
}

func TestConsulMeshGateway_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		c := (*ConsulMeshGateway)(nil)
		c.Canonicalize()
		must.Nil(t, c)
	})

	t.Run("unset mode", func(t *testing.T) {
		c := &ConsulMeshGateway{Mode: ""}
		c.Canonicalize()
		must.Eq(t, "", c.Mode)
	})

	t.Run("set mode", func(t *testing.T) {
		c := &ConsulMeshGateway{Mode: "remote"}
		c.Canonicalize()
		must.Eq(t, "remote", c.Mode)
	})
}

func TestConsulMeshGateway_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		c := (*ConsulMeshGateway)(nil)
		result := c.Copy()
		must.Nil(t, result)
	})

	t.Run("instantiated", func(t *testing.T) {
		c := &ConsulMeshGateway{
			Mode: "local",
		}
		result := c.Copy()
		must.Eq(t, c, result)
	})
}

func TestConsulGatewayTLSConfig_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		c := (*ConsulGatewayTLSConfig)(nil)
		result := c.Copy()
		must.Nil(t, result)
	})

	t.Run("enabled", func(t *testing.T) {
		c := &ConsulGatewayTLSConfig{
			Enabled: true,
		}
		result := c.Copy()
		must.Eq(t, c, result)
	})

	t.Run("customized", func(t *testing.T) {
		c := &ConsulGatewayTLSConfig{
			Enabled:       true,
			TLSMinVersion: "TLSv1_2",
			TLSMaxVersion: "TLSv1_3",
			CipherSuites:  []string{"foo", "bar"},
		}
		result := c.Copy()
		must.Eq(t, c, result)
	})
}
