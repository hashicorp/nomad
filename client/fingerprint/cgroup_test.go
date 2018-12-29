// +build linux

package fingerprint

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper/testlog"
	"github.com/hashicorp/nomad/nomad/structs"
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

func TestCGroupFingerprint(t *testing.T) {
	{
		f := &CGroupFingerprint{
			logger:             testlog.HCLogger(t),
			lastState:          cgroupUnavailable,
			mountPointDetector: &MountPointDetectorMountPointFail{},
		}

		node := &structs.Node{
			Attributes: make(map[string]string),
		}

		request := &FingerprintRequest{Config: &config.Config{}, Node: node}
		var response FingerprintResponse
		err := f.Fingerprint(request, &response)
		if err == nil {
			t.Fatalf("expected an error")
		}

		if a, _ := response.Attributes["unique.cgroup.mountpoint"]; a != "" {
			t.Fatalf("unexpected attribute found, %s", a)
		}
	}

	{
		f := &CGroupFingerprint{
			logger:             testlog.HCLogger(t),
			lastState:          cgroupUnavailable,
			mountPointDetector: &MountPointDetectorValidMountPoint{},
		}

		node := &structs.Node{
			Attributes: make(map[string]string),
		}

		request := &FingerprintRequest{Config: &config.Config{}, Node: node}
		var response FingerprintResponse
		err := f.Fingerprint(request, &response)
		if err != nil {
			t.Fatalf("unexpected error, %s", err)
		}
		if a, ok := response.Attributes["unique.cgroup.mountpoint"]; !ok {
			t.Fatalf("unable to find attribute: %s", a)
		}
	}

	{
		f := &CGroupFingerprint{
			logger:             testlog.HCLogger(t),
			lastState:          cgroupUnavailable,
			mountPointDetector: &MountPointDetectorEmptyMountPoint{},
		}

		node := &structs.Node{
			Attributes: make(map[string]string),
		}

		request := &FingerprintRequest{Config: &config.Config{}, Node: node}
		var response FingerprintResponse
		err := f.Fingerprint(request, &response)
		if err != nil {
			t.Fatalf("unexpected error, %s", err)
		}
		if a, _ := response.Attributes["unique.cgroup.mountpoint"]; a != "" {
			t.Fatalf("unexpected attribute found, %s", a)
		}
	}
	{
		f := &CGroupFingerprint{
			logger:             testlog.HCLogger(t),
			lastState:          cgroupAvailable,
			mountPointDetector: &MountPointDetectorValidMountPoint{},
		}

		node := &structs.Node{
			Attributes: make(map[string]string),
		}

		request := &FingerprintRequest{Config: &config.Config{}, Node: node}
		var response FingerprintResponse
		err := f.Fingerprint(request, &response)
		if err != nil {
			t.Fatalf("unexpected error, %s", err)
		}
		if a, _ := response.Attributes["unique.cgroup.mountpoint"]; a == "" {
			t.Fatalf("expected attribute to be found, %s", a)
		}
	}
}
