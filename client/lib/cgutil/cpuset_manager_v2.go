// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/lib/cpuset"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fs2"
	"github.com/opencontainers/runc/libcontainer/configs"
)

const (
	// CreationPID is a special PID in libcontainer used to denote a cgroup
	// should be created, but with no process added.
	//
	// https://github.com/opencontainers/runc/blob/v1.0.3/libcontainer/cgroups/utils.go#L372
	CreationPID = -1

	// DefaultCgroupParentV2 is the name of Nomad's default parent cgroup, under which
	// all other cgroups are managed. This can be changed with client configuration
	// in case for e.g. Nomad tasks should be further constrained by an externally
	// configured systemd cgroup.
	DefaultCgroupParentV2 = "nomad.slice"
)

// nothing is used for treating a map like a set with no values
type nothing struct{}

// present indicates something exists
var present = nothing{}

type cpusetManagerV2 struct {
	logger hclog.Logger

	parent    string        // relative to cgroup root (e.g. "nomad.slice")
	parentAbs string        // absolute path (e.g. "/sys/fs/cgroup/nomad.slice")
	initial   cpuset.CPUSet // set of initial cores (never changes)

	lock      sync.Mutex                 // hold this when managing pool / sharing / isolating
	pool      cpuset.CPUSet              // pool of cores being shared among all tasks
	sharing   map[identity]nothing       // sharing tasks using cores only from the pool
	isolating map[identity]cpuset.CPUSet // isolating tasks using cores from the pool + reserved cores
}

func NewCpusetManagerV2(parent string, reservable []uint16, logger hclog.Logger) CpusetManager {
	if err := minimumRootControllers(); err != nil {
		logger.Error("failed to enabled minimum set of cgroup controllers; disabling cpuset management", "error", err)
		return new(NoopCpusetManager)
	}

	parentAbs := filepath.Join(CgroupRoot, parent)
	if err := os.MkdirAll(parentAbs, 0o755); err != nil {
		logger.Error("failed to ensure nomad parent cgroup exists; disabling cpuset management", "error", err)
		return new(NoopCpusetManager)
	}

	if len(reservable) == 0 {
		// read from group
		if cpus, err := GetCPUsFromCgroup(parent); err != nil {
			logger.Error("failed to lookup cpus from parent cgroup; disabling cpuset management", "error", err)
			return new(NoopCpusetManager)
		} else {
			reservable = cpus
		}
	}

	return &cpusetManagerV2{
		initial:   cpuset.New(reservable...),
		parent:    parent,
		parentAbs: parentAbs,
		logger:    logger,
		sharing:   make(map[identity]nothing),
		isolating: make(map[identity]cpuset.CPUSet),
	}
}

// minimumControllers sets the minimum set of required controllers on the
// /sys/fs/cgroup/cgroup.subtree_control file - ensuring [cpuset, cpu, io, memory, pids]
// are enabled.
func minimumRootControllers() error {
	e := new(editor)
	s, err := e.read("cgroup.subtree_control")
	if err != nil {
		return err
	}

	required := set.From[string]([]string{"cpuset", "cpu", "io", "memory", "pids"})
	enabled := set.From[string](strings.Fields(s))
	needed := required.Difference(enabled)

	if needed.Size() == 0 {
		return nil // already sufficient
	}

	sb := new(strings.Builder)
	for _, controller := range needed.List() {
		sb.WriteString("+" + controller + " ")
	}

	activation := strings.TrimSpace(sb.String())
	return e.write("cgroup.subtree_control", activation)
}

func (c *cpusetManagerV2) Init() {
	c.logger.Debug("initializing with", "cores", c.initial)
}

func (c *cpusetManagerV2) AddAlloc(alloc *structs.Allocation) {
	if alloc == nil || alloc.AllocatedResources == nil {
		return
	}
	c.logger.Trace("add allocation", "name", alloc.Name, "id", alloc.ID)

	// grab write lock while we recompute and apply changes
	c.lock.Lock()
	defer c.lock.Unlock()

	// first update our tracking of isolating and sharing tasks
	for task, resources := range alloc.AllocatedResources.Tasks {
		id := makeID(alloc.ID, task)
		if len(resources.Cpu.ReservedCores) > 0 {
			c.isolating[id] = cpuset.New(resources.Cpu.ReservedCores...)
		} else {
			c.sharing[id] = present
		}
	}

	// recompute the available sharable cpu cores
	c.recalculate()

	// now write out the entire cgroups space
	c.reconcile()

	// no need to cleanup on adds, we did not remove a task
}

func (c *cpusetManagerV2) RemoveAlloc(allocID string) {
	c.logger.Trace("remove allocation", "id", allocID)

	// grab write lock while we recompute and apply changes.
	c.lock.Lock()
	defer c.lock.Unlock()

	// remove tasks of allocID from the sharing set
	for id := range c.sharing {
		if strings.HasPrefix(string(id), allocID) {
			delete(c.sharing, id)
		}
	}

	// remove tasks of allocID from the isolating set
	for id := range c.isolating {
		if strings.HasPrefix(string(id), allocID) {
			delete(c.isolating, id)
		}
	}

	// recompute available sharable cpu cores
	c.recalculate()

	// now write out the entire cgroups space
	c.reconcile()

	// now remove any tasks no longer running
	c.cleanup()
}

