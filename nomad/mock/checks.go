// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package mock

import (
	"github.com/hashicorp/nomad/nomad/structs"
)

// CheckShim is a mock implementation of checkstore.Shim
//
// So far the implementation does nothing.
type CheckShim struct{}

func (s *CheckShim) Set(allocID string, result *structs.CheckQueryResult) error {
	return nil
}

func (s *CheckShim) List(allocID string) map[structs.CheckID]*structs.CheckQueryResult {
	return nil
}

func (s *CheckShim) Difference(allocID string, ids []structs.CheckID) []structs.CheckID {
	return nil
}

func (s *CheckShim) Remove(allocID string, ids []structs.CheckID) error {
	return nil
}

func (s *CheckShim) Purge(allocID string) error {
	return nil
}

func (s *CheckShim) Snapshot() map[string]string {
	return nil
}
