package consul

import (
	"reflect"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

var (
	// the service as known by nomad
	wanted = api.AgentServiceRegistration{
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
			},
		},
	}

	// the service (and + connect proxy) as known by consul
	existing = &api.AgentService{
		Kind:              "",
		ID:                "aca4c175-1778-5ef4-0220-2ab434147d35",
		Service:           "myservice",
		Tags:              []string{"a", "b"},
		Port:              9000,
		Address:           "1.1.1.1",
		EnableTagOverride: true,
		Meta:              map[string]string{"foo": "1"},
	}

	sidecar = &api.AgentService{
		Kind:    "connect-proxy",
		ID:      "_nomad-task-8e8413af-b5bb-aa67-2c24-c146c45f1ec9-group-mygroup-myservice-9001-sidecar-proxy",
		Service: "myservice-sidecar-proxy",
		Tags:    []string{"x", "y", "z"},
	}
)

func TestSyncLogic_agentServiceUpdateRequired(t *testing.T) {
	t.Parallel()

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
		result := agentServiceUpdateRequired(reason, tweak(wanted), existing, sidecar)
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
	wanted.EnableTagOverride = false
	existing.EnableTagOverride = false

	t.Run("different tags syncPeriodic eto=false", func(t *testing.T) {
		// sync is required because eto=false and the tags do not match
		try(t, true, syncPeriodic, func(w asr) *asr {
			w.Tags = []string{"other", "tags"}
			return &w
		})
	})

	t.Run("different tags syncNewOps eto=false", func(t *testing.T) {
		// sync is required because eto=false and the tags do not match
		try(t, true, syncNewOps, func(w asr) *asr {
			w.Tags = []string{"other", "tags"}
			return &w
		})
	})

	t.Run("different sidecar tags on syncPeriodic eto=false", func(t *testing.T) {
		// like the parent service, sync is required because eto=false and the
		// sidecar's tags do not match
		try(t, true, syncPeriodic, func(w asr) *asr {
			w.Connect.SidecarService.Tags = []string{"other", "tags"}
			return &w
		})
	})

	t.Run("different sidecar tags syncNewOps eto=false", func(t *testing.T) {
		// like the parent service, sync is required because eto=false and the
		// sidecar's tags do not match
		try(t, true, syncNewOps, func(w asr) *asr {
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
		maybeTweakTags(asr, existing, sidecar)
		require.False(t, reflect.DeepEqual([]string{"original"}, asr.Tags))
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
