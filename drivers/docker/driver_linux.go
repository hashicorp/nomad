package docker

import (
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func setCPUSetCgroup(path string, pid int) error {
	// Sometimes the container exists before we can write the
	// cgroup resulting in an error which can be ignored.
	if err := cgroups.WriteCgroupProc(path, pid); err != nil {
		if strings.Contains(err.Error(), "no such process") {
			return nil
		}
		return err
	}
	return nil
}
