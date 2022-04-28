package cgutil

import (
	"os"
	"path/filepath"

	cgroupFs "github.com/opencontainers/runc/libcontainer/cgroups/fs"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

const (
	DefaultCgroupParent      = "/nomad"
	SharedCpusetCgroupName   = "shared"
	ReservedCpusetCgroupName = "reserved"
)

func GetCPUsFromCgroup(group string) ([]uint16, error) {
	cgroupPath, err := getCgroupPathHelper("cpuset", group)
	if err != nil {
		return nil, err
	}

	man := cgroupFs.NewManager(&configs.Cgroup{Path: group}, map[string]string{"cpuset": cgroupPath}, false)
	stats, err := man.GetStats()
	if err != nil {
		return nil, err
	}
	return stats.CPUSetStats.CPUs, nil
}

func getCpusetSubsystemSettings(parent string) (cpus, mems string, err error) {
	if cpus, err = fscommon.ReadFile(parent, "cpuset.cpus"); err != nil {
		return
	}
	if mems, err = fscommon.ReadFile(parent, "cpuset.mems"); err != nil {
		return
	}
	return cpus, mems, nil
}

// cpusetEnsureParent makes sure that the parent directories of current
// are created and populated with the proper cpus and mems files copied
// from their respective parent. It does that recursively, starting from
// the top of the cpuset hierarchy (i.e. cpuset cgroup mount point).
func cpusetEnsureParent(current string) error {
	var st unix.Statfs_t

	parent := filepath.Dir(current)
	err := unix.Statfs(parent, &st)
	if err == nil && st.Type != unix.CGROUP_SUPER_MAGIC {
		return nil
	}
	// Treat non-existing directory as cgroupfs as it will be created,
	// and the root cpuset directory obviously exists.
	if err != nil && err != unix.ENOENT {
		return &os.PathError{Op: "statfs", Path: parent, Err: err}
	}

	if err := cpusetEnsureParent(parent); err != nil {
		return err
	}
	if err := os.Mkdir(current, 0755); err != nil && !os.IsExist(err) {
		return err
	}
	return cpusetCopyIfNeeded(current, parent)
}

// cpusetCopyIfNeeded copies the cpuset.cpus and cpuset.mems from the parent
// directory to the current directory if the file's contents are 0
func cpusetCopyIfNeeded(current, parent string) error {
	currentCpus, currentMems, err := getCpusetSubsystemSettings(current)
	if err != nil {
		return err
	}
	parentCpus, parentMems, err := getCpusetSubsystemSettings(parent)
	if err != nil {
		return err
	}

	if isEmptyCpuset(currentCpus) {
		if err := fscommon.WriteFile(current, "cpuset.cpus", parentCpus); err != nil {
			return err
		}
	}
	if isEmptyCpuset(currentMems) {
		if err := fscommon.WriteFile(current, "cpuset.mems", parentMems); err != nil {
			return err
		}
	}
	return nil
}

func isEmptyCpuset(str string) bool {
	return str == "" || str == "\n"
}

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

// FindCgroupMountpointDir is used to find the cgroup mount point on a Linux
// system.
func FindCgroupMountpointDir() (string, error) {
	mount, err := cgroups.GetCgroupMounts(false)
	if err != nil {
		return "", err
	}
	// It's okay if the mount point is not discovered
	if len(mount) == 0 {
		return "", nil
	}
	return mount[0].Mountpoint, nil
}
