package feasible

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestSandboxVolumes(t *testing.T) {
	ci.Parallel(t)

	store, ctx := MockContext(t)
	nodes := []*RankedNode{
		{Node: mock.Node()},
		{Node: mock.Node()},
		{Node: mock.Node()},
		{Node: mock.Node()},
		{Node: mock.Node()},
	}
	index, _ := store.LatestIndex()

	alloc := mock.AllocForNode(nodes[0].Node)
	alloc.AllocatedResources.Shared.Sandboxes = []*structs.AllocatedSandbox{{
		ID:            uuid.Generate(),
		Namespace:     alloc.Namespace,
		Name:          "foo",
		CapacityBytes: 100_000_000,
		TTL:           time.Hour,
	}}

	must.NoError(t, store.ClaimSandboxVolume(index, alloc))

	static := NewStaticRankIterator(ctx, nodes)
	binp := NewBinPackIterator(ctx, static, false, 0)
	binp.SetTaskGroup(alloc.Job.LookupTaskGroup(alloc.Name))
	binp.SetSchedulerConfiguration(testSchedulerConfig)

	//ctx.state.SandboxesByName(memdb.WatchSet, string, string)

}
