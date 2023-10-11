// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
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
	aclsEnabled  bool
	tlsEnabled   bool
	logger       hclog.Logger
	getState     StateGetter
	getLeaderACL LeaderACLGetter
	region       string

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
	TLSEnabled     bool
	Region         string
	Encrypter      Encrypter
}

func NewAuthenticator(cfg *AuthenticatorConfig) *Authenticator {
	return &Authenticator{
		aclsEnabled:  cfg.AclsEnabled,
		tlsEnabled:   cfg.TLSEnabled,
		logger:       cfg.Logger.With("auth"),
		getState:     cfg.StateFn,
		getLeaderACL: cfg.GetLeaderACLFn,
		region:       cfg.Region,
		aclCache:     structs.NewACLCache[*acl.ACL](aclCacheSize),
		encrypter:    cfg.Encrypter,
	}
}

// Authenticate extracts an AuthenticatedIdentity from the request context or
// provided token and sets the identity on the request. The caller can extract
// an acl.ACL, WorkloadIdentity, or other identifying tokens to use for
// authorization. Keeping these fields independent rather than merging them into
// an ephemeral ACLToken makes the original of the credential clear to RPC
// handlers, who may have different behavior for internal vs external origins.
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
	aclToken, err := s.ResolveSecretToken(secretID)

	switch {
	case err == nil:
		// If ACLs are disabled or we have a non-anonymous token, return that.
		if aclToken == nil || aclToken != structs.AnonymousACLToken {
			args.SetIdentity(&structs.AuthenticatedIdentity{ACLToken: aclToken})
			return nil
		}

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
	if ctx == nil {
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

// ResolveACL is an authentication wrapper which handles resolving both ACL
// tokens and Workload Identities. If both are provided the ACL token is
// preferred, but it is best for the RPC caller to only include the credentials
// for the identity they intend the operation to be performed with.
func (s *Authenticator) ResolveACL(args structs.RequestWithIdentity) (*acl.ACL, error) {
	identity := args.GetIdentity()
	if !s.aclsEnabled || identity == nil {
		return nil, nil
	}
	aclToken := identity.GetACLToken()
	if aclToken != nil {
		return s.ResolveACLForToken(aclToken)
	}
	claims := identity.GetClaims()
	if claims != nil {
		return s.ResolveClaims(claims)
	}
	return nil, nil
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

	if s.tlsEnabled && !ctx.IsStatic() {
		tlsCert := ctx.Certificate()
		if tlsCert == nil {
			return nil, errors.New("missing certificate information")
		}

		// set on the identity whether or not its valid for server RPC, so we
		// can capture it for metrics
		identity.TLSName = tlsCert.Subject.CommonName

		expected := "server." + s.region + ".nomad"
		_, err := validateCertificateForName(tlsCert, expected)
		if err != nil {
			return nil, err
		}
		return acl.ServerACL, nil
	}

	// Note: if servers had auth tokens like clients do, we would be able to
	// verify them here and only return the server ACL for actual servers even
	// if mTLS was disabled. Without mTLS, any request can spoof server RPCs.
	// This is known and documented in the Security Model:
	// https://developer.hashicorp.com/nomad/docs/concepts/security#requirements
	return acl.ServerACL, nil
}

// AuthenticateClientOnly returns an ACL object for use *only* with internal
// RPCs originating from clients (including those forwarded). This should never
// be used for RPCs that serve HTTP endpoints to avoid confused deputy attacks
// by making a request to a client that's forwarded. It should also not be used
// with Node.Register, which should use AuthenticateClientTOFU
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

	if s.tlsEnabled && !ctx.IsStatic() {
		tlsCert := ctx.Certificate()
		if tlsCert == nil {
			return nil, errors.New("missing certificate information")
		}

		// set on the identity whether or not its valid for server RPC, so we
		// can capture it for metrics
		identity.TLSName = tlsCert.Subject.CommonName

		expected := fmt.Sprintf("client.%s.nomad", s.region)
		_, err := validateCertificateForName(tlsCert, expected)
		if err != nil {
			expected := fmt.Sprintf("server.%s.nomad", s.region)
			_, err := validateCertificateForName(tlsCert, expected)
			if err != nil {
				return nil, err
			}
		}
	}

	secretID := args.GetAuthToken()
	if secretID == "" {
		return nil, structs.ErrPermissionDenied
	}

	// Otherwise, see if the secret ID belongs to a node. We should
	// reach this point only on first connection.
	node, err := s.getState().NodeBySecretID(nil, secretID)
	if err != nil {
		// this is a go-memdb error; shouldn't happen
		return nil, fmt.Errorf("could not resolve node secret: %w", err)
	}
	if node == nil {
		return nil, structs.ErrPermissionDenied
	}
	identity.ClientID = node.ID
	return acl.ClientACL, nil
}

// AuthenticateClientOnlyLegacy is a version of AuthenticateClientOnly that's
// used by a few older RPCs that did not properly enforce node secrets.
// COMPAT(1.8.0): In Nomad 1.6.0 we starting sending those node secrets, so we
// can remove this in Nomad 1.8.0.
func (s *Authenticator) AuthenticateClientOnlyLegacy(ctx RPCContext, args structs.RequestWithIdentity) (*acl.ACL, error) {

	remoteIP, err := ctx.GetRemoteIP() // capture for metrics
	if err != nil {
		s.logger.Error("could not determine remote address", "error", err)
	}

	identity := &structs.AuthenticatedIdentity{RemoteIP: remoteIP}
	defer args.SetIdentity(identity) // always set the identity, even on errors

	if s.tlsEnabled && !ctx.IsStatic() {
		tlsCert := ctx.Certificate()
		if tlsCert == nil {
			return nil, errors.New("missing certificate information")
		}

		// set on the identity whether or not its valid for server RPC, so we
		// can capture it for metrics
		identity.TLSName = tlsCert.Subject.CommonName

		expected := fmt.Sprintf("client.%s.nomad", s.region)
		_, err := validateCertificateForName(tlsCert, expected)
		if err != nil {
			expected := fmt.Sprintf("server.%s.nomad", s.region)
			_, err := validateCertificateForName(tlsCert, expected)
			if err != nil {
				return nil, err
			}
		}
	}

	return acl.ClientACL, nil
}

// validateCertificateForName returns true if the certificate is valid
// for the given domain name.
func validateCertificateForName(cert *x509.Certificate, expectedName string) (bool, error) {
	if cert == nil {
		return false, nil
	}

	validNames := []string{cert.Subject.CommonName}
	validNames = append(validNames, cert.DNSNames...)
	for _, valid := range validNames {
		if expectedName == valid {
			return true, nil
		}
	}

	return false, fmt.Errorf("invalid certificate, %s not in %s",
		expectedName, strings.Join(validNames, ","))
}

// ResolveACLForToken resolves an ACL from a token only. It should be used only
// by Variables endpoints, which have additional implicit policies for their
// claims so we can't wrap them up in ResolveACL.
//
// TODO: figure out a way to the Variables endpoint implicit policies baked into
// their acl.ACL object so that we can avoid using this method.
func (s *Authenticator) ResolveACLForToken(aclToken *structs.ACLToken) (*acl.ACL, error) {
	if !s.aclsEnabled {
		return nil, nil
	}
	snap, err := s.getState().Snapshot()
	if err != nil {
		return nil, err
	}
	return resolveACLFromToken(snap, s.aclCache, aclToken)
}

// ResolveToken is used to translate an ACL Token Secret ID into
// an ACL object, nil if ACLs are disabled, or an error.
func (s *Authenticator) ResolveToken(secretID string) (*acl.ACL, error) {
	// Fast-path if ACLs are disabled
	if !s.aclsEnabled {
		return nil, nil
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

// VerifyClaim asserts that the token is valid and that the resulting
// allocation ID belongs to a non-terminal allocation
func (s *Authenticator) VerifyClaim(token string) (*structs.IdentityClaims, error) {

	claims, err := s.encrypter.VerifyClaim(token)
	if err != nil {
		return nil, err
	}
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

	// the claims for terminal allocs are always treated as expired
	if alloc.TerminalStatus() {
		return nil, fmt.Errorf("allocation is terminal")
	}

	return claims, nil
}

func (s *Authenticator) ResolveClaims(claims *structs.IdentityClaims) (*acl.ACL, error) {

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

// ResolveSecretToken is used to translate an ACL Token Secret ID into
// an ACLToken object, nil if ACLs are disabled, or an error.
func (s *Authenticator) ResolveSecretToken(secretID string) (*structs.ACLToken, error) {
	// TODO(Drew) Look into using ACLObject cache or create a separate cache

	// Fast-path if ACLs are disabled
	if !s.aclsEnabled {
		return nil, nil
	}
	defer metrics.MeasureSince([]string{"nomad", "acl", "resolveSecretToken"}, time.Now())

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
	jobId := alloc.Job.ID
	if alloc.Job.ParentID != "" {
		jobId = alloc.Job.ParentID
	}
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

	return policies, nil
}
