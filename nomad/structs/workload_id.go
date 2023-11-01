// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
)

const (
	// WorkloadIdentityDefaultName is the name of the default (builtin) Workload
	// Identity.
	WorkloadIdentityDefaultName = "default"

	// WorkloadIdentityDefaultAud is the audience of the default identity.
	WorkloadIdentityDefaultAud = "nomadproject.io"

	// WorkloadIdentityVaultPrefix is the name prefix of workload identities
	// used to derive Vault tokens.
	WorkloadIdentityVaultPrefix = "vault_"

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

	// WIChangeModeNoop takes no action when a new token is retrieved.
	WIChangeModeNoop = "noop"

	// WIChangeModeSignal signals the task when a new token is retrieved.
	WIChangeModeSignal = "signal"

	// WIChangeModeRestart restarts the task when a new token is retrieved.
	WIChangeModeRestart = "restart"
)

var (
	// validIdentityName is used to validate workload identity Name fields. Must
	// be safe to use in filenames.
	validIdentityName = regexp.MustCompile("^[a-zA-Z0-9-_]{1,128}$")
)

// WorkloadIdentity is the jobspec block which determines if and how a workload
// identity is exposed to tasks similar to the Vault block.
//
// CAUTION: a similar struct called WorkloadIdentityConfig lives in
// nomad/structs/config/workload_id.go and is used for agent configuration.
// Updates here may need to be applied there as well.
type WorkloadIdentity struct {
	Name string

	// Audience is the valid recipients for this identity (the "aud" JWT claim)
	// and defaults to the identity's name.
	Audience []string

	// ChangeMode is used to configure the task's behavior when the identity
	// token changes.
	ChangeMode string

	// ChangeSignal is the signal sent to the task when a new token is
	// retrieved. This is only valid when using the signal change mode.
	ChangeSignal string

	// Env injects the Workload Identity into the Task's environment if
	// set.
	Env bool

	// File writes the Workload Identity into the Task's secrets directory
	// if set.
	File bool

	// ServiceName is used to bind the identity to a correct Consul service.
	ServiceName string

	// TTL is used to determine the expiration of the credentials created for
	// this identity (eg the JWT "exp" claim).
	TTL time.Duration
}

func (wi *WorkloadIdentity) Copy() *WorkloadIdentity {
	if wi == nil {
		return nil
	}
	return &WorkloadIdentity{
		Name:         wi.Name,
		Audience:     slices.Clone(wi.Audience),
		ChangeMode:   wi.ChangeMode,
		ChangeSignal: wi.ChangeSignal,
		Env:          wi.Env,
		File:         wi.File,
		ServiceName:  wi.ServiceName,
		TTL:          wi.TTL,
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

	if wi.ChangeMode != other.ChangeMode {
		return false
	}

	if wi.ChangeSignal != other.ChangeSignal {
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

	if wi.TTL != other.TTL {
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

	if wi.ChangeSignal != "" {
		wi.ChangeSignal = strings.ToUpper(wi.ChangeSignal)
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

	switch wi.ChangeMode {
	case "", WIChangeModeNoop, WIChangeModeRestart:
		// Treat "" as noop. Make sure signal isn't set.
		if wi.ChangeSignal != "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("can only use change_signal=%q with change_mode=%q",
				wi.ChangeSignal, WIChangeModeSignal))
		}
	case WIChangeModeSignal:
		if wi.ChangeSignal == "" {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("change_signal must be specified when using change_mode=%q", WIChangeModeSignal))
		}
	default:
		// Unknown change_mode
		mErr.Errors = append(mErr.Errors, fmt.Errorf("invalid change_mode: %s", wi.ChangeMode))
	}

	if wi.TTL > 0 && (wi.Name == "" || wi.Name == WorkloadIdentityDefaultName) {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("ttl for default identity not yet supported"))
	}

	if wi.TTL < 0 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("ttl must be >= 0"))
	}

	return mErr.ErrorOrNil()
}

func (wi *WorkloadIdentity) Warnings() error {
	if wi == nil {
		return fmt.Errorf("must not be nil")
	}

	var mErr multierror.Error

	if n := len(wi.Audience); n == 0 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("identities without an audience are insecure"))
	} else if n > 1 {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("while multiple audiences is allowed, it is more secure to use 1 audience per identity"))
	}

	if wi.Name != "" && wi.Name != WorkloadIdentityDefaultName {
		if wi.TTL == 0 {
			mErr.Errors = append(mErr.Errors, fmt.Errorf("identities without an expiration are insecure"))
		}
	}

	// Warn users about using env vars without restarts
	if wi.Env && wi.ChangeMode != WIChangeModeRestart {
		mErr.Errors = append(mErr.Errors, fmt.Errorf("using env=%t without change_mode=%q may result in task not getting updated identity",
			wi.Env, WIChangeModeRestart))
	}

	return mErr.ErrorOrNil()
}

// WorkloadIdentityRequest encapsulates the 3 parameters used to generated a
// signed workload identity: the alloc, task, and specific identity's name.
type WorkloadIdentityRequest struct {
	AllocID string
	WIHandle
}

// SignedWorkloadIdentity is the response to a WorkloadIdentityRequest and
// includes the JWT for the requested workload identity.
type SignedWorkloadIdentity struct {
	WorkloadIdentityRequest
	JWT        string
	Expiration time.Time
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

type WorkloadType int

const (
	WorkloadTypeTask WorkloadType = iota
	WorkloadTypeService
)

// WIHandle is used by code that needs to uniquely match a workload identity
// with the task or service it belongs to.
type WIHandle struct {
	IdentityName string
	// WorkloadIdentifier is either a ServiceName or a TaskName
	WorkloadIdentifier string
	WorkloadType       WorkloadType
}

func (w WIHandle) Equal(o WIHandle) bool {
	return w.IdentityName == o.IdentityName &&
		w.WorkloadIdentifier == o.WorkloadIdentifier &&
		w.WorkloadType == o.WorkloadType
}
