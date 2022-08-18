package api

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/api/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestConsul_Canonicalize(t *testing.T) {
	testutil.Parallel(t)
	t.Run("missing ns", func(t *testing.T) {
		c := new(Consul)
		c.Canonicalize()
		require.Empty(t, c.Namespace)
	})

	t.Run("complete", func(t *testing.T) {
		c := &Consul{Namespace: "foo"}
		c.Canonicalize()
		require.Equal(t, "foo", c.Namespace)
	})
}

func TestConsul_Copy(t *testing.T) {
	testutil.Parallel(t)
	t.Run("complete", func(t *testing.T) {
		result := (&Consul{
			Namespace: "foo",
		}).Copy()
		require.Equal(t, &Consul{
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
		require.Equal(t, "foo", a.Namespace)
		require.Equal(t, "bar", *ns)
	})

	t.Run("inherit", func(t *testing.T) {
		a := &Consul{Namespace: ""}
		ns := pointerOf("bar")
		a.MergeNamespace(ns)
		require.Equal(t, "bar", a.Namespace)
		require.Equal(t, "bar", *ns)
	})

	t.Run("parent is nil", func(t *testing.T) {
		a := &Consul{Namespace: "foo"}
		ns := (*string)(nil)
		a.MergeNamespace(ns)
		require.Equal(t, "foo", a.Namespace)
		require.Nil(t, ns)
	})
}

func TestConsulConnect_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil connect", func(t *testing.T) {
		cc := (*ConsulConnect)(nil)
		cc.Canonicalize()
		require.Nil(t, cc)
	})

	t.Run("empty connect", func(t *testing.T) {
		cc := new(ConsulConnect)
		cc.Canonicalize()
		require.Empty(t, cc.Native)
		require.Nil(t, cc.SidecarService)
		require.Nil(t, cc.SidecarTask)
	})
}

func TestConsulSidecarService_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil sidecar_service", func(t *testing.T) {
		css := (*ConsulSidecarService)(nil)
		css.Canonicalize()
		require.Nil(t, css)
	})

	t.Run("empty sidecar_service", func(t *testing.T) {
		css := new(ConsulSidecarService)
		css.Canonicalize()
		require.Empty(t, css.Tags)
		require.Nil(t, css.Proxy)
	})

	t.Run("non-empty sidecar_service", func(t *testing.T) {
		css := &ConsulSidecarService{
			Tags: make([]string, 0),
			Port: "port",
			Proxy: &ConsulProxy{
				LocalServiceAddress: "lsa",
				LocalServicePort:    80,
			},
		}
		css.Canonicalize()
		require.Equal(t, &ConsulSidecarService{
			Tags: nil,
			Port: "port",
			Proxy: &ConsulProxy{
				LocalServiceAddress: "lsa",
				LocalServicePort:    80},
		}, css)
	})
}

func TestConsulProxy_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil proxy", func(t *testing.T) {
		cp := (*ConsulProxy)(nil)
		cp.Canonicalize()
		require.Nil(t, cp)
	})

	t.Run("empty proxy", func(t *testing.T) {
		cp := new(ConsulProxy)
		cp.Canonicalize()
		require.Empty(t, cp.LocalServiceAddress)
		require.Zero(t, cp.LocalServicePort)
		require.Nil(t, cp.ExposeConfig)
		require.Nil(t, cp.Upstreams)
		require.Empty(t, cp.Config)
	})

	t.Run("non empty proxy", func(t *testing.T) {
		cp := &ConsulProxy{
			LocalServiceAddress: "127.0.0.1",
			LocalServicePort:    80,
			ExposeConfig:        new(ConsulExposeConfig),
			Upstreams:           make([]*ConsulUpstream, 0),
			Config:              make(map[string]interface{}),
		}
		cp.Canonicalize()
		require.Equal(t, "127.0.0.1", cp.LocalServiceAddress)
		require.Equal(t, 80, cp.LocalServicePort)
		require.Equal(t, &ConsulExposeConfig{}, cp.ExposeConfig)
		require.Nil(t, cp.Upstreams)
		require.Nil(t, cp.Config)
	})
}

func TestConsulUpstream_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil upstream", func(t *testing.T) {
		cu := (*ConsulUpstream)(nil)
		result := cu.Copy()
		require.Nil(t, result)
	})

	t.Run("complete upstream", func(t *testing.T) {
		cu := &ConsulUpstream{
			DestinationName:      "dest1",
			DestinationNamespace: "ns2",
			Datacenter:           "dc2",
			LocalBindPort:        2000,
			LocalBindAddress:     "10.0.0.1",
			MeshGateway:          &ConsulMeshGateway{Mode: "remote"},
		}
		result := cu.Copy()
		require.Equal(t, cu, result)
	})
}

func TestConsulUpstream_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil upstream", func(t *testing.T) {
		cu := (*ConsulUpstream)(nil)
		cu.Canonicalize()
		require.Nil(t, cu)
	})

	t.Run("complete", func(t *testing.T) {
		cu := &ConsulUpstream{
			DestinationName:      "dest1",
			DestinationNamespace: "ns2",
			Datacenter:           "dc2",
			LocalBindPort:        2000,
			LocalBindAddress:     "10.0.0.1",
			MeshGateway:          &ConsulMeshGateway{Mode: ""},
		}
		cu.Canonicalize()
		require.Equal(t, &ConsulUpstream{
			DestinationName:      "dest1",
			DestinationNamespace: "ns2",
			Datacenter:           "dc2",
			LocalBindPort:        2000,
			LocalBindAddress:     "10.0.0.1",
			MeshGateway:          &ConsulMeshGateway{Mode: ""},
		}, cu)
	})
}

