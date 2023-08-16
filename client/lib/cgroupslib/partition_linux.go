// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
)

// GetPartition creates a Partition suitable for managing cores on this
// Linux system.
func GetPartition(cores *idset.Set[hw.CoreID]) Partition {
	return NewPartition(cores)
}

// NewPartition creates a cpuset partition manager for managing the books
// when allocations are created and destroyed. The initial set of cores is
// the usable set of cores by Nomad.
func NewPartition(cores *idset.Set[hw.CoreID]) Partition {
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
		sharePath:   sharePath,
		reservePath: reservePath,
		share:       cores.Copy(),
		reserve:     idset.Empty[hw.CoreID](),
	}
}

type partition struct {
	sharePath   string
	reservePath string

	lock    sync.Mutex
	share   *idset.Set[hw.CoreID]
	reserve *idset.Set[hw.CoreID]
}

func (p *partition) Restore(cores *idset.Set[hw.CoreID]) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.share.RemoveSet(cores)
	p.reserve.InsertSet(cores)
}

func (p *partition) Reserve(cores *idset.Set[hw.CoreID]) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.share.RemoveSet(cores)
	p.reserve.InsertSet(cores)

	return p.write()
}

func (p *partition) Release(cores *idset.Set[hw.CoreID]) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.reserve.RemoveSet(cores)
	p.share.InsertSet(cores)

	return p.write()
}

func (p *partition) write() error {
	shareStr := p.share.String()
	if err := os.WriteFile(p.sharePath, []byte(shareStr), 0644); err != nil {
		return err
	}
	reserveStr := p.reserve.String()
	if err := os.WriteFile(p.reservePath, []byte(reserveStr), 0644); err != nil {
		return err
	}
	return nil
}
