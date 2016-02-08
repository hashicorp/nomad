package driver

import (
	"os/exec"

	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"
)

// TODO Figure out if this is needed in Wondows
func isolateCommand(cmd *exec.Cmd) {
}

func destroyCgroup(group *cgroupConfig.Cgroup) error {
	return nil
}
