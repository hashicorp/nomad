//go:build linux

package cgroupslib

// IsV2 returns true if the system is using cgroups v2 (unified heirarchy),
// or otherwise false.
func IsV2() bool {
	return true
}
