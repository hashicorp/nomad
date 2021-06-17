package consul

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestSyncLogic_agentServiceUpdateRequired(t *testing.T) {
	t.Parallel()

	// the service as known by nomad
	wanted := func() api.AgentServiceRegistration {
		return api.AgentServiceRegistration{
			Kind:              "",
			ID:                "aca4c175-1778-5ef4-0220-2ab434147d35",
			Name:              "myservice",
			Tags:              []string{"a", "b"},
			Port:              9000,
			Address:           "1.1.1.1",
			EnableTagOverride: true,
			Meta:              map[string]string{"foo": "1"},
			Connect: &api.AgentServiceConnect{
				Native: false,
				SidecarService: &api.AgentServiceRegistration{
					Kind: "connect-proxy",
					ID:   "_nomad-task-8e8413af-b5bb-aa67-2c24-c146c45f1ec9-group-mygroup-myservice-9001-sidecar-proxy",
					Name: "name-sidecar-proxy",
					Tags: []string{"x", "y", "z"},
					Proxy: &api.AgentServiceConnectProxyConfig{
						Upstreams: []api.Upstream{{
							Datacenter:      "dc1",
							DestinationName: "dest1",
						}},
					},
				},
			},
		}
	}

	// the service (and + connect proxy) as known by consul
	existing := &api.AgentService{
		Kind:              "",
		ID:                "aca4c175-1778-5ef4-0220-2ab434147d35",
		Service:           "myservice",
		Tags:              []string{"a", "b"},
		Port:              9000,
		Address:           "1.1.1.1",
		EnableTagOverride: true,
		Meta:              map[string]string{"foo": "1"},
	}

	sidecar := &api.AgentService{
		Kind:    "connect-proxy",
		ID:      "_nomad-task-8e8413af-b5bb-aa67-2c24-c146c45f1ec9-group-mygroup-myservice-9001-sidecar-proxy",
		Service: "myservice-sidecar-proxy",
		Tags:    []string{"x", "y", "z"},
		Proxy: &api.AgentServiceConnectProxyConfig{
			Upstreams: []api.Upstream{{
				Datacenter:      "dc1",
				DestinationName: "dest1",
			}},
		},
	}

	// By default wanted and existing match. Each test should modify wanted in
	// 1 way, and / or configure the type of sync operation that is being
	// considered, then evaluate the result of the update-required algebra.

	type asr = api.AgentServiceRegistration
	type tweaker func(w asr) *asr // create a conveniently modifiable copy

	try := func(
		t *testing.T,
		exp bool,
		reason syncReason,
		tweak tweaker) {
		result := agentServiceUpdateRequired(reason, tweak(wanted()), existing, sidecar)
		require.Equal(t, exp, result)
	}

	t.Run("matching", func(t *testing.T) {
		try(t, false, syncNewOps, func(w asr) *asr {
			return &w
		})
	})

	t.Run("different kind", func(t *testing.T) {
		try(t, true, syncNewOps, func(w asr) *asr {
			w.Kind = "other"
			return &w
		})
	})

	t.Run("different id", func(t *testing.T) {
		try(t, true, syncNewOps, func(w asr) *asr {
			w.ID = "_other"
			return &w
		})
	})

	t.Run("different port", func(t *testing.T) {
		try(t, true, syncNewOps, func(w asr) *asr {
			w.Port = 9001
			return &w
		})
	})

	t.Run("different address", func(t *testing.T) {
		try(t, true, syncNewOps, func(w asr) *asr {
			w.Address = "2.2.2.2"
			return &w
		})
	})

	t.Run("different name", func(t *testing.T) {
		try(t, true, syncNewOps, func(w asr) *asr {
			w.Name = "bob"
			return &w
		})
	})

	t.Run("different enable_tag_override", func(t *testing.T) {
		try(t, true, syncNewOps, func(w asr) *asr {
			w.EnableTagOverride = false
			return &w
		})
	})

	t.Run("different meta", func(t *testing.T) {
		try(t, true, syncNewOps, func(w asr) *asr {
			w.Meta = map[string]string{"foo": "2"}
			return &w
		})
	})

	t.Run("different sidecar upstream", func(t *testing.T) {
		try(t, true, syncNewOps, func(w asr) *asr {
			w.Connect.SidecarService.Proxy.Upstreams[0].DestinationName = "dest2"
			return &w
		})
	})

	t.Run("remove sidecar upstream", func(t *testing.T) {
		try(t, true, syncNewOps, func(w asr) *asr {
			w.Connect.SidecarService.Proxy.Upstreams = nil
			return &w
		})
	})

	t.Run("additional sidecar upstream", func(t *testing.T) {
		try(t, true, syncNewOps, func(w asr) *asr {
			w.Connect.SidecarService.Proxy.Upstreams = append(
				w.Connect.SidecarService.Proxy.Upstreams,
				api.Upstream{
					Datacenter:      "dc2",
					DestinationName: "dest2",
				},
			)
			return &w
		})
	})

	t.Run("nil proxy block", func(t *testing.T) {
		try(t, true, syncNewOps, func(w asr) *asr {
			w.Connect.SidecarService.Proxy = nil
			return &w
		})
	})

	t.Run("different tags syncNewOps eto=true", func(t *testing.T) {
		// sync is required even though eto=true, because NewOps indicates the
		// service definition  in nomad has changed (e.g. job run a modified job)
		try(t, true, syncNewOps, func(w asr) *asr {
			w.Tags = []string{"other", "tags"}
			return &w
		})
	})

	t.Run("different tags syncPeriodic eto=true", func(t *testing.T) {
		// sync is not required since eto=true and this is a periodic sync
		// with consul - in which case we keep Consul's definition of the tags
		try(t, false, syncPeriodic, func(w asr) *asr {
			w.Tags = []string{"other", "tags"}
			return &w
		})
	})

	t.Run("different sidecar tags on syncPeriodic eto=true", func(t *testing.T) {
		try(t, false, syncPeriodic, func(w asr) *asr {
			// like the parent service, the sidecar's tags do not get enforced
			// if ETO is true and this is a periodic sync
			w.Connect.SidecarService.Tags = []string{"other", "tags"}
			return &w
		})
	})

	t.Run("different sidecar tags on syncNewOps eto=true", func(t *testing.T) {
		try(t, true, syncNewOps, func(w asr) *asr {
			// like the parent service, the sidecar's tags always get enforced
			// regardless of ETO if this is a sync due to applied operations
			w.Connect.SidecarService.Tags = []string{"other", "tags"}
			return &w
		})
	})

	// for remaining tests, EnableTagOverride = false
	existing.EnableTagOverride = false

	t.Run("different tags syncPeriodic eto=false", func(t *testing.T) {
		// sync is required because eto=false and the tags do not match
		try(t, true, syncPeriodic, func(w asr) *asr {
			w.EnableTagOverride = false
			w.Tags = []string{"other", "tags"}
			return &w
		})
	})

	t.Run("different tags syncNewOps eto=false", func(t *testing.T) {
		// sync is required because eto=false and the tags do not match
		try(t, true, syncNewOps, func(w asr) *asr {
			w.EnableTagOverride = false
			w.Tags = []string{"other", "tags"}
			return &w
		})
	})

	t.Run("different sidecar tags on syncPeriodic eto=false", func(t *testing.T) {
		// like the parent service, sync is required because eto=false and the
		// sidecar's tags do not match
		try(t, true, syncPeriodic, func(w asr) *asr {
			w.EnableTagOverride = false
			w.Connect.SidecarService.Tags = []string{"other", "tags"}
			return &w
		})
	})

	t.Run("different sidecar tags syncNewOps eto=false", func(t *testing.T) {
		// like the parent service, sync is required because eto=false and the
		// sidecar's tags do not match
		try(t, true, syncNewOps, func(w asr) *asr {
			w.EnableTagOverride = false
			w.Connect.SidecarService.Tags = []string{"other", "tags"}
			return &w
		})
	})
}

