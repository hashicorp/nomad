//go:build linux

package fingerprint

import (
	"fmt"
)

const (
	cgroupAvailable = "available"
)

// Fingerprint tries to find a valid cgroup mount point and the version of cgroups
// if a mount-point is present.
func (f *CGroupFingerprint) Fingerprint(req *FingerprintRequest, resp *FingerprintResponse) error {
	mount, err := f.mountPointDetector.MountPoint()
	if err != nil {
		f.clearCGroupAttributes(resp)
		return fmt.Errorf("failed to discover cgroup mount point: %s", err)
	}

	// Check if a cgroup mount point was found.
	if mount == "" {
		f.clearCGroupAttributes(resp)
		if f.lastState == cgroupAvailable {
			f.logger.Warn("cgroups are now unavailable")
		}
		f.lastState = cgroupUnavailable
		return nil
	}

	// Check the version in use.
	version := f.versionDetector.CgroupVersion()

	resp.AddAttribute(cgroupMountPointAttribute, mount)
	resp.AddAttribute(cgroupVersionAttribute, version)
	resp.Detected = true

	if f.lastState == cgroupUnavailable {
		f.logger.Info("cgroups are available")
	}
	f.lastState = cgroupAvailable
	return nil
}
