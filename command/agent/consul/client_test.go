package consul

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestSyncLogic_agentServiceUpdateRequired(t *testing.T) {
	t.Parallel()

	wanted := api.AgentServiceRegistration{
		Kind:              "service",
		ID:                "_id",
		Name:              "name",
		Tags:              []string{"a", "b"},
		Port:              9000,
		Address:           "1.1.1.1",
		EnableTagOverride: true,
		Meta:              map[string]string{"foo": "1"},
	}

	existing := &api.AgentService{
		Kind:              "service",
		ID:                "_id",
		Service:           "name",
		Tags:              []string{"a", "b"},
		Port:              9000,
		Address:           "1.1.1.1",
		EnableTagOverride: true,
		Meta:              map[string]string{"foo": "1"},
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
		result := agentServiceUpdateRequired(reason, tweak(wanted), existing)
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

	t.Run("different tags syncNewOps eto->true", func(t *testing.T) {
		// sync is required even though eto=true, because NewOps indicates the
		// service definition  in nomad has changed (e.g. job run a modified job)
		try(t, true, syncNewOps, func(w asr) *asr {
			w.Tags = []string{"other", "tags"}
			return &w
		})
	})

	t.Run("different tags syncPeriodic eto->true", func(t *testing.T) {
		// sync is not required since eto=true and this is a periodic sync
		// with consul - in which case we keep Consul's definition of the tags
		try(t, false, syncPeriodic, func(w asr) *asr {
			w.Tags = []string{"other", "tags"}
			return &w
		})
	})

	// for remaining tests, EnableTagOverride = false
	wanted.EnableTagOverride = false
	existing.EnableTagOverride = false

	t.Run("different tags : syncPeriodic : eto->false", func(t *testing.T) {
		// sync is required because eto=false and the tags do not match
		try(t, true, syncPeriodic, func(w asr) *asr {
			w.Tags = []string{"other", "tags"}
			return &w
		})
	})

	t.Run("different tags : syncNewOps : eto->false", func(t *testing.T) {
		// sync is required because it was triggered by NewOps and the tags
		// do not match
		try(t, true, syncNewOps, func(w asr) *asr {
			w.Tags = []string{"other", "tags"}
			return &w
		})
	})
}

func TestSyncLogic_maybeTweakTags(t *testing.T) {
	t.Parallel()
	r := require.New(t)

	wanted := &api.AgentServiceRegistration{Tags: []string{"original"}}
	existing := &api.AgentService{Tags: []string{"other"}}
	maybeTweakTags(wanted, existing)
	r.Equal([]string{"original"}, wanted.Tags)

	wantedETO := &api.AgentServiceRegistration{Tags: []string{"original"}, EnableTagOverride: true}
	existingETO := &api.AgentService{Tags: []string{"other"}, EnableTagOverride: true}
	maybeTweakTags(wantedETO, existingETO)
	r.Equal(existingETO.Tags, wantedETO.Tags)
	r.False(&(existingETO.Tags) == &(wantedETO.Tags))
}
