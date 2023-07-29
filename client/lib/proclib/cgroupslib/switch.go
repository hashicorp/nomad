//go:build linux

package cgroupslib

import (
	"sync"
)

var (
	// NomadCgroupParent is a global variable because trust me, setting this
	// from the Nomad client initalization is much less painful than trying to
	// plumb it through in every place we need to reference it. This value will
	// be written to only once, during init, and after that it's only reads.
	NomadCgroupParent = defaultParent()
)

func defaultParent() string {
	switch GetMode() {
	case CG1:
		return "/nomad"
	default:
		return "nomad.slice"
	}
}

type Mode byte

const (
	CG1 = iota
	CG2
	OFF
)

var (
	mode      Mode
	detection sync.Once
)

// GetMode returns the cgroups mode of operation.
func GetMode() Mode {
	detection.Do(func() {
		mode = detect()
	})
	return mode
}
