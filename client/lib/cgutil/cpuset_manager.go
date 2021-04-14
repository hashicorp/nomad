package cgutil

import (
	"context"

	"github.com/hashicorp/nomad/lib/cpuset"
	"github.com/hashicorp/nomad/nomad/structs"
)

// CpusetManager is used to setup cpuset cgroups for each task. A pool of shared cpus is managed for
// tasks which don't require any reserved cores and a cgroup is managed seperetly for each task which
// require reserved cores.
type CpusetManager interface {
	// Init should be called before any tasks are managed to ensure the cgroup parent exists and
	// check that proper permissions are granted to manage cgroups.
	Init() error

	// AddAlloc adds an allocation to the manager
	AddAlloc(alloc *structs.Allocation)

	// RemoveAlloc removes an alloc by ID from the manager
	RemoveAlloc(allocID string)

	// CgroupPathFor returns a callback for getting the cgroup path and any error that may have occurred during
	// cgroup initialization. The callback will block if the cgroup has not been created
	CgroupPathFor(allocID, taskName string) CgroupPathGetter
}

// CgroupPathGetter is a function which returns the cgroup path and any error which ocured during cgroup initialization.
// It should block until the cgroup has been created or an error is reported
type CgroupPathGetter func(context.Context) (path string, err error)

type TaskCgroupInfo struct {
	CgroupPath         string
	RelativeCgroupPath string
	Cpuset             cpuset.CPUSet
	Error              error
}

func NoopCpusetManager() CpusetManager { return noopCpusetManager{} }

type noopCpusetManager struct{}

func (n noopCpusetManager) Init() error {
	return nil
}

func (n noopCpusetManager) AddAlloc(alloc *structs.Allocation) {
}

func (n noopCpusetManager) RemoveAlloc(allocID string) {
}

func (n noopCpusetManager) CgroupPathFor(allocID, task string) CgroupPathGetter {
	return func(context.Context) (string, error) { return "", nil }
}
