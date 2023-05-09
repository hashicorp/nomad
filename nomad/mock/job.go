package mock

// partially backported from 1.5.x -- other job things are in mock.go

import (
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/structs"
)

// MinJob returns a minimal service job with a mock driver task.
func MinJob() *structs.Job {
	job := &structs.Job{
		ID:     "j" + uuid.Short(),
		Name:   "j",
		Region: "global",
		Type:   "service",
		TaskGroups: []*structs.TaskGroup{
			{
				Name:  "g",
				Count: 1,
				Tasks: []*structs.Task{
					{
						Name:   "t",
						Driver: "mock_driver",
						Config: map[string]any{
							// An empty config actually causes an error, so set a reasonably
							// long run_for duration.
							"run_for": "10m",
						},
						LogConfig: structs.DefaultLogConfig(),
					},
				},
			},
		},
		Datacenters: []string{"dc1"},
		Priority:    50,
	}
	job.Canonicalize()
	return job
}
