package executor

import (
	"os"
	"sync"

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
