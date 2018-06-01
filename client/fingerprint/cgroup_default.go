// +build !linux

package fingerprint

import cstructs "github.com/hashicorp/nomad/client/structs"

// FindCgroupMountpointDir is used to find the cgroup mount point on a Linux
// system. Here it is a no-op implemtation
func FindCgroupMountpointDir() (string, error) {
	return "", nil
}

func (f *CGroupFingerprint) Fingerprint(*cstructs.FingerprintRequest, *cstructs.FingerprintResponse) error {
	return nil
}
