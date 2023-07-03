// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"time"

	"golang.org/x/exp/slices"
)

const (
	// WorkloadIdentityDefaultName is the name of the default (builtin) Workload
	// Identity.
	WorkloadIdentityDefaultName = "default"

	// WorkloadIdentityDefaultAud is the audience of the default identity.
	WorkloadIdentityDefaultAud = "nomadproject.io"

	// WIRejectionReasonMissingAlloc is the WorkloadIdentityRejection.Reason
	// returned when an allocation longer exists. This may be due to the alloc
	// being GC'd or the job being updated.
	WIRejectionReasonMissingAlloc = "allocation not found"

	// WIRejectionReasonMissingTask is the WorkloadIdentityRejection.Reason
	// returned when the requested task no longer exists on the allocation.
	WIRejectionReasonMissingTask = "task not found"

	// WIRejectionReasonMissingIdentity is the WorkloadIdentityRejection.Reason
	// returned when the requested identity does not exist on the allocation.
	WIRejectionReasonMissingIdentity = "identity not found"

	// WIChangeModes are the change_mode values supported by workload identities.
	WIChangeModeNoop    = "noop"
	WIChangeModeRestart = "restart"
	WIChangeModeSignal  = "signal"
)

// WorkloadIdentity is the jobspec block which determines if and how a workload
// identity is exposed to tasks similar to the Vault block.
type WorkloadIdentity struct {
	Name string

	// Audience is the valid recipients for this identity (the "aud" JWT claim)
	// and defaults to the identity's name.
	Audience []string

	// Env injects the Workload Identity into the Task's environment if
	// set.
	Env bool

	// File writes the Workload Identity into the Task's secrets directory
	// if set.
	File bool

	// TTL is used to determine the expiration of the credentials created for
	// this identity (eg the JWT "exp" claim).
	TTL time.Duration

	// Splay is a duration used to jitter credential expiration. For example a
	// JWT's exp = ttl + rand(0, splay)
	Splay time.Duration

	// ChangeMode is similar to the Vault block's change_mode and determines what
	// Nomad does when the credentials for this identity change (eg when a JWT is
	// rotated prior to expiration).
	ChangeMode string
}

func (wi *WorkloadIdentity) Copy() *WorkloadIdentity {
	if wi == nil {
		return nil
	}
	return &WorkloadIdentity{
		Name:       wi.Name,
		Audience:   slices.Clone(wi.Audience),
		Env:        wi.Env,
		File:       wi.File,
		TTL:        wi.TTL,
		Splay:      wi.Splay,
		ChangeMode: wi.ChangeMode,
	}
}

func (wi *WorkloadIdentity) Equal(other *WorkloadIdentity) bool {
	if wi == nil || other == nil {
		return wi == other
	}

	if wi.Name != other.Name {
		return false
	}

	if !slices.Equal(wi.Audience, other.Audience) {
		return false
	}

	if wi.Env != other.Env {
		return false
	}

	if wi.File != other.File {
		return false
	}

	if wi.TTL != other.TTL {
		return false
	}

	if wi.Splay != other.Splay {
		return false
	}

	if wi.ChangeMode != other.ChangeMode {
		return false
	}

	return true
}

func (wi *WorkloadIdentity) Canonicalize() {
	if wi == nil {
		return
	}

	if wi.Name == "" {
		wi.Name = WorkloadIdentityDefaultName
	}

	// The default identity is only valid for use with Nomad itself.
	if wi.Name == WorkloadIdentityDefaultName {
		wi.Audience = []string{WorkloadIdentityDefaultAud}
		//TODO(schmichael) - should probably set all default defaults somewhere
		//else so we can detect and maximze the ttl
	}

	// If no audience is set, use the block name.
	if len(wi.Audience) == 0 {
		wi.Audience = []string{wi.Name}
	}

	//TODO(schmichael) should ttl and splay be defaulted here?

	// If no ChangeMode is set, default to noop
	wi.ChangeMode = WIChangeModeNoop
}

// WorkloadIdentityRequest encapsulates the 3 parameters used to generated a
// signed workload identity: the alloc, task, and specific identity's name.
type WorkloadIdentityRequest struct {
	AllocID      string
	TaskName     string
	IdentityName string
}

// SignedWorkloadIdentity is the response to a WorkloadIdentityRequest and
// includes the JWT for the requested workload identity.
type SignedWorkloadIdentity struct {
	WorkloadIdentityRequest
	JWT string
}

// WorkloadIdentityRejection is the response to a WorkloadIdentityRequest that
// is rejected and includes a reason.
type WorkloadIdentityRejection struct {
	WorkloadIdentityRequest
	Reason string
}

// AllocIdentitiesRequest is the RPC arguments for requesting signed workload
// identities.
type AllocIdentitiesRequest struct {
	Identities []WorkloadIdentityRequest
	QueryOptions
}

// AllocIdentitiesResponse is the RPC response for requested workload
// identities including any rejections.
type AllocIdentitiesResponse struct {
	SignedIdentities []SignedWorkloadIdentity
	Rejections       []WorkloadIdentityRejection
	QueryMeta
}