func (c *cpusetManagerV2) CgroupPathFor(allocID, task string) CgroupPathGetter {
	// The CgroupPathFor implementation must block until cgroup for allocID.task
	// exists [and can accept a PID].
	return func(ctx context.Context) (string, error) {
		ticks, cancel := helper.NewSafeTimer(100 * time.Millisecond)
		defer cancel()

		for {
			path := c.pathOf(makeID(allocID, task))
			mgr, err := fs2.NewManager(nil, path)
			if err != nil {
				return "", err
			}

			if mgr.Exists() {
				return path, nil
			}

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-ticks.C:
				continue
			}
		}
	}
}

// recalculate the number of cores sharable by non-isolating tasks (and isolating tasks)
//
// must be called while holding c.lock
func (c *cpusetManagerV2) recalculate() {
	remaining := c.initial.Copy()
	for _, set := range c.isolating {
		remaining = remaining.Difference(set)
	}
	c.pool = remaining
}

// reconcile will actually write the cpuset values for all tracked tasks.
//
// must be called while holding c.lock
func (c *cpusetManagerV2) reconcile() {
	for id := range c.sharing {
		c.write(id, c.pool)
	}

	for id, set := range c.isolating {
		c.write(id, c.pool.Union(set))
	}
}

// cleanup will remove any cgroups for allocations no longer being tracked
//
// must be called while holding c.lock
func (c *cpusetManagerV2) cleanup() {
	// create a map to lookup ids we know about
	size := len(c.sharing) + len(c.isolating)
	ids := make(map[identity]nothing, size)
	for id := range c.sharing {
		ids[id] = present
	}
	for id := range c.isolating {
		ids[id] = present
	}

	if err := filepath.WalkDir(c.parentAbs, func(path string, entry os.DirEntry, err error) error {
		// skip anything we cannot read
		if err != nil {
			return nil
		}

		// a cgroup is a directory
		if !entry.IsDir() {
			return nil
		}

		dir := filepath.Dir(path)
		base := filepath.Base(path)

		// only manage scopes directly under nomad.slice
		if dir != c.parentAbs || !strings.HasSuffix(base, ".scope") {
			return nil
		}

		// only remove the scope if we do not track it
		id := identity(strings.TrimSuffix(base, ".scope"))
		_, exists := ids[id]
		if !exists {
			c.remove(path)
		}

		return nil
	}); err != nil {
		c.logger.Error("failed to cleanup cgroup", "error", err)
	}
}

// pathOf returns the absolute path to a task with identity id.
func (c *cpusetManagerV2) pathOf(id identity) string {
	return filepath.Join(c.parentAbs, makeScope(id))
}

// remove does the actual fs delete of the cgroup
//
// We avoid removing a cgroup if it still contains a PID, as the cpuset manager
// may be initially empty on a Nomad client restart.
func (c *cpusetManagerV2) remove(path string) {
	mgr, err := fs2.NewManager(nil, path)
	if err != nil {
		c.logger.Warn("failed to create manager", "path", path, "error", err)
		return
	}

	// get the list of pids managed by this scope (should be 0 or 1)
	pids, _ := mgr.GetPids()

	// do not destroy the scope if a PID is still present
	// this is a normal condition when an agent restarts with running tasks
	// and the v2 manager is still rebuilding its tracked tasks
	if len(pids) > 0 {
		return
	}

	// remove the cgroup
	if err3 := mgr.Destroy(); err3 != nil {
		c.logger.Warn("failed to cleanup cgroup", "path", path, "error", err)
		return
	}
}

// write does the actual write of cpuset set for cgroup id
func (c *cpusetManagerV2) write(id identity, set cpuset.CPUSet) {
	path := c.pathOf(id)

	// make a manager for the cgroup
	m, err := fs2.NewManager(new(configs.Cgroup), path)
	if err != nil {
		c.logger.Error("failed to manage cgroup", "path", path, "error", err)
		return
	}

	// create the cgroup
	if err = m.Apply(CreationPID); err != nil {
		c.logger.Error("failed to apply cgroup", "path", path, "error", err)
		return
	}

	// set the cpuset value for the cgroup
	if err = m.Set(&configs.Resources{
		CpusetCpus:  set.String(),
		SkipDevices: true,
	}); err != nil {
		c.logger.Error("failed to set cgroup", "path", path, "error", err)
		return
	}
}

// fromRoot returns the joined filepath of group on the CgroupRoot
func fromRoot(group string) string {
	return filepath.Join(CgroupRoot, group)
}

// getCPUsFromCgroupV2 retrieves the effective cpuset for the group, which must
// be directly under the cgroup root (i.e. the parent, like nomad.slice).
func getCPUsFromCgroupV2(group string) ([]uint16, error) {
	path := fromRoot(group)
	effective, err := cgroups.ReadFile(path, "cpuset.cpus.effective")
	if err != nil {
		return nil, err
	}
	set, err := cpuset.Parse(effective)
	if err != nil {
		return nil, err
	}
	return set.ToSlice(), nil
}

// identity is the "<allocID>.<taskName>" string that uniquely identifies an
// individual instance of a task within the flat cgroup namespace
type identity string

func makeID(allocID, task string) identity {
	return identity(fmt.Sprintf("%s.%s", allocID, task))
}

func makeScope(id identity) string {
	return string(id) + ".scope"
}
