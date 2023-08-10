// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/serviceregistration"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
)

func TestSyncLogic_maybeTweakTaggedAddresses(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name     string
		wanted   map[string]api.ServiceAddress
		existing map[string]api.ServiceAddress
		id       string
		exp      []string
	}{
		{
			name:   "not managed by nomad",
			id:     "_nomad-other-hello",
			wanted: map[string]api.ServiceAddress{
				// empty
			},
			existing: map[string]api.ServiceAddress{
				"lan_ipv4": {},
				"wan_ipv4": {},
				"custom":   {},
			},
			exp: []string{"lan_ipv4", "wan_ipv4", "custom"},
		},
		{
			name: "remove defaults",
			id:   "_nomad-task-hello",
			wanted: map[string]api.ServiceAddress{
				"lan_custom": {},
				"wan_custom": {},
			},
			existing: map[string]api.ServiceAddress{
				"lan_ipv4":   {},
				"wan_ipv4":   {},
				"lan_ipv6":   {},
				"wan_ipv6":   {},
				"lan_custom": {},
				"wan_custom": {},
			},
			exp: []string{"lan_custom", "wan_custom"},
		},
		{
			name: "overridden defaults",
			id:   "_nomad-task-hello",
			wanted: map[string]api.ServiceAddress{
				"lan_ipv4": {},
				"wan_ipv4": {},
				"lan_ipv6": {},
				"wan_ipv6": {},
				"custom":   {},
			},
			existing: map[string]api.ServiceAddress{
				"lan_ipv4": {},
				"wan_ipv4": {},
				"lan_ipv6": {},
				"wan_ipv6": {},
				"custom":   {},
			},
			exp: []string{"lan_ipv4", "wan_ipv4", "lan_ipv6", "wan_ipv6", "custom"},
		},
		{
			name: "applies to nomad client",
			id:   "_nomad-client-12345",
			wanted: map[string]api.ServiceAddress{
				"custom": {},
			},
			existing: map[string]api.ServiceAddress{
				"lan_ipv4": {},
				"wan_ipv4": {},
				"lan_ipv6": {},
				"wan_ipv6": {},
				"custom":   {},
			},
			exp: []string{"custom"},
		},
		{
			name: "applies to nomad server",
			id:   "_nomad-server-12345",
			wanted: map[string]api.ServiceAddress{
				"custom": {},
			},
			existing: map[string]api.ServiceAddress{
				"lan_ipv4": {},
				"wan_ipv4": {},
				"lan_ipv6": {},
				"wan_ipv6": {},
				"custom":   {},
			},
			exp: []string{"custom"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			asr := &api.AgentServiceRegistration{
				ID:              tc.id,
				TaggedAddresses: maps.Clone(tc.wanted),
			}
			as := &api.AgentService{
				TaggedAddresses: maps.Clone(tc.existing),
			}
			maybeTweakTaggedAddresses(asr, as)
			must.MapContainsKeys(t, as.TaggedAddresses, tc.exp)
		})
	}
}