func TestSidecarTask_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil sidecar_task", func(t *testing.T) {
		st := (*SidecarTask)(nil)
		st.Canonicalize()
		require.Nil(t, st)
	})

	t.Run("empty sidecar_task", func(t *testing.T) {
		st := new(SidecarTask)
		st.Canonicalize()
		require.Nil(t, st.Config)
		require.Nil(t, st.Env)
		require.Equal(t, DefaultResources(), st.Resources)
		require.Equal(t, DefaultLogConfig(), st.LogConfig)
		require.Nil(t, st.Meta)
		require.Equal(t, 5*time.Second, *st.KillTimeout)
		require.Equal(t, 0*time.Second, *st.ShutdownDelay)
	})

	t.Run("non empty sidecar_task resources", func(t *testing.T) {
		exp := DefaultResources()
		exp.MemoryMB = pointerOf(333)
		st := &SidecarTask{
			Resources: &Resources{MemoryMB: pointerOf(333)},
		}
		st.Canonicalize()
		require.Equal(t, exp, st.Resources)
	})
}

func TestConsulGateway_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		cg := (*ConsulGateway)(nil)
		cg.Canonicalize()
		require.Nil(t, cg)
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
		require.Equal(t, pointerOf(5*time.Second), cg.Proxy.ConnectTimeout)
		require.True(t, cg.Proxy.EnvoyGatewayBindTaggedAddresses)
		require.Nil(t, cg.Proxy.EnvoyGatewayBindAddresses)
		require.True(t, cg.Proxy.EnvoyGatewayNoDefaultBind)
		require.Empty(t, cg.Proxy.EnvoyDNSDiscoveryType)
		require.Nil(t, cg.Proxy.Config)
		require.Nil(t, cg.Ingress.Listeners)
	})
}

func TestConsulGateway_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		result := (*ConsulGateway)(nil).Copy()
		require.Nil(t, result)
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
		require.Equal(t, gateway, result)
	})
}

func TestConsulIngressConfigEntry_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		c := (*ConsulIngressConfigEntry)(nil)
		c.Canonicalize()
		require.Nil(t, c)
	})

	t.Run("empty fields", func(t *testing.T) {
		c := &ConsulIngressConfigEntry{
			TLS:       nil,
			Listeners: []*ConsulIngressListener{},
		}
		c.Canonicalize()
		require.Nil(t, c.TLS)
		require.Nil(t, c.Listeners)
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
		require.Equal(t, &ConsulIngressConfigEntry{
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
		require.Nil(t, result)
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
			}, {
				Name:  "service2",
				Hosts: []string{"2.2.2.2"},
			}},
		}},
	}

	t.Run("complete", func(t *testing.T) {
		result := entry.Copy()
		require.Equal(t, entry, result)
	})
}

func TestConsulTerminatingConfigEntry_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		c := (*ConsulTerminatingConfigEntry)(nil)
		c.Canonicalize()
		require.Nil(t, c)
	})

	t.Run("empty services", func(t *testing.T) {
		c := &ConsulTerminatingConfigEntry{
			Services: []*ConsulLinkedService{},
		}
		c.Canonicalize()
		require.Nil(t, c.Services)
	})
}

func TestConsulTerminatingConfigEntry_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		result := (*ConsulIngressConfigEntry)(nil).Copy()
		require.Nil(t, result)
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
		require.Equal(t, entry, result)
	})
}

func TestConsulMeshConfigEntry_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		ce := (*ConsulMeshConfigEntry)(nil)
		ce.Canonicalize()
		require.Nil(t, ce)
	})

	t.Run("instantiated", func(t *testing.T) {
		ce := new(ConsulMeshConfigEntry)
		ce.Canonicalize()
		require.NotNil(t, ce)
	})
}

func TestConsulMeshConfigEntry_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		ce := (*ConsulMeshConfigEntry)(nil)
		ce2 := ce.Copy()
		require.Nil(t, ce2)
	})

	t.Run("instantiated", func(t *testing.T) {
		ce := new(ConsulMeshConfigEntry)
		ce2 := ce.Copy()
		require.NotNil(t, ce2)
	})
}

func TestConsulMeshGateway_Canonicalize(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		c := (*ConsulMeshGateway)(nil)
		c.Canonicalize()
		require.Nil(t, c)
	})

	t.Run("unset mode", func(t *testing.T) {
		c := &ConsulMeshGateway{Mode: ""}
		c.Canonicalize()
		require.Equal(t, "", c.Mode)
	})

	t.Run("set mode", func(t *testing.T) {
		c := &ConsulMeshGateway{Mode: "remote"}
		c.Canonicalize()
		require.Equal(t, "remote", c.Mode)
	})
}

func TestConsulMeshGateway_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		c := (*ConsulMeshGateway)(nil)
		result := c.Copy()
		require.Nil(t, result)
	})

	t.Run("instantiated", func(t *testing.T) {
		c := &ConsulMeshGateway{
			Mode: "local",
		}
		result := c.Copy()
		require.Equal(t, c, result)
	})
}

func TestConsulGatewayTLSConfig_Copy(t *testing.T) {
	testutil.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		c := (*ConsulGatewayTLSConfig)(nil)
		result := c.Copy()
		require.Nil(t, result)
	})

	t.Run("enabled", func(t *testing.T) {
		c := &ConsulGatewayTLSConfig{
			Enabled: true,
		}
		result := c.Copy()
		require.Equal(t, c, result)
	})

	t.Run("customized", func(t *testing.T) {
		c := &ConsulGatewayTLSConfig{
			Enabled:       true,
			TLSMinVersion: "TLSv1_2",
			TLSMaxVersion: "TLSv1_3",
			CipherSuites:  []string{"foo", "bar"},
		}
		result := c.Copy()
		require.Equal(t, c, result)
	})
}
