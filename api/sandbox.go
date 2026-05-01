// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"time"
)

type SandboxVolumeRequest struct {
	// MaxCount in the maximum number of identical sandboxes with this name and
	// namespace that should be created. We'll always try to reclaim oldest
	// sandboxes first.
	MaxCount int `hcl:"max_count,optional"`

	MaxClaims int `hcl:"max_claims,optional"`

	// TTL is the lifetime of an unclaimed sandbox. After this point it can be
	// garbage collected.
	TTL time.Duration `hcl:"ttl,optional"`

	MinBytes int64 `hcl:"min_bytes"`
	MaxBytes int64 `hcl:"max_bytes"`
}
