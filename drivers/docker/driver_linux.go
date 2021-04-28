package docker

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func setCPUSetCgroup(path string, pid int) error {
	return cgroups.WriteCgroupProc(path, pid)
}
