//go:build linux

package cgutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	lcc "github.com/opencontainers/runc/libcontainer/configs"
)

// UseV2 indicates whether only cgroups.v2 is enabled. If cgroups.v2 is not
// enabled or is running in hybrid mode with cgroups.v1, Nomad will make use of
// cgroups.v1
//
// This is a read-only value.
var UseV2 = safelyDetectUnifiedMode()

// Currently it is possible for the runc utility function to panic
// https://github.com/opencontainers/runc/pull/3745
func safelyDetectUnifiedMode() (result bool) {
	defer func() {
		if r := recover(); r != nil {
			result = false
		}
	}()
	result = cgroups.IsCgroup2UnifiedMode()
	return
}

// GetCgroupParent returns the mount point under the root cgroup in which Nomad
// will create cgroups. If parent is not set, an appropriate name for the version
// of cgroups will be used.
func GetCgroupParent(parent string) string {
	switch {
	case parent != "":
		return parent
	case UseV2:
		return DefaultCgroupParentV2
	default:
		return DefaultCgroupV1Parent
	}
}

// CreateCPUSetManager creates a V1 or V2 CpusetManager depending on system configuration.
func CreateCPUSetManager(parent string, reservable []uint16, logger hclog.Logger) CpusetManager {
	parent = GetCgroupParent(parent) // use appropriate default parent if not set in client config
	switch {
	case UseV2:
		return NewCpusetManagerV2(parent, reservable, logger.Named("cpuset.v2"))
	default:
		return NewCpusetManagerV1(parent, reservable, logger.Named("cpuset.v1"))
	}
}

// GetCPUsFromCgroup gets the effective cpuset value for the given cgroup.
func GetCPUsFromCgroup(group string) ([]uint16, error) {
	group = GetCgroupParent(group)
	if UseV2 {
		return getCPUsFromCgroupV2(group)
	}
	return getCPUsFromCgroupV1(group)
}

// CgroupScope returns the name of the scope for Nomad's managed cgroups for
// the given allocID and task.
//
// e.g. "<allocID>.<task>.scope"
//
// Only useful for v2.
func CgroupScope(allocID, task string) string {
	return fmt.Sprintf("%s.%s.scope", allocID, task)
}

// ConfigureBasicCgroups will initialize a cgroup and modify config to contain
// a reference to its path.
//
// v1: creates a random "freezer" cgroup which can later be used for cleanup of processes.
// v2: does nothing.
func ConfigureBasicCgroups(config *lcc.Config) error {
	if UseV2 {
		return nil
	}

	id := uuid.Generate()
	// In v1 we must setup the freezer cgroup ourselves.
	subsystem := "freezer"
	path, err := GetCgroupPathHelperV1(subsystem, filepath.Join(DefaultCgroupV1Parent, id))
	if err != nil {
		return fmt.Errorf("failed to find %s cgroup mountpoint: %v", subsystem, err)
	}
	if err = os.MkdirAll(path, 0755); err != nil {
		return err
	}
	config.Cgroups.Path = path
	return nil
}

// FindCgroupMountpointDir is used to find the cgroup mount point on a Linux
// system.
//
// Note that in cgroups.v1, this returns one of many subsystems that are mounted.
// e.g. a return value of "/sys/fs/cgroup/systemd" really implies the root is
// "/sys/fs/cgroup", which is interesting on hybrid systems where the 'unified'
// subsystem is mounted as if it were a subsystem, but the actual root is different.
// (i.e. /sys/fs/cgroup/unified).
//
// As far as Nomad is concerned, UseV2 is the source of truth for which hierarchy
// to use, and that will only be a true value if cgroups.v2 is mounted on
// /sys/fs/cgroup (i.e. system is not in v1 or hybrid mode).
//
// ➜ mount -l | grep cgroup
// tmpfs on /sys/fs/cgroup type tmpfs (ro,nosuid,nodev,noexec,mode=755,inode64)
// cgroup2 on /sys/fs/cgroup/unified type cgroup2 (rw,nosuid,nodev,noexec,relatime,nsdelegate)
// cgroup on /sys/fs/cgroup/systemd type cgroup (rw,nosuid,nodev,noexec,relatime,xattr,name=systemd)
// cgroup on /sys/fs/cgroup/memory type cgroup (rw,nosuid,nodev,noexec,relatime,memory)
// (etc.)
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

// CopyCpuset copies the cpuset.cpus value from source into destination.
func CopyCpuset(source, destination string) error {
	correct, err := cgroups.ReadFile(source, "cpuset.cpus")
	if err != nil {
		return err
	}

	err = cgroups.WriteFile(destination, "cpuset.cpus", correct)
	if err != nil {
		return err
	}

	return nil
}

// MaybeDisableMemorySwappiness will disable memory swappiness, if that controller
// is available. Always the case for cgroups v2, but is not always the case on
// very old kernels with cgroups v1.
func MaybeDisableMemorySwappiness() *uint64 {
	bypass := (*uint64)(nil)
	zero := pointer.Of[uint64](0)

	// cgroups v2 always set zero
	if UseV2 {
		return zero
	}

	// cgroups v1 detect if swappiness is supported by attempting to write to
	// the nomad parent cgroup swappiness interface
	e := &editor{fromRoot: "memory/nomad"}
	err := e.write("memory.swappiness", "0")
	if err != nil {
		return bypass
	}

	return zero
}
