// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"sync"
)

var (
	// NomadCgroupParent is a global variable because setting this value
	// from the Nomad client initialization is much less painful than trying to
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

var (
	mode      Mode
	detection sync.Once
)

// GetMode returns the cgroups Mode of operation.
func GetMode() Mode {
	detection.Do(func() {
		mode = detect()
	})
	return mode
}
