// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"github.com/shoenig/netlog"

	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/nomad/client/lib/idset"
)

// NewPartition creates a cpuset partition manager for managing the books
// when allocations are created and destroyed. The initial set of cores is
// the usable set of cores by Nomad.
func NewPartition(cores *idset.Set[idset.CoreID]) Partition {
	// todo: how to restore this?

	return &partition{
		sharePath:   filepath.Join(root, NomadCgroupParent, ShareGroup(), "cpuset.cpus"),
		reservePath: filepath.Join(root, NomadCgroupParent, ReserveGroup(), "cpuset.cpus"),
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

func (p *partition) Reserve(cores *idset.Set[idset.CoreID]) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.share.RemoveSet(cores)
	p.reserve.InsertSet(cores)

	netlog.Green("Partition.Reserve", "cores", cores, "share", p.share, "reserve", p.reserve)
	return p.write()
}

func (p *partition) Release(cores *idset.Set[idset.CoreID]) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.reserve.RemoveSet(cores)
	p.share.InsertSet(cores)

	netlog.Green("Partition.Release", "cores", cores, "share", p.share, "reserve", p.reserve)
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
		netlog.Yellow("write", "shareStr", shareStr)
		reserveStr := p.reserve.String()
		if err := os.WriteFile(p.reservePath, []byte(reserveStr), 0644); err != nil {
			return err
		}
		netlog.Yellow("write", "reserveStr", reserveStr)
	}
	return nil
}
