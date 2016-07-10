package executor

import (
	"os"
	"sync"

	dstructs "github.com/hashicorp/nomad/client/driver/structs"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"
)

type resourceContainer struct {
	groups  *cgroupConfig.Cgroup
	cgPaths map[string]string
	cgLock  sync.Mutex
}

// clientCleanup remoevs this host's Cgroup from the Nomad Client's context
func clientCleanup(ic *dstructs.IsolationConfig, pid int) error {
	if err := DestroyCgroup(ic.Cgroup, ic.CgroupPaths, pid); err != nil {
		return err
	}
	return nil
}

// cleanup removes this host's Cgroup from within an Executor's context
func (rc *resourceContainer) executorCleanup() error {
	rc.cgLock.Lock()
	defer rc.cgLock.Unlock()
	if err := DestroyCgroup(rc.groups, rc.cgPaths, os.Getpid()); err != nil {
		return err
	}
	return nil
}

func (rc *resourceContainer) getIsolationConfig() *dstructs.IsolationConfig {
	return &dstructs.IsolationConfig{
		Cgroup:      rc.groups,
		CgroupPaths: rc.cgPaths,
	}
}
