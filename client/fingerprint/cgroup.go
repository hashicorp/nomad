package fingerprint

import (
	"fmt"
	"log"
	"time"

	client "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	cgroupAvailable   = "available"
	cgroupUnavailable = "unavailable"
)

type CGroupFingerprint struct {
	logger             *log.Logger
	lastState          string
	mountPointDetector MountPointDetector
}

// An interface to isolate calls to the cgroup library
// This facilitates testing where we can implement
// fake mount points to test various code paths
type MountPointDetector interface {
	MountPoint() (string, error)
}

// Implements the interface detector which calls the cgroups library directly
type DefaultMountPointDetector struct {
}

// Call out to the default cgroup library
func (b *DefaultMountPointDetector) MountPoint() (string, error) {
	return FindCgroupMountpointDir()
}

func NewCGroupFingerprint(logger *log.Logger) Fingerprint {
	f := &CGroupFingerprint{
		logger:             logger,
		lastState:          cgroupUnavailable,
		mountPointDetector: &DefaultMountPointDetector{},
	}
	return f
}

func (f *CGroupFingerprint) Fingerprint(cfg *client.Config, node *structs.Node) (bool, error) {
	// Try to find a valid cgroup mount point
	mount, err := f.mountPointDetector.MountPoint()
	if err != nil {
		return false, fmt.Errorf("Failed to discover cgroup mount point: %s", err)
	}

	// Check if a cgroup mount point was found
	if mount == "" {
		// Clear any attributes from the previous fingerprint.
		f.clearCGroupAttributes(node)

		if f.lastState == cgroupAvailable {
			f.logger.Printf("[INFO] fingerprint.cgroups: cgroups are unavailable")
		}
		f.lastState = cgroupUnavailable
		return true, nil
	}

	node.Attributes["cgroup.mountpoint"] = mount

	if f.lastState == cgroupUnavailable {
		f.logger.Printf("[INFO] fingerprint.cgroups: cgroups are available")
	}
	f.lastState = cgroupAvailable
	return true, nil
}

func (f *CGroupFingerprint) clearCGroupAttributes(n *structs.Node) {
	delete(n.Attributes, "cgroup.mountpoint")
}

func (f *CGroupFingerprint) Periodic() (bool, time.Duration) {
	return true, 15 * time.Second
}
