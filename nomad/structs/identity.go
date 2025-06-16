// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"strings"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
)

// IdentityDefaultAud is the default audience to use for default Nomad
// identities.
const IdentityDefaultAud = "nomadproject.io"

// IdentityClaims is an envelope for a Nomad identity JWT that can be either a
// node identity or a workload identity. It contains the specific claims for the
// identity type, as well as the common JWT claims.
type IdentityClaims struct {

	// *NodeIdentityClaims contains the claims specific to a node identity.
	*NodeIdentityClaims

	// *WorkloadIdentityClaims contains the claims specific to a workload as
	// defined by an allocation running on a client.
	*WorkloadIdentityClaims

	// The public JWT claims for the identity. These claims are always present,
	// regardless of whether the identity is for a node or workload.
	jwt.Claims
}

// IsNode checks if the identity JWT is a node identity.
func (i *IdentityClaims) IsNode() bool { return i != nil && i.NodeIdentityClaims != nil }

// IsWorkload checks if the identity JWT is a workload identity.
func (i *IdentityClaims) IsWorkload() bool { return i != nil && i.WorkloadIdentityClaims != nil }

// IsExpiring checks if the identity JWT is expired or close to expiring. Close
// is defined as within one-third of the TTL provided.
func (i *IdentityClaims) IsExpiring(now time.Time, ttl time.Duration) bool {

	// Protect against nil identity claims and fast circuit a check on an
	// identity that does not have expiry.
	if i == nil || i.Expiry == nil {
		return false
	}

	// Calculate the threshold for "close to expiring" as one-third of the TTL
	// relative to the current time.
	threshold := now.Add(ttl / 3)

	return i.Expiry.Time().Before(threshold)
}

// setExpiry sets the "expiry" or "exp" claim for the identity JWT. It is the
// absolute time at which the JWT will expire.
//
// If no TTL is provided, the expiry will not be set, which means the JWT will
// never expire.
func (i *IdentityClaims) setExpiry(now time.Time, ttl time.Duration) {
	if ttl > 0 {
		i.Expiry = jwt.NewNumericDate(now.Add(ttl))
	}
}

// setAudience sets the "audience" or "aud" claim for the identity JWT.
func (i *IdentityClaims) setAudience(aud []string) { i.Audience = aud }

// setNodeSubject sets the "subject" or "sub" claim for the node identity JWT.
// It follows the format "node:<region>:<node_pool>:<node_id>:default", where
// "default" indicates identity name. While this is currently hardcoded, it
// could be configurable in the future as we expand the node identity offering
// and allow greater control of node access.
func (i *IdentityClaims) setNodeSubject(node *Node, region string) {
	i.Subject = strings.Join([]string{
		"node",
		region,
		node.NodePool,
		node.ID,
		"default",
	}, ":")
}

// setWorkloadSubject sets the "subject" or "sub" claim for the workload
// identity JWT. It follows the format
// "<region>:<namespace>:<job_id>:<group>:<workload_id>:<identity_id>". The
// subject does not include a type identifier which differs from the node
// identity and is something we may want to change in the future.
func (i *IdentityClaims) setWorkloadSubject(job *Job, group, wID, id string) {
	i.Subject = strings.Join([]string{
		job.Region,
		job.Namespace,
		job.GetIDforWorkloadIdentity(),
		group,
		wID,
		id,
	}, ":")
}
