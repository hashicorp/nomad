// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-hclog"
	metrics "github.com/hashicorp/go-metrics/compat"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
)

// aclCacheSize is the number of ACL objects to keep cached. ACLs have a parsing
// and construction cost, so we keep the hot objects cached to reduce the ACL
// token resolution time.
const aclCacheSize = 512

type StateGetter func() *state.StateStore
type LeaderACLGetter func() string

type RPCContext interface {
	IsTLS() bool
	IsStatic() bool
	Certificate() *x509.Certificate
	GetRemoteIP() (net.IP, error)
}

type Encrypter interface {
	VerifyClaim(string) (*structs.IdentityClaims, error)
}

type Authenticator struct {
	aclsEnabled bool

	// verifyTLS is used to determine whether the server should verify TLS and
	// is an atomic bool, so that the server TLS reload can update it at runtime
	// with a race condition.
	verifyTLS *atomic.Bool

	logger       hclog.Logger
	getState     StateGetter
	getLeaderACL LeaderACLGetter
	region       string

	validServerCertNames []string
	validClientCertNames []string

	// aclCache is used to maintain the parsed ACL objects
	aclCache *structs.ACLCache[*acl.ACL]

	// encrypter is a pointer to the server's Encrypter that can be used to
	// verify claims
	encrypter Encrypter
}

type AuthenticatorConfig struct {
	StateFn        StateGetter
	Logger         hclog.Logger
	GetLeaderACLFn LeaderACLGetter
	AclsEnabled    bool
	VerifyTLS      bool
	Region         string
	Encrypter      Encrypter
}

func NewAuthenticator(cfg *AuthenticatorConfig) *Authenticator {
	a := Authenticator{
		aclsEnabled:          cfg.AclsEnabled,
		verifyTLS:            &atomic.Bool{},
		logger:               cfg.Logger.With("auth"),
		getState:             cfg.StateFn,
		getLeaderACL:         cfg.GetLeaderACLFn,
		region:               cfg.Region,
		aclCache:             structs.NewACLCache[*acl.ACL](aclCacheSize),
		encrypter:            cfg.Encrypter,
		validServerCertNames: []string{"server." + cfg.Region + ".nomad"},
		validClientCertNames: []string{
			"client." + cfg.Region + ".nomad",
			"server." + cfg.Region + ".nomad",
		},
	}

	a.verifyTLS.Store(cfg.VerifyTLS)
	return &a
}

// SetVerifyTLS is a helper method to set the verifyTLS field. This is used in
// when the server TLS configuration is updated.
func (s *Authenticator) SetVerifyTLS(verifyTLS bool) { s.verifyTLS.Store(verifyTLS) }

