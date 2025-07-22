// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
)

// IdentityDefaultAud is the default audience to use for default Nomad
// identities.
const IdentityDefaultAud = "nomadproject.io"

// IdentityClaims is an envelope for a Nomad identity JWT that can be either a
// node identity, node introduction identity, or a workload identity. It
// contains the specific claims for the identity type, as well as the common JWT
// claims.
type IdentityClaims struct {

	// NodeIdentityClaims contains the claims specific to a node identity.
	*NodeIdentityClaims

	// NodeIntroductionIdentityClaims contains the claims specific to a node
	// introduction identity.
	*NodeIntroductionIdentityClaims

	// WorkloadIdentityClaims contains the claims specific to a workload as
	// defined by an allocation running on a client.
	*WorkloadIdentityClaims

	// The public JWT claims for the identity. These claims are always present,
	// regardless of whether the identity is for a node or workload.
	jwt.Claims
}

// MarshalJSON is a custom JSON marshaler that specifically handles the node
// pool field which exists within the node identity and node introduction
// embedded objects.
func (i *IdentityClaims) MarshalJSON() ([]byte, error) {
	type Alias IdentityClaims
	exported := &struct {
		NomadNodePool string `json:"nomad_node_pool"`
		*Alias
	}{
		NomadNodePool: "",
		Alias:         (*Alias)(i),
	}
	if i.IsNodeIntroduction() {
		exported.NomadNodePool = i.NodeIntroductionIdentityClaims.NodePool
	} else if i.IsNode() {
		exported.NomadNodePool = i.NodeIdentityClaims.NodePool
	}
	return json.Marshal(exported)
}

// UnmarshalJSON is a custom JSON unmarshaler that specifically handles the node
// pool field which exists within the node identity and node introduction
// embedded objects.
func (i *IdentityClaims) UnmarshalJSON(data []byte) (err error) {
	type Alias IdentityClaims
	aux := &struct {
		NomadNodePool string `json:"nomad_node_pool"`
		*Alias
	}{
		Alias: (*Alias)(i),
	}
	if err = json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if i.IsNodeIntroduction() {
		i.NodeIntroductionIdentityClaims.NodePool = aux.NomadNodePool
	} else if i.IsNode() {
		i.NodeIdentityClaims.NodePool = aux.NomadNodePool
	}

	return nil
}

// IsNode checks if the identity JWT is a node identity.
func (i *IdentityClaims) IsNode() bool { return i != nil && i.NodeIdentityClaims != nil }

// IsNodeIntroduction checks if the identity JWT is a node introduction
// identity.
func (i *IdentityClaims) IsNodeIntroduction() bool {
	return i != nil && i.NodeIntroductionIdentityClaims != nil
}

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

	return i.Expiry.Time().UTC().Before(threshold)
}

// IsExpiringInThreshold checks if the identity JWT is expired or close to
// expiring. It uses a passed threshold to determine "close to expiring" which
// is not manipulated, unlike TTL in the IsExpiring method.
func (i *IdentityClaims) IsExpiringInThreshold(threshold time.Time) bool {
	if i != nil && i.Expiry != nil {
		return threshold.After(i.Expiry.Time())
	}
	return false
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

// setNodeSubject sets the "subject" or "sub" claim for the node introduction
// identity JWT. It follows the format
// "node-introduction:<region>:<node_pool>:<node_name>:default", where "default"
// indicates identity name. While this is currently hardcoded, it could be
// configurable in the future as we expand the node identity offering and allow
// greater control of node access.If the operator does not provide a node name,
// this is omitted from the subject.
func (i *IdentityClaims) setNodeIntroductionSubject(name, pool, region string) {

	// Build our initial subject with the node introduction type, region, and
	// pool.
	sub := []string{"node-introduction", region, pool}

	// Optionally, add the node name if it is provided. Operators set this when
	// they want to identify the node that is being introduced and limit the
	// identity use to a single node.
	if name != "" {
		sub = append(sub, name)
	}

	sub = append(sub, "default")

	i.Subject = strings.Join(sub, ":")
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
