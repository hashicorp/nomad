package executor

import (
	"os"
	"sync"

	"github.com/hashicorp/nomad/client/stats"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"
)

// resourceContainerContext is a platform-specific struct for managing a
// resource container.  In the case of Linux, this is used to control Cgroups.
type resourceContainerContext struct {
	groups *cgroupConfig.Cgroup
	cgLock sync.Mutex
}

// cleanup removes this host's Cgroup from within an Executor's context
func (rc *resourceContainerContext) executorCleanup() error {
	rc.cgLock.Lock()
	defer rc.cgLock.Unlock()
	if err := DestroyCgroup(rc.groups, os.Getpid()); err != nil {
		return err
	}
	return nil
}

func (rc *resourceContainerContext) isEmpty() bool {
	return rc.groups == nil
}

func (rc *resourceContainerContext) getAllPidsByCgroup() (map[int]*nomadPid, error) {
	nPids := map[int]*nomadPid{}

	if rc.groups == nil {
		return nPids, nil
	}

	var path string
	if p, ok := rc.groups.Paths["freezer"]; ok {
		path = p
	} else {
		path = rc.groups.Path
	}

	pids, err := cgroups.GetAllPids(path)
	if err != nil {
		return nPids, err
	}

	for _, pid := range pids {
		nPids[pid] = &nomadPid{
			pid:           pid,
			cpuStatsTotal: stats.NewCpuStats(),
			cpuStatsUser:  stats.NewCpuStats(),
			cpuStatsSys:   stats.NewCpuStats(),
		}
	}

	return nPids, nil
}
