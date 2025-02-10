// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package procstats

import (
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/client/lib/cgroupslib"
)

type Cgrouper interface {
	StatsCgroup() string
}

func List(cg Cgrouper) *set.Set[ProcessID] {
	cgroup := cg.StatsCgroup()
	ed := cgroupslib.OpenPath(cgroup)
	s, err := ed.PIDs()
	if err != nil {
		return set.New[ProcessID](0)
	}
	return s
}
