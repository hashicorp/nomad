package drainerv2

import "github.com/hashicorp/nomad/nomad/structs"

type DrainingJobWatcher interface {
	RegisterJob(jobID, namespace string)
	Drain() <-chan []*structs.Allocation
}
