// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"golang.org/x/exp/slices"
)

const (
	// WorkloadIdentityDefaultName is the name of the default (builtin) Workload
	// Identity.
	WorkloadIdentityDefaultName = "default"

	// WorkloadIdentityDefaultAud is the audience of the default identity.
	WorkloadIdentityDefaultAud = "nomadproject.io"
)

// WorkloadIdentity is the jobspec block which determines if and how a workload
// identity is exposed to tasks similar to the Vault block.
type WorkloadIdentity struct {
	Name string

	// Audiences are the valid recipients for this identity (the "aud" JWT claim)
	// and defaults to the identity's name.
	Audiences []string

	// Env injects the Workload Identity into the Task's environment if
	// set.
	Env bool

	// File writes the Workload Identity into the Task's secrets directory
	// if set.
	File bool
}

func (wi *WorkloadIdentity) Copy() *WorkloadIdentity {
	if wi == nil {
		return nil
	}
	return &WorkloadIdentity{
		Name:      wi.Name,
		Audiences: slices.Clone(wi.Audiences),
		Env:       wi.Env,
		File:      wi.File,
	}
}

func (wi *WorkloadIdentity) Equal(other *WorkloadIdentity) bool {
	if wi == nil || other == nil {
		return wi == other
	}

	if wi.Name != other.Name {
		return false
	}

	if !slices.Equal(wi.Audiences, other.Audiences) {
		return false
	}

	if wi.Env != other.Env {
		return false
	}

	if wi.File != other.File {
		return false
	}

	return true
}

func (wi *WorkloadIdentity) Canonicalize() {
	// An unnamed identity block is treated as the default Nomad Workload
	// Identity.
	if wi.Name == "" {
		wi.Name = WorkloadIdentityDefaultName
	}

	// The default identity is only valid for use with Nomad itself.
	if wi.Name == WorkloadIdentityDefaultName {
		wi.Audiences = []string{WorkloadIdentityDefaultAud}
	}

	// If no audience is set, use the block name.
	if len(wi.Audiences) == 0 {
		wi.Audiences = []string{wi.Name}
	}
}
