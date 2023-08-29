// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cgroupslib

import (
	"sync"

	"github.com/hashicorp/nomad/client/lib/idset"
)

// MockPartition creates an in-memory Partition manager backed by 8 fake cpu cores.
func MockPartition() Partition {
	return &mock{
		share:   idset.From[idset.CoreID]([]idset.CoreID{0, 1, 2, 3, 4, 5, 6, 7}),
		reserve: idset.Empty[idset.CoreID](),
	}
}

type mock struct {
	lock    sync.Mutex
	share   *idset.Set[idset.CoreID]
	reserve *idset.Set[idset.CoreID]
}

func (m *mock) Restore(cores *idset.Set[idset.CoreID]) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.share.RemoveSet(cores)
	m.reserve.InsertSet(cores)
}

func (m *mock) Reserve(cores *idset.Set[idset.CoreID]) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.reserve.RemoveSet(cores)
	m.share.InsertSet(cores)

	return nil
}

func (m *mock) Release(cores *idset.Set[idset.CoreID]) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.reserve.RemoveSet(cores)
	m.share.InsertSet(cores)

	return nil
}
