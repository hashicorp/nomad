// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
)

// GetPartition creates a Partition suitable for managing cores on this
// Linux system.
func GetPartition(log hclog.Logger, cores *idset.Set[hw.CoreID]) Partition {
	return NewPartition(log, cores)
}

// NewPartition creates a cpuset partition manager for managing the books
// when allocations are created and destroyed. The initial set of cores is
// the usable set of cores by Nomad.
func NewPartition(log hclog.Logger, cores *idset.Set[hw.CoreID]) Partition {
	var (
		sharePath   string
		reservePath string
	)

	switch GetMode() {
	case OFF:
		return NoopPartition()
	case CG1:
		sharePath = filepath.Join(root, "cpuset", NomadCgroupParent, SharePartition(), "cpuset.cpus")
		reservePath = filepath.Join(root, "cpuset", NomadCgroupParent, ReservePartition(), "cpuset.cpus")
	case CG2:
		sharePath = filepath.Join(root, NomadCgroupParent, SharePartition(), "cpuset.cpus")
		reservePath = filepath.Join(root, NomadCgroupParent, ReservePartition(), "cpuset.cpus")
	}

	return &partition{
		usableCores:  cores.Copy(),
		log:          log,
		sharePath:    sharePath,
		reservePath:  reservePath,
		reservations: make(map[string]*idset.Set[hw.CoreID]),
	}
}

type partition struct {
	log         hclog.Logger
	sharePath   string
	reservePath string
	usableCores *idset.Set[hw.CoreID]

	lock sync.Mutex

	// reservations tracks the cores held by each allocation, keyed by
	// allocation ID. The reserve cpuset is the union of these sets and the
	// share cpuset is whatever is left of usableCores.
	//
	// The books are kept per allocation rather than as a single set of
	// reserved cores so that releasing one allocation never removes a core
	// still held by another. Two allocations can briefly hold the same core
	// when the scheduler places a replacement before the client has finished
	// stopping the allocation it replaces, and a Release may arrive without a
	// matching Reserve when an earlier prerun hook fails. With a single set
	// either case empties the reserve cpuset while a task still lives in it,
	// which the kernel refuses (ENOSPC on cgroups v2, EBUSY on v1), and every
	// allocation on the node then fails until that task exits.
	reservations map[string]*idset.Set[hw.CoreID]
}

// reserved returns the union of all reserved cores. Must be called with the
// lock held.
func (p *partition) reserved() *idset.Set[hw.CoreID] {
	reserve := idset.Empty[hw.CoreID]()
	for _, cores := range p.reservations {
		reserve.InsertSet(cores)
	}
	return reserve
}

// shared returns the usable cores not reserved by any allocation. Must be
// called with the lock held.
func (p *partition) shared() *idset.Set[hw.CoreID] {
	share := p.usableCores.Copy()
	share.RemoveSet(p.reserved())
	return share
}

func (p *partition) Restore(allocID string, cores *idset.Set[hw.CoreID]) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.add(allocID, cores)
}

func (p *partition) Reserve(allocID string, cores *idset.Set[hw.CoreID]) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	// Use the intersection with the usable cores to avoid adding more cores than available.
	usableCores := p.usableCores.Intersect(cores)

	// The same allocation may reserve its cores more than once (e.g. after a
	// client restart) which is not an overlap; only consider other allocations.
	overlappingCores := idset.Empty[hw.CoreID]()
	for id, held := range p.reservations {
		if id != allocID {
			overlappingCores.InsertSet(held.Intersect(usableCores))
		}
	}
	if overlappingCores.Size() > 0 {
		// COMPAT: prior to Nomad 1.9.X this would silently happen, this should probably return an error instead
		p.log.Warn("Unable to exclusively reserve the requested cores", "alloc_id", allocID, "cores", cores.Slice(), "overlapping_cores", overlappingCores.Slice())
	}

	p.add(allocID, usableCores)

	return p.write()
}

func (p *partition) Release(allocID string) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	delete(p.reservations, allocID)

	return p.write()
}

// add records cores as held by the allocation, keeping the books free of
// empty entries so that allocations without reserved cores are not tracked.
// Must be called with the lock held.
func (p *partition) add(allocID string, cores *idset.Set[hw.CoreID]) {
	usableCores := p.usableCores.Intersect(cores)
	if usableCores.Size() == 0 {
		return
	}
	held, ok := p.reservations[allocID]
	if !ok {
		held = idset.Empty[hw.CoreID]()
		p.reservations[allocID] = held
	}
	held.InsertSet(usableCores)
}

func (p *partition) write() error {
	if err := p.writeCpuset(p.sharePath, p.shared()); err != nil {
		return fmt.Errorf("cgroupslib: unable to update share cpuset: %w", err)
	}

	if err := p.writeCpuset(p.reservePath, p.reserved()); err != nil {
		return fmt.Errorf("cgroupslib: unable to update reserve cpuset: %w", err)
	}
	return nil
}

func (p *partition) writeCpuset(path string, cores *idset.Set[hw.CoreID]) error {
	value := cores.String()
	err := os.WriteFile(path, []byte(value), 0644)
	if err == nil {
		return nil
	}

	// The kernel refuses to empty the cpuset of a cgroup that still has tasks
	// in it or in any of its descendants (validate_change in
	// kernel/cgroup/cpuset.c returns ENOSPC). This happens when the last
	// reservation on the node is released while a task lingers in the reserve
	// cgroup, or when every usable core is reserved while a task lingers in
	// the share cgroup. Failing here would fail the allocation being started
	// or stopped, and every allocation after it until the lingering task
	// exits. Keeping the previous cpuset is harmless: on cgroups v2 an empty
	// cpuset would fall back to the parent's cores anyway, so the tasks in the
	// cgroup are no more exclusive than they would have been had the write
	// succeeded. The next write will set the cpuset once it is non-empty again.
	if cores.Size() == 0 && errors.Is(err, syscall.ENOSPC) {
		p.log.Warn("Unable to empty cpuset of a cgroup that still has tasks; leaving previous value", "path", path, "error", err)
		return nil
	}

	return fmt.Errorf("write %q to %s: %w", value, path, err)
}
