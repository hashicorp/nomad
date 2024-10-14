// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package procstats

import (
	"github.com/hashicorp/go-set/v3"
	"github.com/mitchellh/go-ps"
)

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
	procs, _ := list(executorPID, ps.Processes)
	return procs
}
