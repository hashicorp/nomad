// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"maps"
	"slices"
	"time"

	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/helper/pointer"
)

// WorkloadIdentityConfig is the agent configuraion block used to define
// default workload identities.
//
// This based on the WorkloadIdentity struct from nomad/structs/workload_id.go
// and may need to be kept in sync.
type WorkloadIdentityConfig struct {
	// Name is used to identity the workload identity. It is not expected to be
	// set by users, but may have a default value.
	Name string `mapstructure:"-" json:"-"`

	// Audience is the valid recipients for this identity (the "aud" JWT claim)
	// and defaults to the identity's name.
	Audience []string `mapstructure:"aud"`

	// Env injects the Workload Identity into the Task's environment if
	// set.
	Env *bool `mapstructure:"env"`

	// File writes the Workload Identity into the Task's secrets directory
	// or specified filepath if set.
	File *bool `mapstructure:"file"`

	// Filepath write the Workload Identity to a specified directory in the
	// Task's filesystem
	Filepath string `mapstructure:"filepath"`

	// TTL is used to determine the expiration of the credentials created for
	// this identity (eg the JWT "exp" claim).
	TTL    *time.Duration `mapstructure:"-"`
	TTLHCL string         `mapstructure:"ttl" json:"-"`

	// ExtraClaims allows a WI configuration to carry extra claims configured by
	// the cluster administrator. Note this field is not available on jobspecs.
	ExtraClaims map[string]string `mapstructure:"extra_claims"`
}

func (wi *WorkloadIdentityConfig) Copy() *WorkloadIdentityConfig {
	if wi == nil {
		return nil
	}
	nwi := new(WorkloadIdentityConfig)
	*nwi = *wi
	nwi.Audience = slices.Clone(wi.Audience)

	if wi.Env != nil {
		nwi.Env = pointer.Of(*wi.Env)
	}
	if wi.File != nil {
		nwi.File = pointer.Of(*wi.File)
	}
	if wi.TTL != nil {
		nwi.TTL = pointer.Of(*wi.TTL)
	}
	nwi.ExtraClaims = maps.Clone(wi.ExtraClaims)

	return nwi
}

func (wi *WorkloadIdentityConfig) Equal(other *WorkloadIdentityConfig) bool {
	if wi == nil || other == nil {
		return wi == other
	}

	if wi.Name != other.Name {
		return false
	}
	if !slices.Equal(wi.Audience, other.Audience) {
		return false
	}
	if !pointer.Eq(wi.Env, other.Env) {
		return false
	}
	if !pointer.Eq(wi.File, other.File) {
		return false
	}
	if wi.Filepath != other.Filepath {
		return false
	}
	if !pointer.Eq(wi.TTL, other.TTL) {
		return false
	}
	if wi.TTLHCL != other.TTLHCL {
		return false
	}
	if !maps.Equal(wi.ExtraClaims, other.ExtraClaims) {
		return false
	}

	return true
}

func (wi *WorkloadIdentityConfig) Merge(other *WorkloadIdentityConfig) *WorkloadIdentityConfig {
	result := wi.Copy()

	if other.Name != "" {
		result.Name = other.Name
	}

	if len(result.Audience) == 0 {
		result.Audience = slices.Clone(other.Audience)
	} else if len(other.Audience) > 0 {
		// Append incoming audiences avoiding duplicates.
		audSet := set.From(result.Audience)
		for _, aud := range other.Audience {
			if !audSet.Contains(aud) {
				audSet.Insert(aud)
				result.Audience = append(result.Audience, aud)
			}
		}
	}

	result.Env = pointer.Merge(result.Env, other.Env)
	result.File = pointer.Merge(result.File, other.File)
	result.TTL = pointer.Merge(result.TTL, other.TTL)
	if other.Filepath != "" {
		result.Filepath = other.Filepath
	}
	if other.TTLHCL != "" {
		result.TTLHCL = other.TTLHCL
	}

	if wi.ExtraClaims == nil {
		result.ExtraClaims = map[string]string{}
	}
	for k, v := range other.ExtraClaims {
		result.ExtraClaims[k] = v
	}

	return result
}
