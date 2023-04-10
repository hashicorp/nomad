// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cgutil

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/hashicorp/nomad/lib/cpuset"
	"github.com/hashicorp/nomad/nomad/structs"
)

const (
	// CgroupRoot is hard-coded in the cgroups specification.
	// It only applies to linux but helpers have references to it in driver(s).
	CgroupRoot = "/sys/fs/cgroup"
)

// CpusetManager is used to setup cpuset cgroups for each task.
type CpusetManager interface {
	// Init should be called before the client starts running allocations. This
	// is where the cpuset manager should start doing background operations.
	Init()

	// AddAlloc adds an allocation to the manager
	AddAlloc(alloc *structs.Allocation)

	// RemoveAlloc removes an alloc by ID from the manager
	RemoveAlloc(allocID string)

	// CgroupPathFor returns a callback for getting the cgroup path and any error that may have occurred during
	// cgroup initialization. The callback will block if the cgroup has not been created
	CgroupPathFor(allocID, taskName string) CgroupPathGetter
}

type NoopCpusetManager struct{}

func (n NoopCpusetManager) Init() {
}

func (n NoopCpusetManager) AddAlloc(alloc *structs.Allocation) {
}

func (n NoopCpusetManager) RemoveAlloc(allocID string) {
}

func (n NoopCpusetManager) CgroupPathFor(allocID, task string) CgroupPathGetter {
	return func(context.Context) (string, error) { return "", nil }
}

// CgroupPathGetter is a function which returns the cgroup path and any error which
// occurred during cgroup initialization.
//
// It should block until the cgroup has been created or an error is reported.
type CgroupPathGetter func(context.Context) (path string, err error)

type TaskCgroupInfo struct {
	CgroupPath         string
	RelativeCgroupPath string
	Cpuset             cpuset.CPUSet
	Error              error
}

// SplitPath determines the parent and cgroup from p.
// p must contain at least 2 elements (parent + cgroup).
//
// Handles the cgroup root if present.
func SplitPath(p string) (string, string) {
	p = strings.TrimPrefix(p, CgroupRoot)
	p = strings.Trim(p, "/")
	parts := strings.Split(p, "/")
	return parts[0], "/" + filepath.Join(parts[1:]...)
}
