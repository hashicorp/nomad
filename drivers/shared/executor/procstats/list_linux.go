// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package procstats

import (
	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
)

type Cgrouper interface {
	Cgroup() string
}

func List(cg Cgrouper) *set.Set[ProcessID] {
	cgroup := cg.Cgroup()
	var ed cgroupslib.Interface
	switch cgroupslib.GetMode() {
	case cgroupslib.CG1:
		ed = cgroupslib.OpenFromCpusetCG1(cgroup, "freezer")
	default:
		ed = cgroupslib.OpenPath(cgroup)
	}

	s, err := ed.PIDs()
	if err != nil {
		return set.New[ProcessID](0)
	}
	return s
}