func TestSyncLogic_tagsDifferent(t *testing.T) {
	t.Run("nil nil", func(t *testing.T) {
		require.False(t, tagsDifferent(nil, nil))
	})

	t.Run("empty nil", func(t *testing.T) {
		// where reflect.DeepEqual does not work
		require.False(t, tagsDifferent([]string{}, nil))
	})

	t.Run("empty empty", func(t *testing.T) {
		require.False(t, tagsDifferent([]string{}, []string{}))
	})

	t.Run("set empty", func(t *testing.T) {
		require.True(t, tagsDifferent([]string{"A"}, []string{}))
	})

	t.Run("set nil", func(t *testing.T) {
		require.True(t, tagsDifferent([]string{"A"}, nil))
	})

	t.Run("different content", func(t *testing.T) {
		require.True(t, tagsDifferent([]string{"A"}, []string{"B"}))
	})

	t.Run("different lengths", func(t *testing.T) {
		require.True(t, tagsDifferent([]string{"A"}, []string{"A", "B"}))
	})
}

func TestSyncLogic_sidecarTagsDifferent(t *testing.T) {
	type tc struct {
		parent, wanted, sidecar []string
		expect                  bool
	}

	try := func(t *testing.T, test tc) {
		result := sidecarTagsDifferent(test.parent, test.wanted, test.sidecar)
		require.Equal(t, test.expect, result)
	}

	try(t, tc{parent: nil, wanted: nil, sidecar: nil, expect: false})

	// wanted is nil, compare sidecar to parent
	try(t, tc{parent: []string{"foo"}, wanted: nil, sidecar: nil, expect: true})
	try(t, tc{parent: []string{"foo"}, wanted: nil, sidecar: []string{"foo"}, expect: false})
	try(t, tc{parent: []string{"foo"}, wanted: nil, sidecar: []string{"bar"}, expect: true})
	try(t, tc{parent: nil, wanted: nil, sidecar: []string{"foo"}, expect: true})

	// wanted is non-nil, compare sidecar to wanted
	try(t, tc{parent: nil, wanted: []string{"foo"}, sidecar: nil, expect: true})
	try(t, tc{parent: nil, wanted: []string{"foo"}, sidecar: []string{"foo"}, expect: false})
	try(t, tc{parent: nil, wanted: []string{"foo"}, sidecar: []string{"bar"}, expect: true})
	try(t, tc{parent: []string{"foo"}, wanted: []string{"foo"}, sidecar: []string{"bar"}, expect: true})
}

