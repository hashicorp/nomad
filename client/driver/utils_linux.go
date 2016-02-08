package driver

import (
	"os/exec"
	"syscall"

	"fmt"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	cgroupConfig "github.com/opencontainers/runc/libcontainer/configs"
)

// isolateCommand sets the setsid flag in exec.Cmd to true so that the process
// becomes the process leader in a new session and doesn't receive signals that
// are sent to the parent process.
func isolateCommand(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}

// destroyCgroup destroys a cgroup and thereby killing all the processes in that
// group
func destroyCgroup(group *cgroupConfig.Cgroup) error {
	if group == nil {
		return nil
	}
	var manager cgroups.Manager
	manager = &cgroupFs.Manager{Cgroups: group}
	if systemd.UseSystemd() {
		manager = &systemd.Manager{Cgroups: group}
	}

	if err := manager.Destroy(); err != nil {
		return fmt.Errorf("failed to destroy cgroup: %v", err)
	}
	return nil
}