func TestSyncLogic_agentServiceUpdateRequired(t *testing.T) {
	ci.Parallel(t)

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
			TaggedAddresses: map[string]api.ServiceAddress{
				"public_wan": {Address: "1.2.3.4", Port: 8080},
			},
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
		TaggedAddresses: map[string]api.ServiceAddress{
			"public_wan": {Address: "1.2.3.4", Port: 8080},
		},
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

	s := &ServiceClient{
		logger: testlog.HCLogger(t),
	}

	try := func(
		t *testing.T,
		exp bool,
		reason syncReason,
		tweak tweaker) {
		result := s.agentServiceUpdateRequired(reason, tweak(wanted()), existing, sidecar)
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

	t.Run("different tagged addresses", func(t *testing.T) {
		try(t, true, syncNewOps, func(w asr) *asr {
			w.TaggedAddresses = map[string]api.ServiceAddress{
				"public_wan": {Address: "5.6.7.8", Port: 8080},
			}
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

func TestSyncLogic_sidecarTagsDifferent(t *testing.T) {
	ci.Parallel(t)

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
	ci.Parallel(t)

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
	ci.Parallel(t)

	// Check the edge cases where the connect service is deleted on the nomad
	// side (i.e. are we checking multiple nil pointers).

	try := func(asr *api.AgentServiceRegistration) {
		existing := &api.AgentService{Tags: []string{"a", "b"}}
		sidecar := &api.AgentService{Tags: []string{"a", "b"}}
		maybeTweakTags(asr, existing, sidecar)
		must.NotEq(t, []string{"original"}, asr.Tags)
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

// TestServiceRegistration_CheckOnUpdate tests that a ServiceRegistrations
// CheckOnUpdate is populated and updated properly
func TestServiceRegistration_CheckOnUpdate(t *testing.T) {
	ci.Parallel(t)

	mockAgent := NewMockAgent(ossFeatures)
	namespacesClient := NewNamespacesClient(NewMockNamespaces(nil), mockAgent)
	logger := testlog.HCLogger(t)
	sc := NewServiceClient(mockAgent, namespacesClient, logger, true)

	allocID := uuid.Generate()
	ws := &serviceregistration.WorkloadServices{
		AllocInfo: structs.AllocInfo{
			AllocID: allocID,
			Task:    "taskname",
		},
		Restarter: &restartRecorder{},
		Services: []*structs.Service{
			{
				Name:      "taskname-service",
				PortLabel: "x",
				Tags:      []string{"tag1", "tag2"},
				Meta:      map[string]string{"meta1": "foo"},
				Checks: []*structs.ServiceCheck{
					{

						Name:      "c1",
						Type:      "tcp",
						Interval:  time.Second,
						Timeout:   time.Second,
						PortLabel: "x",
						OnUpdate:  structs.OnUpdateIgnoreWarn,
					},
				},
			},
		},
		Networks: []*structs.NetworkResource{
			{
				DynamicPorts: []structs.Port{
					{Label: "x", Value: xPort},
					{Label: "y", Value: yPort},
				},
			},
		},
	}

	require.NoError(t, sc.RegisterWorkload(ws))

	require.NotNil(t, sc.allocRegistrations[allocID])

	allocReg := sc.allocRegistrations[allocID]
	serviceReg := allocReg.Tasks["taskname"]
	require.NotNil(t, serviceReg)

	// Ensure that CheckOnUpdate was set correctly
	require.Len(t, serviceReg.Services, 1)
	for _, sreg := range serviceReg.Services {
		require.NotEmpty(t, sreg.CheckOnUpdate)
		for _, onupdate := range sreg.CheckOnUpdate {
			require.Equal(t, structs.OnUpdateIgnoreWarn, onupdate)
		}
	}

	// Update
	wsUpdate := new(serviceregistration.WorkloadServices)
	*wsUpdate = *ws
	wsUpdate.Services[0].Checks[0].OnUpdate = structs.OnUpdateRequireHealthy

	require.NoError(t, sc.UpdateWorkload(ws, wsUpdate))

	require.NotNil(t, sc.allocRegistrations[allocID])

	allocReg = sc.allocRegistrations[allocID]
	serviceReg = allocReg.Tasks["taskname"]
	require.NotNil(t, serviceReg)

	// Ensure that CheckOnUpdate was updated correctly
	require.Len(t, serviceReg.Services, 1)
	for _, sreg := range serviceReg.Services {
		require.NotEmpty(t, sreg.CheckOnUpdate)
		for _, onupdate := range sreg.CheckOnUpdate {
			require.Equal(t, structs.OnUpdateRequireHealthy, onupdate)
		}
	}
}

func TestSyncLogic_proxyUpstreamsDifferent(t *testing.T) {
	ci.Parallel(t)

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

func TestSyncReason_String(t *testing.T) {
	ci.Parallel(t)

	require.Equal(t, "periodic", fmt.Sprintf("%s", syncPeriodic))
	require.Equal(t, "shutdown", fmt.Sprintf("%s", syncShutdown))
	require.Equal(t, "operations", fmt.Sprintf("%s", syncNewOps))
	require.Equal(t, "unexpected", fmt.Sprintf("%s", syncReason(128)))
}

func TestSyncOps_empty(t *testing.T) {
	ci.Parallel(t)

	try := func(ops *operations, exp bool) {
		require.Equal(t, exp, ops.empty())
	}

	try(&operations{regServices: make([]*api.AgentServiceRegistration, 1)}, false)
	try(&operations{regChecks: make([]*api.AgentCheckRegistration, 1)}, false)
	try(&operations{deregServices: make([]string, 1)}, false)
	try(&operations{deregChecks: make([]string, 1)}, false)
	try(&operations{}, true)
	try(nil, true)
}

func TestSyncLogic_maybeSidecarProxyCheck(t *testing.T) {
	ci.Parallel(t)

	try := func(input string, exp bool) {
		result := maybeSidecarProxyCheck(input)
		require.Equal(t, exp, result)
	}

	try("service:_nomad-task-2f5fb517-57d4-44ee-7780-dc1cb6e103cd-group-api-count-api-9001-sidecar-proxy", true)
	try("service:_nomad-task-2f5fb517-57d4-44ee-7780-dc1cb6e103cd-group-api-count-api-9001-sidecar-proxy:1", true)
	try("service:_nomad-task-2f5fb517-57d4-44ee-7780-dc1cb6e103cd-group-api-count-api-9001-sidecar-proxy:2", true)
	try("service:_nomad-task-2f5fb517-57d4-44ee-7780-dc1cb6e103cd-group-api-count-api-9001", false)
	try("_nomad-task-2f5fb517-57d4-44ee-7780-dc1cb6e103cd-group-api-count-api-9001-sidecar-proxy:1", false)
	try("service:_nomad-task-2f5fb517-57d4-44ee-7780-dc1cb6e103cd-group-api-count-api-9001-sidecar-proxy:X", false)
	try("service:_nomad-task-2f5fb517-57d4-44ee-7780-dc1cb6e103cd-group-api-count-api-9001-sidecar-proxy: ", false)
	try("service", false)
}

func TestSyncLogic_parseTaggedAddresses(t *testing.T) {
	ci.Parallel(t)

	t.Run("nil", func(t *testing.T) {
		m, err := parseTaggedAddresses(nil, 0)
		must.NoError(t, err)
		must.MapEmpty(t, m)
	})

	t.Run("parse fail", func(t *testing.T) {
		ta := map[string]string{
			"public_wan": "not an address",
		}
		result, err := parseTaggedAddresses(ta, 8080)
		must.Error(t, err)
		must.MapEmpty(t, result)
	})

	t.Run("parse address", func(t *testing.T) {
		ta := map[string]string{
			"public_wan": "1.2.3.4",
		}
		result, err := parseTaggedAddresses(ta, 8080)
		must.NoError(t, err)
		must.MapEq(t, map[string]api.ServiceAddress{
			"public_wan": {Address: "1.2.3.4", Port: 8080},
		}, result)
	})

	t.Run("parse address and port", func(t *testing.T) {
		ta := map[string]string{
			"public_wan": "1.2.3.4:9999",
		}
		result, err := parseTaggedAddresses(ta, 8080)
		must.NoError(t, err)
		must.MapEq(t, map[string]api.ServiceAddress{
			"public_wan": {Address: "1.2.3.4", Port: 9999},
		}, result)
	})
}
