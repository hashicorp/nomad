// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !ent
// +build !ent

package state

func (s *StateStore) enforceVariablesQuota(_ uint64, _ WriteTxn, _ string, _ int64) error {
	return nil
}