func TestSyncLogic_maybeTweakTags(t *testing.T) {
	t.Parallel()

	differentPointers := func(a, b []string) bool {
		return &(a) != &(b)
	}

	try := func(inConsul, inConsulSC []string, eto bool) {
		wanted := &api.AgentServiceRegistration{
			Tags: []string{"original"},
			Connect: &api.AgentServiceConnect{
				SidecarService: &api.AgentServiceRegistration{
					Tags: []string{"original-sidecar"},
				},
			},
			EnableTagOverride: eto,
		}

		existing := &api.AgentService{Tags: inConsul}
		sidecar := &api.AgentService{Tags: inConsulSC}

		maybeTweakTags(wanted, existing, sidecar)

		switch eto {
		case false:
			require.Equal(t, []string{"original"}, wanted.Tags)
			require.Equal(t, []string{"original-sidecar"}, wanted.Connect.SidecarService.Tags)
			require.True(t, differentPointers(wanted.Tags, wanted.Connect.SidecarService.Tags))
		case true:
			require.Equal(t, inConsul, wanted.Tags)
			require.Equal(t, inConsulSC, wanted.Connect.SidecarService.Tags)
			require.True(t, differentPointers(wanted.Tags, wanted.Connect.SidecarService.Tags))
		}
	}

	try([]string{"original"}, []string{"original-sidecar"}, true)
	try([]string{"original"}, []string{"original-sidecar"}, false)
	try([]string{"modified"}, []string{"original-sidecar"}, true)
	try([]string{"modified"}, []string{"original-sidecar"}, false)
	try([]string{"original"}, []string{"modified-sidecar"}, true)
	try([]string{"original"}, []string{"modified-sidecar"}, false)
	try([]string{"modified"}, []string{"modified-sidecar"}, true)
	try([]string{"modified"}, []string{"modified-sidecar"}, false)
}

