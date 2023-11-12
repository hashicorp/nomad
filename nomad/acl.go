// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *Server) Authenticate(ctx *RPCContext, args structs.RequestWithIdentity) error {
	return s.auth.Authenticate(ctx, args)
}

func (s *Server) AuthenticateServerOnly(ctx *RPCContext, args structs.RequestWithIdentity) (*acl.ACL, error) {
	return s.auth.AuthenticateServerOnly(ctx, args)
}

func (s *Server) AuthenticateClientOnly(ctx *RPCContext, args structs.RequestWithIdentity) (*acl.ACL, error) {
	return s.auth.AuthenticateClientOnly(ctx, args)
}

func (s *Server) AuthenticateClientOnlyLegacy(ctx *RPCContext, args structs.RequestWithIdentity) (*acl.ACL, error) {
	return s.auth.AuthenticateClientOnlyLegacy(ctx, args)
}

func (s *Server) ResolveACL(args structs.RequestWithIdentity) (*acl.ACL, error) {
	return s.auth.ResolveACL(args)
}

func (s *Server) VerifyClaim(token string) (*structs.IdentityClaims, error) {
	return s.auth.VerifyClaim(token)
}

func (s *Server) ResolveToken(secretID string) (*acl.ACL, error) {
	return s.auth.ResolveToken(secretID)
}

func (s *Server) ResolvePoliciesForClaims(claims *structs.IdentityClaims) ([]*structs.ACLPolicy, error) {
	return s.auth.ResolvePoliciesForClaims(claims)
}
