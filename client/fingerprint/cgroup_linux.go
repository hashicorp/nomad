// +build linux

package fingerprint

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func FindCgroupMountpointDir() (string, error) {
	mount, err := cgroups.FindCgroupMountpointDir()
	if err != nil {
		switch e := err.(type) {
		case *cgroups.NotFoundError:
			// It's okay if the mount point is not discovered
			return "", nil
		default:
			// All other errors are passed back as is
			return "", e
		}
	}
	return mount, nil
}
