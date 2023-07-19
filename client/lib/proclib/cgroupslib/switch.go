//go:build linux

package cgroupslib

type Mode byte

const (
	CG1 = iota
	CG2
	OFF
)

// GetMode returns the cgroups mode of operation.
func GetMode() Mode {
	return CG2
}
