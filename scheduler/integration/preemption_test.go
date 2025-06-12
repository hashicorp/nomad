package integration

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	psstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/hashicorp/nomad/scheduler/tests"
	"github.com/shoenig/test/must"
)

// TestPreemptionMultiple tests evicting multiple allocations in the same time
func TestPreemptionMultiple(t *testing.T) {
	ci.Parallel(t)

	// The test setup:
	//  * a node with 4 GPUs
	//  * a low priority job with 4 allocs, each is using 1 GPU
	//
	// Then schedule a high priority job needing 2 allocs, using 2 GPUs each.
	// Expectation:
	// All low priority allocs should preempted to accomodate the high priority job
	h := tests.NewHarness(t)

	legacyCpuResources, processorResources := tests.CpuResources(4000)

	// node with 4 GPUs
	node := mock.Node()
	node.NodeResources = &structs.NodeResources{
		Processors: processorResources,
		Cpu:        legacyCpuResources,
		Memory: structs.NodeMemoryResources{
			MemoryMB: 8192,
		},
		Disk: structs.NodeDiskResources{
			DiskMB: 100 * 1024,
		},
		Networks: []*structs.NetworkResource{
			{
				Device: "eth0",
				CIDR:   "192.168.0.100/32",
				MBits:  1000,
			},
		},
		Devices: []*structs.NodeDeviceResource{
			{
				Type:   "gpu",
				Vendor: "nvidia",
				Name:   "1080ti",
				Attributes: map[string]*psstructs.Attribute{
					"memory":           psstructs.NewIntAttribute(11, psstructs.UnitGiB),
					"cuda_cores":       psstructs.NewIntAttribute(3584, ""),
					"graphics_clock":   psstructs.NewIntAttribute(1480, psstructs.UnitMHz),
					"memory_bandwidth": psstructs.NewIntAttribute(11, psstructs.UnitGBPerS),
				},
				Instances: []*structs.NodeDevice{
					{
						ID:      "dev0",
						Healthy: true,
					},
					{
						ID:      "dev1",
						Healthy: true,
					},
					{
						ID:      "dev2",
						Healthy: true,
					},
					{
						ID:      "dev3",
						Healthy: true,
					},
				},
			},
		},
	}

	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))

	// low priority job with 4 allocs using all 4 GPUs
	lowPrioJob := mock.Job()
	lowPrioJob.Priority = 5
	lowPrioJob.TaskGroups[0].Count = 4
	lowPrioJob.TaskGroups[0].Networks = nil
	lowPrioJob.TaskGroups[0].Tasks[0].Services = nil
	lowPrioJob.TaskGroups[0].Tasks[0].Resources.Networks = nil
	lowPrioJob.TaskGroups[0].Tasks[0].Resources.Devices = structs.ResourceDevices{{
		Name:  "gpu",
		Count: 1,
	}}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, lowPrioJob))

	allocs := []*structs.Allocation{}
	allocIDs := map[string]struct{}{}
	for i := 0; i < 4; i++ {
		alloc := tests.CreateAllocWithDevice(uuid.Generate(), lowPrioJob, lowPrioJob.TaskGroups[0].Tasks[0].Resources, &structs.AllocatedDeviceResource{
			Type:      "gpu",
			Vendor:    "nvidia",
			Name:      "1080ti",
			DeviceIDs: []string{fmt.Sprintf("dev%d", i)},
		})
		alloc.NodeID = node.ID

		allocs = append(allocs, alloc)
		allocIDs[alloc.ID] = struct{}{}
	}
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), allocs))

	// new high priority job with 2 allocs, each using 2 GPUs
	highPrioJob := mock.Job()
	highPrioJob.Priority = 100
	highPrioJob.TaskGroups[0].Count = 2
	highPrioJob.TaskGroups[0].Networks = nil
	highPrioJob.TaskGroups[0].Tasks[0].Services = nil
	highPrioJob.TaskGroups[0].Tasks[0].Resources.Networks = nil
	highPrioJob.TaskGroups[0].Tasks[0].Resources.Devices = structs.ResourceDevices{{
		Name:  "gpu",
		Count: 2,
	}}
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, highPrioJob))

	// schedule
	eval := &structs.Evaluation{
		Namespace:   structs.DefaultNamespace,
		ID:          uuid.Generate(),
		Priority:    highPrioJob.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister,
		JobID:       highPrioJob.ID,
		Status:      structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))

	// Process the evaluation
	must.NoError(t, h.Process(scheduler.NewServiceScheduler, eval))
	must.Len(t, 1, h.Plans)
	must.MapContainsKey(t, h.Plans[0].NodePreemptions, node.ID)

	preempted := map[string]struct{}{}
	for _, alloc := range h.Plans[0].NodePreemptions[node.ID] {
		preempted[alloc.ID] = struct{}{}
	}
	must.Eq(t, allocIDs, preempted)
}