func TestSyncLogic_maybeTweakTags_emptySC(t *testing.T) {
	t.Parallel()

	// Check the edge cases where the connect service is deleted on the nomad
	// side (i.e. are we checking multiple nil pointers).

	try := func(asr *api.AgentServiceRegistration) {
		existing := &api.AgentService{Tags: []string{"a", "b"}}
		sidecar := &api.AgentService{Tags: []string{"a", "b"}}
		maybeTweakTags(asr, existing, sidecar)
		require.False(t, !tagsDifferent([]string{"original"}, asr.Tags))
	}

	try(&api.AgentServiceRegistration{
		Tags:              []string{"original"},
		EnableTagOverride: true,
		Connect:           nil, // ooh danger!
	})

	try(&api.AgentServiceRegistration{
		Tags:              []string{"original"},
		EnableTagOverride: true,
		Connect: &api.AgentServiceConnect{
			SidecarService: nil, // ooh danger!
		},
	})
}

func TestSyncLogic_proxyUpstreamsDifferent(t *testing.T) {
	t.Parallel()

	upstream1 := func() api.Upstream {
		return api.Upstream{
			Datacenter:       "sfo",
			DestinationName:  "billing",
			LocalBindAddress: "127.0.0.1",
			LocalBindPort:    5050,
			MeshGateway: api.MeshGatewayConfig{
				Mode: "remote",
			},
			Config: map[string]interface{}{"foo": 1},
		}
	}

	upstream2 := func() api.Upstream {
		return api.Upstream{
			Datacenter:       "ny",
			DestinationName:  "metrics",
			LocalBindAddress: "127.0.0.1",
			LocalBindPort:    6060,
			MeshGateway: api.MeshGatewayConfig{
				Mode: "local",
			},
			Config: nil,
		}
	}

	newASC := func() *api.AgentServiceConnect {
		return &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Proxy: &api.AgentServiceConnectProxyConfig{
					Upstreams: []api.Upstream{
						upstream1(),
						upstream2(),
					},
				},
			},
		}
	}

	original := newASC()

	t.Run("same", func(t *testing.T) {
		require.False(t, proxyUpstreamsDifferent(original, newASC().SidecarService.Proxy))
	})

	type proxy = *api.AgentServiceConnectProxyConfig
	type tweaker = func(proxy)

	try := func(t *testing.T, desc string, tweak tweaker) {
		t.Run(desc, func(t *testing.T) {
			p := newASC().SidecarService.Proxy
			tweak(p)
			require.True(t, proxyUpstreamsDifferent(original, p))
		})
	}

	try(t, "empty upstreams", func(p proxy) {
		p.Upstreams = make([]api.Upstream, 0)
	})

	try(t, "missing upstream", func(p proxy) {
		p.Upstreams = []api.Upstream{
			upstream1(),
		}
	})

	try(t, "extra upstream", func(p proxy) {
		p.Upstreams = []api.Upstream{
			upstream1(),
			upstream2(),
			{
				Datacenter:      "north",
				DestinationName: "dest3",
			},
		}
	})

	try(t, "different datacenter", func(p proxy) {
		diff := upstream2()
		diff.Datacenter = "south"
		p.Upstreams = []api.Upstream{
			upstream1(),
			diff,
		}
	})

	try(t, "different destination", func(p proxy) {
		diff := upstream2()
		diff.DestinationName = "sink"
		p.Upstreams = []api.Upstream{
			upstream1(),
			diff,
		}
	})

	try(t, "different local_bind_address", func(p proxy) {
		diff := upstream2()
		diff.LocalBindAddress = "10.0.0.1"
		p.Upstreams = []api.Upstream{
			upstream1(),
			diff,
		}
	})

	try(t, "different local_bind_port", func(p proxy) {
		diff := upstream2()
		diff.LocalBindPort = 9999
		p.Upstreams = []api.Upstream{
			upstream1(),
			diff,
		}
	})

	try(t, "different mesh gateway mode", func(p proxy) {
		diff := upstream2()
		diff.MeshGateway.Mode = "none"
		p.Upstreams = []api.Upstream{
			upstream1(),
			diff,
		}
	})

	try(t, "different config", func(p proxy) {
		diff := upstream1()
		diff.Config = map[string]interface{}{"foo": 2}
		p.Upstreams = []api.Upstream{
			diff,
			upstream2(),
		}
	})
}
