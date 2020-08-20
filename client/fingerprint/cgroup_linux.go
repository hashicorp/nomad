// +build linux

package fingerprint

import (
	"fmt"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

const (
	cgroupAvailable = "available"
)

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

// Fingerprint tries to find a valid cgroup mount point
func (f *CGroupFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	mount, err := f.mountPointDetector.MountPoint()
	if err != nil {
		f.clearCGroupAttributes(resp)
		return fmt.Errorf("Failed to discover cgroup mount point: %s", err)
	}

	// Check if a cgroup mount point was found
	if mount == "" {

		f.clearCGroupAttributes(resp)

		if f.lastState == cgroupAvailable {
			f.logger.Info("cgroups are unavailable")
		}
		f.lastState = cgroupUnavailable
		return nil
	}

	resp.AddAttribute("unique.cgroup.mountpoint", mount)
	resp.Detected = true

	if f.lastState == cgroupUnavailable {
		f.logger.Info("cgroups are available")
	}
	f.lastState = cgroupAvailable
	return nil
}
