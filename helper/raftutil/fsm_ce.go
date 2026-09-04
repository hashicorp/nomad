// Copyright IBM Corp. 2015, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent

package raftutil

import "github.com/hashicorp/nomad/nomad/state"

func insertEnterpriseState(m map[string][]any, state *state.StateStore) {
}