// Authenticate extracts an AuthenticatedIdentity from the request context or
// provided token and sets the identity on the request. The caller can extract
// an acl.ACL, WorkloadIdentity, or other identifying tokens to use for
// authorization. Keeping these fields independent rather than merging them into
// an ephemeral ACLToken makes the original of the credential clear to RPC
// handlers, who may have different behavior for internal vs external origins.
//
// Note: when making a server-to-server RPC that authenticates with this method,
// the RPC *must* include the leader's ACL token. Use AuthenticateServerOnly for
// requests that don't have access to the leader's ACL token.
//
// Note: when called on the follower we'll be making stale queries, so it's
// possible if the follower is behind that the leader will get a different value
// if an ACL token or allocation's WI has just been created.
//
// This method returns errors that are used for testing diagnostics. RPC callers
// should always return ErrPermissionDenied after checking forwarding when one
// of these errors is received.
func (s *Authenticator) Authenticate(ctx RPCContext, args structs.RequestWithIdentity) error {

	// get the user ACLToken or anonymous token
	secretID := args.GetAuthToken()
	aclToken, err := s.resolveSecretToken(secretID)

	switch {
	case err == nil && (aclToken == structs.AnonymousACLToken ||
		aclToken == structs.ACLsDisabledToken):
		// When ACLs are disabled or if we have an anonymous token, we want to
		// continue on to check mTLS certs, if available, so set the token but
		// don't return yet
		args.SetIdentity(&structs.AuthenticatedIdentity{ACLToken: aclToken})

	case err == nil:
		// ACLs are enabled and we have a non-anonymous token, so set that as
		// our identity and return
		args.SetIdentity(&structs.AuthenticatedIdentity{ACLToken: aclToken})
		return nil

	case errors.Is(err, structs.ErrTokenExpired):
		return err

	case errors.Is(err, structs.ErrTokenInvalid):
		// if it's not a UUID it might be an identity claim
		claims, err := s.VerifyClaim(secretID)
		if err != nil {
			// we already know the token wasn't valid for an ACL in the state
			// store, so if we get an error at this point we have an invalid
			// token and there are no other options but to bail out
			return err
		}

		args.SetIdentity(&structs.AuthenticatedIdentity{Claims: claims})
		return nil

	case errors.Is(err, structs.ErrTokenNotFound):
		// Check if the secret ID is the leader's secret ID, in which case treat
		// it as a management token.
		leaderAcl := s.getLeaderACL()
		if leaderAcl != "" && secretID == leaderAcl {
			aclToken = structs.LeaderACLToken
			break
		} else {
			// Otherwise, see if the secret ID belongs to a node. We should
			// reach this point only on first connection.
			node, err := s.getState().NodeBySecretID(nil, secretID)
			if err != nil {
				// this is a go-memdb error; shouldn't happen
				return fmt.Errorf("could not resolve node secret: %w", err)
			}
			if node != nil {
				args.SetIdentity(&structs.AuthenticatedIdentity{ClientID: node.ID})
				return nil
			}
		}

		// we were passed a bogus token so we'll return an error, but we'll also
		// want to capture the IP for metrics
		remoteIP, err := ctx.GetRemoteIP()
		if err != nil {
			s.logger.Error("could not determine remote address", "error", err)
		}
		args.SetIdentity(&structs.AuthenticatedIdentity{RemoteIP: remoteIP})
		return structs.ErrPermissionDenied

	default: // any other error
		return fmt.Errorf("could not resolve user: %w", err)

	}

	// If there's no context we're in a "static" handler which only happens for
	// cases where the leader is making RPCs internally (volumewatcher and
	// deploymentwatcher)
	if ctx.IsStatic() {
		args.SetIdentity(&structs.AuthenticatedIdentity{ACLToken: aclToken})
		return nil
	}

	// At this point we either have an anonymous token or an invalid one.

	// Unlike clients that provide their Node ID on first connection, server
	// RPCs don't include an ID for the server so we identify servers by cert
	// and IP address.
	identity := &structs.AuthenticatedIdentity{ACLToken: aclToken}
	if ctx.IsTLS() {
		identity.TLSName = ctx.Certificate().Subject.CommonName
	}

	remoteIP, err := ctx.GetRemoteIP()
	if err != nil {
		s.logger.Error(
			"could not authenticate RPC request or determine remote address", "error", err)
		return err
	}
	identity.RemoteIP = remoteIP
	args.SetIdentity(identity)
	return nil
}

// ResolveACL is an authentication wrapper which handles resolving ACL tokens,
// Workload Identities, or client secrets into acl.ACL objects. Exclusively
// server-to-server or client-to-server requests should be using
// AuthenticateServerOnly or AuthenticateClientOnly and never use this method.
func (s *Authenticator) ResolveACL(args structs.RequestWithIdentity) (*acl.ACL, error) {
	identity := args.GetIdentity()
	if identity == nil {
		// should never happen
		return nil, structs.ErrPermissionDenied
	}

	if !s.aclsEnabled {
		return acl.ACLsDisabledACL, nil
	}

	if identity.ClientID != "" {
		return acl.ClientACL, nil
	}
	claims := identity.GetClaims()
	if claims != nil {
		return s.resolveClaims(claims)
	}

	// this will include any anonymous token, so this is the last chance to
	// avoid an error
	aclToken := identity.GetACLToken()
	if aclToken != nil {
		return s.resolveACLForToken(aclToken)
	}

	return nil, structs.ErrPermissionDenied
}

