// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package state

import "github.com/hashicorp/nomad/v2/nomad/structs"

func (s *StateStore) EnforceHostVolumeQuota(_ *structs.HostVolume, _ *structs.HostVolume) error {
	return nil
}

func (s *StateStore) enforceHostVolumeQuotaTxn(_ Txn, _ uint64, _ *structs.HostVolume, _ *structs.HostVolume, _ bool) error {
	return nil
}

func (s *StateStore) subtractVolumeFromQuotaUsageTxn(_ Txn, _ uint64, _ *structs.HostVolume) error {
	return nil
}
