// +build !linux

package fingerprint

// FindCgroupMountpointDir is used to find the cgroup mount point on a Linux
// system. Here it is a no-op implemtation
func FindCgroupMountpointDir() (string, error) {
	return "", nil
}

func (f *CGroupFingerprint) Fingerprint(*FingerprintRequest, *FingerprintResponse) error {
	return nil
}
