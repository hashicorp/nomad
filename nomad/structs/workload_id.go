// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"slices"

	"github.com/hashicorp/go-multierror"
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
)

var (
	// validIdentityName is used to validate workload identity Name fields. Must
	// be safe to use in filenames.
	//
	// Reuse validNamespaceName to save a bit of memory.
	validIdentityName = validNamespaceName
)

// WorkloadIdentity is the jobspec block which determines if and how a workload
// identity is exposed to tasks similar to the Vault block.
//
// CAUTION: a copy of this struct definition lives in config/consul.go in order
// to avoid import cycles. If updating WorkloadIdentity, please remember to update
// its copy as well.
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

	// ServiceName is used to bind the identity to a correct Consul service.
	ServiceName string
}

func (wi *WorkloadIdentity) Copy() *WorkloadIdentity {
	if wi == nil {
		return nil
	}
	return &WorkloadIdentity{
		Name:        wi.Name,
		Audience:    slices.Clone(wi.Audience),
		Env:         wi.Env,
		File:        wi.File,
		ServiceName: wi.ServiceName,
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

	if wi.ServiceName != other.ServiceName {
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
	}
}

func (wi *WorkloadIdentity) Validate() error {
	if wi == nil {
		return fmt.Errorf("must not be nil")
	}

	var mErr multierror.Error

	if !validIdentityName.MatchString(wi.Name) {
		err := fmt.Errorf("invalid name %q. Must match regex %s", wi.Name, validIdentityName)
		mErr.Errors = append(mErr.Errors, err)
	}

	for i, aud := range wi.Audience {
		if aud == "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("an empty string is an invalid audience (%d)", i+1))
		}
	}

	return mErr.ErrorOrNil()
}

func (wi *WorkloadIdentity) Warnings() error {
	if wi == nil {
		return fmt.Errorf("must not be nil")
	}

	if n := len(wi.Audience); n == 0 {
		return fmt.Errorf("identities without an audience are insecure")
	} else if n > 1 {
		return fmt.Errorf("while multiple audiences is allowed, it is more secure to use 1 audience per identity")
	}

	return nil
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
	Identities []*WorkloadIdentityRequest
	QueryOptions
}

// AllocIdentitiesResponse is the RPC response for requested workload
// identities including any rejections.
type AllocIdentitiesResponse struct {
	SignedIdentities []*SignedWorkloadIdentity
	Rejections       []*WorkloadIdentityRejection
	QueryMeta
}
