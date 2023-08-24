// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/nomad/client/lib/idset"
)

// NewPartition creates a cpuset partition manager for managing the books
// when allocations are created and destroyed. The initial set of cores is
// the usable set of cores by Nomad.
func NewPartition(cores *idset.Set[idset.CoreID]) Partition {
	return &partition{
		sharePath:   filepath.Join(root, NomadCgroupParent, SharePartition(), "cpuset.cpus"),
		reservePath: filepath.Join(root, NomadCgroupParent, ReservePartition(), "cpuset.cpus"),
		share:       cores.Copy(),
		reserve:     idset.Empty[idset.CoreID](),
	}
}

type partition struct {
	sharePath   string
	reservePath string

	lock    sync.Mutex
	share   *idset.Set[idset.CoreID]
	reserve *idset.Set[idset.CoreID]
}

func (p *partition) Restore(cores *idset.Set[idset.CoreID]) {
	p.lock.Lock()
	p.lock.Unlock()

	p.share.RemoveSet(cores)
	p.reserve.InsertSet(cores)
}

func (p *partition) Reserve(cores *idset.Set[idset.CoreID]) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.share.RemoveSet(cores)
	p.reserve.InsertSet(cores)

	return p.write()
}

func (p *partition) Release(cores *idset.Set[idset.CoreID]) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.reserve.RemoveSet(cores)
	p.share.InsertSet(cores)

	return p.write()
}

func (p *partition) write() error {
	switch GetMode() {
	case CG1:
		panic("not yet implemented")
	case CG2:
		shareStr := p.share.String()
		if err := os.WriteFile(p.sharePath, []byte(shareStr), 0644); err != nil {
			return err
		}
		reserveStr := p.reserve.String()
		if err := os.WriteFile(p.reservePath, []byte(reserveStr), 0644); err != nil {
			return err
		}
	}
	return nil
}
