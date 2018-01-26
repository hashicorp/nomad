// +build linux

package fingerprint

import (
	"log"
	"time"

	cstructs "github.com/hashicorp/nomad/client/structs"
)

const (
	cgroupAvailable   = "available"
	cgroupUnavailable = "unavailable"
	interval          = 15
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

// NewCGroupFingerprint returns a new cgroup fingerprinter
func NewCGroupFingerprint(logger *log.Logger) Fingerprint {
	f := &CGroupFingerprint{
		logger:             logger,
		lastState:          cgroupUnavailable,
		mountPointDetector: &DefaultMountPointDetector{},
	}
	return f
}

// clearCGroupAttributes clears any node attributes related to cgroups that might
// have been set in a previous fingerprint run.
func (f *CGroupFingerprint) clearCGroupAttributes(r *cstructs.FingerprintResponse) {
	r.RemoveAttribute("unique.cgroup.mountpoint")
}

// Periodic determines the interval at which the periodic fingerprinter will run.
func (f *CGroupFingerprint) Periodic() (bool, time.Duration) {
	return true, interval * time.Second
}