// AuthenticateServerOnly returns an ACL object for use *only* with internal
// server-to-server RPCs. This should never be used for RPCs that serve HTTP
// endpoints or accept ACL tokens to avoid confused deputy attacks by making a
// request to a follower that's forwarded.
//
// The returned ACL object is always an acl.ServerACL but in the future this
// could be extended to allow servers to have jurisdiction over specific pools,
// etc.
func (s *Authenticator) AuthenticateServerOnly(ctx RPCContext, args structs.RequestWithIdentity) (*acl.ACL, error) {

	remoteIP, err := ctx.GetRemoteIP() // capture for metrics
	if err != nil {
		s.logger.Error("could not determine remote address", "error", err)
	}

	identity := &structs.AuthenticatedIdentity{RemoteIP: remoteIP}
	defer args.SetIdentity(identity) // always set the identity, even on errors

	// Note: if servers had auth tokens like clients do, we would be able to
	// verify them here and only return the server ACL for actual servers even
	// if mTLS was disabled. Without mTLS, any request can spoof server RPCs.
	// This is known and documented in the Security Model:
	// https://developer.hashicorp.com/nomad/docs/concepts/security#requirements
	if err := verifyTLS(s.verifyTLS.Load(), ctx, s.validServerCertNames, identity); err != nil {
		return nil, err
	}

	return acl.ServerACL, nil
}

// AuthenticateNodeIdentityGenerator is used for RPC endpoints (Node.Register
// and Node.UpdateStatus) that have the potential to generate node identities.
//
// While the Authenticate method serves as a complete general purpose
// authenticator, in some critical cases for identity generation checking, it
// swallows the information needed.
func (s *Authenticator) AuthenticateNodeIdentityGenerator(ctx RPCContext, args structs.RequestWithIdentity) error {

	remoteIP, err := ctx.GetRemoteIP() // capture for metrics
	if err != nil {
		s.logger.Error("could not determine remote address", "error", err)
	}

	identity := &structs.AuthenticatedIdentity{RemoteIP: remoteIP}
	defer args.SetIdentity(identity)

	if err := verifyTLS(s.verifyTLS.Load(), ctx, s.validClientCertNames, identity); err != nil {
		return err
	}

	authToken := args.GetAuthToken()

	// If the auth token is empty, we treat it as an anonymous request. In the
	// event of a node registration, this means the node is not yet registered.
	if authToken == "" {
		identity.ACLToken = structs.AnonymousACLToken
		return nil
	}

	// If the auth token is a UUID, we check whether it's a node secret ID or
	// the leader's ACL token. If it's not a UUID, we assume it's a node
	// identity. Anything outside these cases is not supported and no identity
	// will be set.
	if helper.IsUUID(authToken) {
		if leaderAcl := s.getLeaderACL(); leaderAcl != "" && authToken == leaderAcl {
			identity.ACLToken = structs.LeaderACLToken
		} else {
			node, err := s.getState().NodeBySecretID(nil, authToken)
			if err != nil {
				return fmt.Errorf("could not resolve node")
			}
			if node == nil {
				return structs.ErrPermissionDenied
			}
			identity.ClientID = node.ID
		}
	} else {
		// When verifying a node identity claim, we do not want to swallow the
		// initial error. This is because the caller may want to handle the
		// error type in the case that the JWT is expired.
		claims, err := s.VerifyClaim(authToken)
		if err != nil {
			return err
		}
		if !claims.IsNode() {
			return structs.ErrPermissionDenied
		}
		identity.Claims = claims
	}
	return nil
}

