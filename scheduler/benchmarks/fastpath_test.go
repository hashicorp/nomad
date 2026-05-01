package benchmarks

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/hashicorp/nomad/scheduler/tests"
	"github.com/shoenig/test/must"
)

// BenchmarkFastPath exercises the service scheduler using the fast path and
// non-fast path to find a specific node
func BenchmarkFastPath(b *testing.B) {

	clusterSizes := []int{500, 1000, 5000, 10000}

	type benchmark struct {
		name         string
		clusterSize  int
		withFastPath bool
		target       string
	}

	benchmarks := []benchmark{}
	for _, clusterSize := range clusterSizes {
		benchmarks = append(benchmarks,
			benchmark{
				name:         fmt.Sprintf("%d nodes fast path", clusterSize),
				clusterSize:  clusterSize,
				withFastPath: true,
			},
		)
		benchmarks = append(benchmarks,
			benchmark{
				name:         fmt.Sprintf("%d nodes existing", clusterSize),
				clusterSize:  clusterSize,
				withFastPath: false,
			},
		)
	}

	for _, bm := range benchmarks {
		h := tests.NewHarness(b)
		upsertNodes(h, bm.clusterSize, 10)
		upsertSandboxVolume(h)
		job := generateFastPathJob(h, bm.withFastPath)
		h.SetNoSubmit()
		eval := upsertJob(h, job)
		b.ResetTimer()

		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				err := h.Process(scheduler.NewServiceScheduler, eval)
				must.NoError(b, err)
			}
		})
	}

}

func upsertSandboxVolume(h *tests.Harness) {
	iter, _ := h.State.Nodes(nil)
	for range 3 {
		obj := iter.Next()
		node := obj.(*structs.Node)
		err := h.State.UpsertSandboxVolume(h.NextIndex(), &structs.SandboxVolume{
			ID:            uuid.Generate(),
			Namespace:     "default",
			Name:          "foo",
			AllocIDs:      []string{},
			NodeID:        node.ID,
			CapacityBytes: 0,
			MaxClaims:     3,
			TTL:           time.Hour,
		})
		if err != nil {
			panic(err)
		}
	}
}

func generateFastPathJob(h *tests.Harness, withFastPath bool) *structs.Job {
	job := mock.Job()
	job.Datacenters = []string{"dc-1", "dc-2"}

	if withFastPath {
		job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
			"foo": {
				Name:           "foo",
				Type:           structs.VolumeTypeSandbox,
				Source:         "foo",
				AccessMode:     structs.HostVolumeAccessModeSingleNodeWriter,
				AttachmentMode: structs.HostVolumeAttachmentModeBlockDevice,
				Sandbox: &structs.SandboxVolumeRequest{
					MaxCount:  1,
					MaxClaims: 1,
					TTL:       time.Hour,
					MinBytes:  100_000_000,
				},
			},
		}
		job.Constraints = []*structs.Constraint{}
		job.TaskGroups[0].Constraints = []*structs.Constraint{}
	} else {
		// grab the first node at random
		iter, _ := h.State.Nodes(nil)
		obj := iter.Next()
		node := obj.(*structs.Node)
		job.Constraints = []*structs.Constraint{{
			LTarget: "${node.unique.id}",
			RTarget: node.ID,
			Operand: "=",
		}}
		job.TaskGroups[0].Constraints = []*structs.Constraint{}
	}

	job.TaskGroups[0].Count = 3
	job.TaskGroups[0].Services = []*structs.Service{}
	job.TaskGroups[0].Tasks[0].Resources = &structs.Resources{
		CPU:      100,
		MemoryMB: 100,
	}
	return job
}
