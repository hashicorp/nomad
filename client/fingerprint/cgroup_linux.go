// +build linux

package fingerprint

import (
	"fmt"

	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

// FindCgroupMountpointDir is used to find the cgroup mount point on a Linux
// system.
func FindCgroupMountpointDir() (string, error) {
	mount, err := cgroups.FindCgroupMountpointDir()
	if err != nil {
		switch e := err.(type) {
		case *cgroups.NotFoundError:
			// It's okay if the mount point is not discovered
			return "", nil
		default:
			// All other errors are passed back as is
			return "", e
		}
	}
	return mount, nil
}

// Fingerprint tries to find a valid cgroup mount point
func (f *CGroupFingerprint) Fingerprint(req *cstructs.FingerprintRequest, resp *cstructs.FingerprintResponse) error {
	mount, err := f.mountPointDetector.MountPoint()
	if err != nil {
		f.clearCGroupAttributes(resp)
		return fmt.Errorf("Failed to discover cgroup mount point: %s", err)
	}

	// Check if a cgroup mount point was found
	if mount == "" {

		f.clearCGroupAttributes(resp)

		if f.lastState == cgroupAvailable {
			f.logger.Printf("[INFO] fingerprint.cgroups: cgroups are unavailable")
		}
		f.lastState = cgroupUnavailable
		return nil
	}

	resp.AddAttribute("unique.cgroup.mountpoint", mount)
	resp.Detected = true

	if f.lastState == cgroupUnavailable {
		f.logger.Printf("[INFO] fingerprint.cgroups: cgroups are available")
	}
	f.lastState = cgroupAvailable
	return nil
}