// AuthenticateClientOnly returns an ACL object for use *only* with internal
// RPCs originating from clients (including those forwarded). This should never
// be used for RPCs that serve HTTP endpoints to avoid confused deputy attacks
// by making a request to a client that's forwarded. It should also not be used
// with Node.Register or NodeUpdateStatus, which should use
// AuthenticateNodeIdentityGenerator.
//
// The returned ACL object is always a acl.ClientACL but in the future this
// could be extended to allow clients access only to their own pool and
// associated namespaces, etc.
func (s *Authenticator) AuthenticateClientOnly(ctx RPCContext, args structs.RequestWithIdentity) (*acl.ACL, error) {

	remoteIP, err := ctx.GetRemoteIP() // capture for metrics
	if err != nil {
		s.logger.Error("could not determine remote address", "error", err)
	}

	identity := &structs.AuthenticatedIdentity{RemoteIP: remoteIP}
	defer args.SetIdentity(identity) // always set the identity, even on errors

	if err := verifyTLS(s.verifyTLS.Load(), ctx, s.validClientCertNames, identity); err != nil {
		return nil, err
	}

	authToken := args.GetAuthToken()
	if authToken == "" {
		return nil, structs.ErrPermissionDenied
	}

	// If the auth token is a UUID, we treat it as a node secret ID. Otherwise,
	// we assume it's a node identity claim. Anything outside these cases is not
	// permitted when using this method.
	if helper.IsUUID(authToken) {
		node, err := s.getState().NodeBySecretID(nil, authToken)
		if err != nil {
			return nil, fmt.Errorf("could not resolve node secret: %w", err)
		}
		if node == nil {
			return nil, structs.ErrPermissionDenied
		}
		identity.ClientID = node.ID
	} else {
		claims, err := s.VerifyClaim(authToken)
		if err != nil {
			return nil, err
		}
		if !claims.IsNode() {
			return nil, structs.ErrPermissionDenied
		}
		identity.Claims = claims
	}

	return acl.ClientACL, nil
}

// verifyTLS is a helper function that performs TLS verification, if required,
// given the passed RPCContext and valid names.
//
// It will always set the TLSName on the identity if we are performing
// verification, so callers don't have to worry about setting it themselves.
func verifyTLS(verify bool, ctx RPCContext, validNames []string, identity *structs.AuthenticatedIdentity) error {

	if verify && !ctx.IsStatic() {

		tlsCert := ctx.Certificate()
		if tlsCert == nil {
			return errors.New("missing certificate information")
		}

		// Always set on the identity whether it's valid verifyTLS, so we can
		// capture it for metrics.
		identity.TLSName = tlsCert.Subject.CommonName

		// Perform the certificate validation, using the passed valid names.
		_, err := validateCertificateForNames(tlsCert, validNames)
		if err != nil {
			return err
		}
	}

	return nil
}

// validateCertificateForNames returns true if the certificate is valid for any
// of the given domain names.
func validateCertificateForNames(cert *x509.Certificate, expectedNames []string) (bool, error) {
	if cert == nil {
		return false, nil
	}

	validNames := []string{cert.Subject.CommonName}
	validNames = append(validNames, cert.DNSNames...)

	for _, expectedName := range expectedNames {
		if slices.Contains(validNames, expectedName) {
			return true, nil
		}
	}

	return false, fmt.Errorf("invalid certificate: %s not in expected %s",
		strings.Join(validNames, ", "),
		strings.Join(expectedNames, ", "))

}

// IdentityToACLClaim returns an ACLClaim suitable for checking permissions
func IdentityToACLClaim(ai *structs.AuthenticatedIdentity, store *state.StateStore) *acl.ACLClaim {
	if ai == nil || ai.Claims == nil {
		return nil
	}

	var group string
	alloc, err := store.AllocByID(nil, ai.Claims.AllocationID)
	if err != nil {
		// we should never hit this error, but if we did the caller would get a
		// nil claim and auth will fail
		return nil
	}
	if alloc != nil {
		group = alloc.TaskGroup
	}

	return &acl.ACLClaim{
		Namespace: ai.Claims.Namespace,
		Job:       ai.Claims.JobID,
		Group:     group,
		Task:      ai.Claims.TaskName,
	}
}

// resolveACLForToken resolves an ACL from a token only. It should be used only
// by Variables endpoints, which have additional implicit policies for their
// claims so we can't wrap them up in ResolveACL.
func (s *Authenticator) resolveACLForToken(aclToken *structs.ACLToken) (*acl.ACL, error) {
	snap, err := s.getState().Snapshot()
	if err != nil {
		return nil, err
	}
	return resolveACLFromToken(snap, s.aclCache, aclToken)
}

