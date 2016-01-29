// +build !linux

package fingerprint

// FindCgroupMountpointDir returns an empty path on non-Linux systems
func FindCgroupMountpointDir() (string, error) {
	return "", nil
}
