package cgroupslib

import (
	"github.com/hashicorp/nomad/helper/pointer"
)

// MaybeDisableMemorySwappiness will disable memory swappiness, if that controller
// is available. Always the case for cgroups v2, but is not always the case on
// very old kernels with cgroups v1.
func MaybeDisableMemorySwappiness() *uint64 {
	switch GetMode() {
	case CG2:
		return pointer.Of[uint64](0)
	default:
		panic("todo")

		// cgroups v1 detect if swappiness is supported by attempting to write to
		// the nomad parent cgroup swappiness interface
		// e := &editor{fromRoot: "memory/nomad"}
		// err := e.write("memory.swappiness", "0")
		// if err != nil {
		// 	return bypass
		// }
		// return zero
	}
}
