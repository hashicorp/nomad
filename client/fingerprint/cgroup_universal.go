// +build !linux

package fingerprint

// cgroups only exist on Linux
func FindCgroupMountpointDir() (string, error) {
	return "", nil
}
