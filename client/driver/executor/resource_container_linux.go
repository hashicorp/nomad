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

// cleanup removes this host's Cgroup
func (rc *resourceContainer) cleanup() error {
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
