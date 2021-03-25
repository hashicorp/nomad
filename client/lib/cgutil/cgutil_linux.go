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
	DefaultCgroupParent    = "/nomad"
	SharedCpusetCgroupName = "shared"
)

// InitCpusetParent checks that the cgroup parent and expected child cgroups have been created
// If the cgroup parent is set to /nomad then this will ensure that the /nomad/shared
// cgroup is initialized. The /nomad/reserved cgroup will be lazily created when a workload
// with reserved cores is created
func InitCpusetParent(cgroupParent string) error {
	if cgroupParent == "" {
		cgroupParent = DefaultCgroupParent
	}
	var err error
	if cgroupParent, err = getCgroupPathHelper("cpuset", cgroupParent); err != nil {
		return err
	}

	// 'ensureParent' start with parent because we don't want to
	// explicitly inherit from parent, it could conflict with
	// 'cpuset.cpu_exclusive'.
	if err := cpusetEnsureParent(cgroupParent); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(cgroupParent, SharedCpusetCgroupName), 0755); err != nil && !os.IsExist(err) {
		return err
	}

	return nil
}

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
		if err := fscommon.WriteFile(current, "cpuset.cpus", string(parentCpus)); err != nil {
			return err
		}
	}
	if isEmptyCpuset(currentMems) {
		if err := fscommon.WriteFile(current, "cpuset.mems", string(parentMems)); err != nil {
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