// ResolveToken is used to translate an ACL Token Secret ID into
// an ACL object, nil if ACLs are disabled, or an error.
//
// TODO(tgross): this is used in lots of places we probably should be calling
// Authenticate + ResolveACL on, because they support HTTP APIs that may be used
// by the Task API
func (s *Authenticator) ResolveToken(secretID string) (*acl.ACL, error) {
	// Fast-path if ACLs are disabled
	if !s.aclsEnabled {
		return acl.ACLsDisabledACL, nil
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "resolveToken"}, time.Now())

	// Check if the secret ID is the leader secret ID, in which case treat it as
	// a management token.
	if leaderAcl := s.getLeaderACL(); leaderAcl != "" && secretID == leaderAcl {
		return acl.ManagementACL, nil
	}

	// Snapshot the state
	snap, err := s.getState().Snapshot()
	if err != nil {
		return nil, err
	}

	// Resolve the ACL
	return resolveTokenFromSnapshotCache(snap, s.aclCache, secretID)
}

// VerifyClaim asserts that the token is valid. If it is for a workload
// identity, it will ensure that the resulting allocation ID belongs to a
// non-terminal allocation. If the token is for a node identity, it will ensure
// the node ID, node pool, node class, and datacenter match the claim.
//
// This should usually not be called by RPC handlers.
func (s *Authenticator) VerifyClaim(token string) (*structs.IdentityClaims, error) {

	claims, err := s.encrypter.VerifyClaim(token)
	if err != nil {
		return nil, err
	}

	if claims.IsWorkload() {
		if err := s.verifyWorkloadIdentityClaim(claims); err != nil {
			return nil, err
		}
		return claims, nil
	}

	if claims.IsNode() {
		if err := s.verifyNodeIdentityClaim(claims); err != nil {
			return nil, err
		}
		return claims, nil
	}

	return claims, nil
}

func (s *Authenticator) verifyWorkloadIdentityClaim(claims *structs.IdentityClaims) error {
	snap, err := s.getState().Snapshot()
	if err != nil {
		return err
	}
	alloc, err := snap.AllocByID(nil, claims.AllocationID)
	if err != nil {
		return err
	}
	if alloc == nil || alloc.Job == nil {
		return fmt.Errorf("allocation does not exist")
	}

	// the claims for terminal allocs are always treated as expired
	if alloc.ClientTerminalStatus() {
		return fmt.Errorf("allocation is terminal")
	}

	return nil
}

func (s *Authenticator) verifyNodeIdentityClaim(claims *structs.IdentityClaims) error {

	snap, err := s.getState().Snapshot()
	if err != nil {
		return err
	}
	node, err := snap.NodeByID(nil, claims.NodeIdentityClaims.NodeID)
	if err != nil {
		return err
	}
	if node == nil {
		return errors.New("node does not exist")
	}

	return nil
}

func (s *Authenticator) resolveClaims(claims *structs.IdentityClaims) (*acl.ACL, error) {

	// Nomad node identity claims only currently map to a client ACL. If we open
	// this up in the future, we will want to modify this section to perform
	// similar work that is done for workload claims.
	if claims.IsNode() {
		return acl.ClientACL, nil
	}

	policies, err := s.ResolvePoliciesForClaims(claims)
	if err != nil {
		return nil, err
	}

	// Compile and cache the ACL object. For many claims this will result in an
	// ACL object with no policies, which can be efficiently cached.
	aclObj, err := structs.CompileACLObject(s.aclCache, policies)
	if err != nil {
		return nil, err
	}
	return aclObj, nil
}

// resolveTokenFromSnapshotCache is used to resolve an ACL object from a
// snapshot of state, using a cache to avoid parsing and ACL construction when
// possible. It is split from resolveToken to simplify testing.
func resolveTokenFromSnapshotCache(snap *state.StateSnapshot, cache *structs.ACLCache[*acl.ACL], secretID string) (*acl.ACL, error) {
	// Lookup the ACL Token
	var token *structs.ACLToken
	var err error

	// Handle anonymous requests
	if secretID == "" {
		token = structs.AnonymousACLToken
	} else {
		token, err = snap.ACLTokenBySecretID(nil, secretID)
		if err != nil {
			return nil, err
		}
		if token == nil {
			return nil, structs.ErrTokenNotFound
		}
		if token.IsExpired(time.Now().UTC()) {
			return nil, structs.ErrTokenExpired
		}
	}

	return resolveACLFromToken(snap, cache, token)

}

