// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package procstats

import (
	"github.com/hashicorp/go-set/v2"
	"github.com/mitchellh/go-ps"
)

func gather(procs map[int]ps.Process, family set.Collection[int], root int, candidate ps.Process) bool {
	if candidate == nil {
		return false
	}
	pid := candidate.Pid()
	if pid == 0 || pid == 1 {
		return false
	}
	if pid == root {
		return true
	}
	parent := procs[candidate.PPid()]
	result := gather(procs, family, root, parent)
	if result {
		family.Insert(pid)
	}
	return result
}

func mapping(all []ps.Process) map[int]ps.Process {
	result := make(map[int]ps.Process)
	for _, process := range all {
		result[process.Pid()] = process
	}
	return result
}

func list(executorPID int, processes func() ([]ps.Process, error)) set.Collection[ProcessID] {
	family := set.From([]int{executorPID})

	all, err := processes()
	if err != nil {
		return set.New[ProcessID](0)
	}

	m := mapping(all)
	for _, candidate := range all {
		gather(m, family, executorPID, candidate)
	}

	return family
}

// List will scan the process table and return a set of the process family
// tree starting with executorPID as the root.
//
// The implementation here specifically avoids using more than one system
// call. Unlike on Linux where we just read a cgroup, on Windows we must build
// the tree manually. We do so knowing only the child->parent relationships.
//
// So this turns into a fun leet code problem, where we invert the tree using
// only a bucket of edges pointing in the wrong direction. Basically we just
// iterate every process, recursively follow its parent, and determine whether
// executorPID is an ancestor.
//
// See https://github.com/hashicorp/nomad/issues/20042 as an example of what
// happens when you use syscalls to work your way from the root down to its
// descendants.
func List(executorPID int) set.Collection[ProcessID] {
	return list(executorPID, ps.Processes)
}
