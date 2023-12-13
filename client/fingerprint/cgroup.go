// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/cgutil"
)

const (
	cgroupUnavailable = "unavailable" // "available" is over in cgroup_linux

	cgroupMountPointAttribute = "unique.cgroup.mountpoint"
	cgroupVersionAttribute    = "unique.cgroup.version"

	cgroupDetectInterval = 15 * time.Second
)

type CGroupFingerprint struct {
	logger             hclog.Logger
	lastState          string
	mountPointDetector MountPointDetector
	versionDetector    CgroupVersionDetector
}

// MountPointDetector isolates calls to the cgroup library.
//
// This facilitates testing where we can implement fake mount points to test
// various code paths.
type MountPointDetector interface {
	// MountPoint returns a cgroup mount-point.
	//
	// In v1, this is one arbitrary subsystem (e.g. /sys/fs/cgroup/cpu).
	//
	// In v2, this is the actual root mount point (i.e. /sys/fs/cgroup).
	MountPoint() (string, error)
}

// DefaultMountPointDetector implements the interface detector which calls the cgroups
// library directly
type DefaultMountPointDetector struct {
}

// MountPoint calls out to the default cgroup library.
func (*DefaultMountPointDetector) MountPoint() (string, error) {
	return cgutil.FindCgroupMountpointDir()
}

// CgroupVersionDetector isolates calls to the cgroup library.
type CgroupVersionDetector interface {
	// CgroupVersion returns v1 or v2 depending on the cgroups version in use.
	CgroupVersion() string
}

// DefaultCgroupVersionDetector implements the version detector which calls the
// cgroups library directly.
type DefaultCgroupVersionDetector struct {
}

func (*DefaultCgroupVersionDetector) CgroupVersion() string {
	if cgutil.UseV2 {
		return "v2"
	}
	return "v1"
}

// NewCGroupFingerprint returns a new cgroup fingerprinter
func NewCGroupFingerprint(logger hclog.Logger) Fingerprint {
	return &CGroupFingerprint{
		logger:             logger.Named("cgroup"),
		lastState:          cgroupUnavailable,
		mountPointDetector: new(DefaultMountPointDetector),
		versionDetector:    new(DefaultCgroupVersionDetector),
	}
}

// clearCGroupAttributes clears any node attributes related to cgroups that might
// have been set in a previous fingerprint run.
func (f *CGroupFingerprint) clearCGroupAttributes(r *FingerprintResponse) {
	r.RemoveAttribute(cgroupMountPointAttribute)
	r.RemoveAttribute(cgroupVersionAttribute)
}

// Periodic determines the interval at which the periodic fingerprinter will run.
func (f *CGroupFingerprint) Periodic() (bool, time.Duration) {
	return true, cgroupDetectInterval
}
