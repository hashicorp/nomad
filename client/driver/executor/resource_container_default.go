// +build darwin dragonfly freebsd netbsd openbsd solaris windows

package executor

import (
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
)

type resourceContainer struct {
}

func clientCleanup(ic *dstructs.IsolationConfig, pid int) error {
	return nil
}

func (rc *resourceContainer) executorCleanup() error {
	return nil
}

func (rc *resourceContainer) getIsolationConfig() *dstructs.IsolationConfig {
	return nil
}
