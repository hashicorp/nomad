//go:build linux

package cgroupslib

import (
	"github.com/hashicorp/nomad/helper/pointer"
)

// MaybeDisableMemorySwappiness will disable memory swappiness, if that controller
// is available. Always the case for cgroups v2, but is not always the case on
// very old kernels with cgroups v1.
func MaybeDisableMemorySwappiness() *uint64 {
	switch GetMode() {
	case CG1:
		err := WriteNomadCG1("memory", "memory.swappiness", "0")
		if err == nil {
			return pointer.Of[uint64](0)
		}
		return nil
	default:
		return pointer.Of[uint64](0)
	}
}
