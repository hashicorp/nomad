// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cgroupslib

import (
	"sync"

	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
)

// MockPartition creates an in-memory Partition manager backed by 8 fake cpu cores.
func MockPartition() Partition {
	return &mock{
		share:   idset.From[hw.CoreID]([]hw.CoreID{0, 1, 2, 3, 4, 5, 6, 7}),
		reserve: idset.Empty[hw.CoreID](),
	}
}

type mock struct {
	lock    sync.Mutex
	share   *idset.Set[hw.CoreID]
	reserve *idset.Set[hw.CoreID]
}

func (m *mock) Restore(cores *idset.Set[hw.CoreID]) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.share.RemoveSet(cores)
	m.reserve.InsertSet(cores)
}

func (m *mock) Reserve(cores *idset.Set[hw.CoreID]) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.reserve.RemoveSet(cores)
	m.share.InsertSet(cores)

	return nil
}

func (m *mock) Release(cores *idset.Set[hw.CoreID]) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.reserve.RemoveSet(cores)
	m.share.InsertSet(cores)

	return nil
}