func resolveACLFromToken(snap *state.StateSnapshot, cache *structs.ACLCache[*acl.ACL], token *structs.ACLToken) (*acl.ACL, error) {

	// Check if this is a management token
	if token.Type == structs.ACLManagementToken {
		return acl.ManagementACL, nil
	}

	// Store all policies detailed in the token request, this includes the
	// named policies and those referenced within the role link.
	policies := make([]*structs.ACLPolicy, 0, len(token.Policies)+len(token.Roles))

	// Iterate all the token policies and add these to our policy tracking
	// array.
	for _, policyName := range token.Policies {
		policy, err := snap.ACLPolicyByName(nil, policyName)
		if err != nil {
			return nil, err
		}
		if policy == nil {
			// Ignore policies that don't exist, since they don't grant any
			// more privilege.
			continue
		}

		// Add the policy to the tracking array.
		policies = append(policies, policy)
	}

	// Iterate all the token role links, so we can unpack these and identify
	// the ACL policies.
	for _, roleLink := range token.Roles {

		// Any error reading the role means we cannot move forward. We just
		// ignore any roles that have been detailed but are not within our
		// state.
		role, err := snap.GetACLRoleByID(nil, roleLink.ID)
		if err != nil {
			return nil, err
		}
		if role == nil {
			continue
		}

		// Unpack the policies held within the ACL role to form a single list
		// of ACL policies that this token has available.
		for _, policyLink := range role.Policies {
			policy, err := snap.ACLPolicyByName(nil, policyLink.Name)
			if err != nil {
				return nil, err
			}

			// Ignore policies that don't exist, since they don't grant any
			// more privilege.
			if policy == nil {
				continue
			}

			// Add the policy to the tracking array.
			policies = append(policies, policy)
		}
	}

	// Compile and cache the ACL object
	aclObj, err := structs.CompileACLObject(cache, policies)
	if err != nil {
		return nil, err
	}
	return aclObj, nil
}

// resolveSecretToken is used to translate an ACL Token Secret ID into a
// Accessor ID, the anonymous accessor, or an error.
func (s *Authenticator) resolveSecretToken(secretID string) (*structs.ACLToken, error) {

	defer metrics.MeasureSince([]string{"nomad", "acl", "accessorForSecretToken"}, time.Now())

	if !s.aclsEnabled {
		return structs.ACLsDisabledToken, nil
	}
	if secretID == "" {
		return structs.AnonymousACLToken, nil
	}
	if !helper.IsUUID(secretID) {
		return nil, structs.ErrTokenInvalid
	}

	snap, err := s.getState().Snapshot()
	if err != nil {
		return nil, err
	}

	// Lookup the ACL Token
	token, err := snap.ACLTokenBySecretID(nil, secretID)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, structs.ErrTokenNotFound
	}
	if token.IsExpired(time.Now().UTC()) {
		return nil, structs.ErrTokenExpired
	}

	return token, nil
}

func (s *Authenticator) ResolvePoliciesForClaims(claims *structs.IdentityClaims) ([]*structs.ACLPolicy, error) {

	snap, err := s.getState().Snapshot()
	if err != nil {
		return nil, err
	}
	alloc, err := snap.AllocByID(nil, claims.AllocationID)
	if err != nil {
		return nil, err
	}
	if alloc == nil || alloc.Job == nil {
		return nil, fmt.Errorf("allocation does not exist")
	}

	// Find any policies attached to the job
	jobId := alloc.Job.GetIDforWorkloadIdentity()
	iter, err := snap.ACLPolicyByJob(nil, alloc.Namespace, jobId)
	if err != nil {
		return nil, err
	}
	policies := []*structs.ACLPolicy{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		policy := raw.(*structs.ACLPolicy)
		if policy.JobACL == nil {
			continue
		}

		switch {
		case policy.JobACL.Group == "":
			policies = append(policies, policy)
		case policy.JobACL.Group != alloc.TaskGroup:
			continue // don't bother checking task
		case policy.JobACL.Task == "":
			policies = append(policies, policy)
		case policy.JobACL.Task == claims.TaskName:
			policies = append(policies, policy)
		}
	}

	iter, err = snap.ACLPolicyByNamespace(nil, alloc.Namespace)
	if err != nil {
		return nil, err
	}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}

		policy := raw.(*structs.ACLPolicy)
		if policy.JobACL == nil {
			continue
		}

		policies = append(policies, policy)
	}

	return policies, nil
}
