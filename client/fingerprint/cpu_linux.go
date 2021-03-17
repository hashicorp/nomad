package fingerprint

import (
	"path/filepath"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	"github.com/opencontainers/runc/libcontainer/configs"
)

const DefaultCgroupParent = "/nomad"

func getCgroupPathHelper(subsystem, cgroup string) (string, error) {
	mnt, root, err := cgroups.FindCgroupMountpointAndRoot("", subsystem)
	if err != nil {
		return "", err
	}

	// This is needed for nested containers, because in /proc/self/cgroup we
	// see paths from host, which don't exist in container.
	relCgroup, err := filepath.Rel(root, cgroup)
	if err != nil {
		return "", err
	}

	return filepath.Join(mnt, relCgroup), nil
}

func deriveCpuset(req *FingerprintRequest) ([]uint16, error) {
	var cgroupParent string
	if req != nil && req.Config != nil {
		cgroupParent = req.Config.CgroupParent
	}
	if cgroupParent == "" {
		cgroupParent = DefaultCgroupParent
	}

	cgroupPath, err := getCgroupPathHelper("cpuset", cgroupParent)
	if err != nil {
		return nil, err
	}

	man := cgroupFs.NewManager(&configs.Cgroup{Path: cgroupParent}, map[string]string{"cpuset": cgroupPath}, false)
	stats, err := man.GetStats()
	if err != nil {
		return nil, err
	}
	return stats.CPUSetStats.CPUs, nil
}
