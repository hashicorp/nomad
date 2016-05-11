// +build !linux

package fingerprint

import (
	client "github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/nomad/structs"
)

// FindCgroupMountpointDir returns an empty path on non-Linux systems
func FindCgroupMountpointDir() (string, error) {
	return "", nil
}

// Fingerprint tries to find a valid cgroup moint point
func (f *CGroupFingerprint) Fingerprint(cfg *client.Config, node *structs.Node) (bool, error) {
	return false, nil
}
