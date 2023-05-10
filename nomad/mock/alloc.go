package mock

// partially backported from 1.5.x -- other alloc things are in mock.go

import (
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

func MinAlloc() *structs.Allocation {
	job := MinJob()
	group := job.TaskGroups[0]
	task := group.Tasks[0]
	return &structs.Allocation{
		ID:            uuid.Generate(),
		EvalID:        uuid.Generate(),
		NodeID:        uuid.Generate(),
		Job:           job,
		TaskGroup:     group.Name,
		ClientStatus:  structs.AllocClientStatusPending,
		DesiredStatus: structs.AllocDesiredStatusRun,
		AllocatedResources: &structs.AllocatedResources{
			Tasks: map[string]*structs.AllocatedTaskResources{
				task.Name: {
					Cpu: structs.AllocatedCpuResources{
						CpuShares: 100,
					},
					Memory: structs.AllocatedMemoryResources{
						MemoryMB: 256,
					},
				},
			},
			Shared: structs.AllocatedSharedResources{
				DiskMB: 150,
			},
		},
	}
}
