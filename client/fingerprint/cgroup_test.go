// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package fingerprint

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/require"
)

// A fake mount point detector that returns an empty path
type MountPointDetectorNoMountPoint struct{}

func (m *MountPointDetectorNoMountPoint) MountPoint() (string, error) {
	return "", nil
}

// A fake mount point detector that returns an error
type MountPointDetectorMountPointFail struct{}

func (m *MountPointDetectorMountPointFail) MountPoint() (string, error) {
	return "", fmt.Errorf("cgroup mountpoint discovery failed")
}

// A fake mount point detector that returns a valid path
type MountPointDetectorValidMountPoint struct{}

func (m *MountPointDetectorValidMountPoint) MountPoint() (string, error) {
	return "/sys/fs/cgroup", nil
}

// A fake mount point detector that returns an empty path
type MountPointDetectorEmptyMountPoint struct{}

func (m *MountPointDetectorEmptyMountPoint) MountPoint() (string, error) {
	return "", nil
}

// A fake version detector that returns the set version.
type FakeVersionDetector struct {
	version string
}

func (f *FakeVersionDetector) CgroupVersion() string {
	return f.version
}

func newRequest(node *structs.Node) *FingerprintRequest {
	return &FingerprintRequest{
		Config: new(config.Config),
		Node:   node,
	}
}

func newNode() *structs.Node {
	return &structs.Node{
		Attributes: make(map[string]string),
	}
}

func TestCgroup_MountPoint(t *testing.T) {
	ci.Parallel(t)

	t.Run("mount-point fail", func(t *testing.T) {
		f := &CGroupFingerprint{
			logger:             testlog.HCLogger(t),
			lastState:          cgroupUnavailable,
			mountPointDetector: new(MountPointDetectorMountPointFail),
			versionDetector:    new(DefaultCgroupVersionDetector),
		}

		request := newRequest(newNode())
		var response FingerprintResponse
		err := f.Fingerprint(request, &response)
		require.EqualError(t, err, "failed to discover cgroup mount point: cgroup mountpoint discovery failed")
		require.Empty(t, response.Attributes[cgroupMountPointAttribute])
	})

	t.Run("mount-point available", func(t *testing.T) {
		f := &CGroupFingerprint{
			logger:             testlog.HCLogger(t),
			lastState:          cgroupUnavailable,
			mountPointDetector: new(MountPointDetectorValidMountPoint),
			versionDetector:    new(DefaultCgroupVersionDetector),
		}

		request := newRequest(newNode())
		var response FingerprintResponse
		err := f.Fingerprint(request, &response)
		require.NoError(t, err)
		require.Equal(t, "/sys/fs/cgroup", response.Attributes[cgroupMountPointAttribute])
	})

	t.Run("mount-point empty", func(t *testing.T) {
		f := &CGroupFingerprint{
			logger:             testlog.HCLogger(t),
			lastState:          cgroupUnavailable,
			mountPointDetector: new(MountPointDetectorEmptyMountPoint),
			versionDetector:    new(DefaultCgroupVersionDetector),
		}

		var response FingerprintResponse
		err := f.Fingerprint(newRequest(newNode()), &response)
		require.NoError(t, err)
		require.Empty(t, response.Attributes[cgroupMountPointAttribute])
	})

	t.Run("mount-point already present", func(t *testing.T) {
		f := &CGroupFingerprint{
			logger:             testlog.HCLogger(t),
			lastState:          cgroupAvailable,
			mountPointDetector: new(MountPointDetectorValidMountPoint),
			versionDetector:    new(DefaultCgroupVersionDetector),
		}

		var response FingerprintResponse
		err := f.Fingerprint(newRequest(newNode()), &response)
		require.NoError(t, err)
		require.Equal(t, "/sys/fs/cgroup", response.Attributes[cgroupMountPointAttribute])
	})
}

func TestCgroup_Version(t *testing.T) {
	t.Run("version v1", func(t *testing.T) {
		f := &CGroupFingerprint{
			logger:             testlog.HCLogger(t),
			lastState:          cgroupUnavailable,
			mountPointDetector: new(MountPointDetectorValidMountPoint),
			versionDetector:    &FakeVersionDetector{version: "v1"},
		}

		var response FingerprintResponse
		err := f.Fingerprint(newRequest(newNode()), &response)
		require.NoError(t, err)
		require.Equal(t, "v1", response.Attributes[cgroupVersionAttribute])
	})

	t.Run("without mount-point", func(t *testing.T) {
		f := &CGroupFingerprint{
			logger:             testlog.HCLogger(t),
			lastState:          cgroupUnavailable,
			mountPointDetector: new(MountPointDetectorEmptyMountPoint),
			versionDetector:    &FakeVersionDetector{version: "v1"},
		}

		var response FingerprintResponse
		err := f.Fingerprint(newRequest(newNode()), &response)
		require.NoError(t, err)
		require.Empty(t, response.Attributes[cgroupMountPointAttribute])
	})
}
