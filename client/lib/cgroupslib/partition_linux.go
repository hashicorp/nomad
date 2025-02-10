// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

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
		usableCores: cores.Copy(),
		log:         log,
		sharePath:   sharePath,
		reservePath: reservePath,
		share:       cores.Copy(),
		reserve:     idset.Empty[hw.CoreID](),
	}
}

type partition struct {
	log         hclog.Logger
	sharePath   string
	reservePath string
	usableCores *idset.Set[hw.CoreID]

	lock    sync.Mutex
	share   *idset.Set[hw.CoreID]
	reserve *idset.Set[hw.CoreID]
}

func (p *partition) Restore(cores *idset.Set[hw.CoreID]) {

	p.lock.Lock()
	defer p.lock.Unlock()

	p.share.RemoveSet(cores)
	// Use the intersection with the usable cores to avoid adding more cores than available.
	p.reserve.InsertSet(p.usableCores.Intersect(cores))

}

func (p *partition) Reserve(cores *idset.Set[hw.CoreID]) error {

	p.lock.Lock()
	defer p.lock.Unlock()

	// Use the intersection with the usable cores to avoid adding more cores than available.
	usableCores := p.usableCores.Intersect(cores)

	overlappingCores := p.reserve.Intersect(usableCores)
	if overlappingCores.Size() > 0 {
		// COMPAT: prior to Nomad 1.9.X this would silently happen, this should probably return an error instead
		p.log.Warn("Unable to exclusively reserve the requested cores", "cores", cores, "overlapping_cores", overlappingCores)
	}

	p.share.RemoveSet(cores)
	p.reserve.InsertSet(usableCores)

	return p.write()
}

func (p *partition) Release(cores *idset.Set[hw.CoreID]) error {

	p.lock.Lock()
	defer p.lock.Unlock()

	p.reserve.RemoveSet(cores)

	// Use the intersection with the usable cores to avoid removing more cores than available.
	p.share.InsertSet(p.usableCores.Intersect(cores))
	return p.write()
}

func (p *partition) write() error {
	shareStr := p.share.String()
	if err := os.WriteFile(p.sharePath, []byte(shareStr), 0644); err != nil {
		return fmt.Errorf("cgroupslib: unable to update share cpuset with %q: %w", shareStr, err)
	}

	reserveStr := p.reserve.String()
	if err := os.WriteFile(p.reservePath, []byte(reserveStr), 0644); err != nil {
		return fmt.Errorf("cgroupslib: unable to update reserve cpuset with %q: %w", reserveStr, err)
	}
	return nil
}
