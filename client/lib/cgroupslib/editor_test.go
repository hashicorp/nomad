// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"testing"

	"github.com/shoenig/test/must"
)

func Test_OpenFromCpusetCG1(t *testing.T) {
	input := "/sys/fs/cgroup/cpuset/nomad/reserve/task"
	iface := "memory"
	i := OpenFromCpusetCG1(input, iface)

	exp := "/sys/fs/cgroup/memory/nomad/task"
	must.Eq(t, exp, i.(*editor).dpath)
}

func Test_OpenFromCpusetCG1_cpuset(t *testing.T) {
	input := "/sys/fs/cgroup/cpuset/nomad/reserve/task"
	iface := "cpuset"
	i := OpenFromCpusetCG1(input, iface)

	exp := "/sys/fs/cgroup/cpuset/nomad/reserve/task"
	must.Eq(t, exp, i.(*editor).dpath)
}
