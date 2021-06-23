package docker

import (
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func setCPUSetCgroup(path string, pid int) error {
	// Sometimes the container exists before we can write the
	// cgroup resulting in an error which can be ignored.
	err := cgroups.WriteCgroupProc(path, pid)
	if err != nil && strings.Contains(err.Error(), "no such process") {
		return nil
	}
	return err
}
