// +build linux

package fingerprint

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/client/config"
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
	f := &CGroupFingerprint{
		logger:             testLogger(),
		lastState:          cgroupUnavailable,
		mountPointDetector: &MountPointDetectorMountPointFail{},
	}

	node := &structs.Node{
		Attributes: make(map[string]string),
	}

	ok, err := f.Fingerprint(&config.Config{}, node)
	if err == nil {
		t.Fatalf("expected an error")
	}
	if ok {
		t.Fatalf("should not apply")
	}
	if a, ok := node.Attributes["unique.cgroup.mountpoint"]; ok {
		t.Fatalf("unexpected attribute found, %s", a)
	}

	f = &CGroupFingerprint{
		logger:             testLogger(),
		lastState:          cgroupUnavailable,
		mountPointDetector: &MountPointDetectorValidMountPoint{},
	}

	node = &structs.Node{
		Attributes: make(map[string]string),
	}

	ok, err = f.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("unexpected error, %s", err)
	}
	if !ok {
		t.Fatalf("should apply")
	}
	assertNodeAttributeContains(t, node, "unique.cgroup.mountpoint")

	f = &CGroupFingerprint{
		logger:             testLogger(),
		lastState:          cgroupUnavailable,
		mountPointDetector: &MountPointDetectorEmptyMountPoint{},
	}

	node = &structs.Node{
		Attributes: make(map[string]string),
	}

	ok, err = f.Fingerprint(&config.Config{}, node)
	if err != nil {
		t.Fatalf("unexpected error, %s", err)
	}
	if !ok {
		t.Fatalf("should apply")
	}
	if a, ok := node.Attributes["unique.cgroup.mountpoint"]; ok {
		t.Fatalf("unexpected attribute found, %s", a)
	}
}
