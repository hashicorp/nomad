// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (srv *Server) Authenticate(ctx *RPCContext, args structs.RequestWithIdentity) error {
	return srv.auth.Authenticate(ctx, args)
}

func (srv *Server) AuthenticateServerOnly(ctx *RPCContext, args structs.RequestWithIdentity) (*acl.ACL, error) {
	return srv.auth.AuthenticateServerOnly(ctx, args)
}

func (srv *Server) AuthenticateClientOnly(ctx *RPCContext, args structs.RequestWithIdentity) (*acl.ACL, error) {
	return srv.auth.AuthenticateClientOnly(ctx, args)
}

func (srv *Server) AuthenticateClientOnlyLegacy(ctx *RPCContext, args structs.RequestWithIdentity) (*acl.ACL, error) {
	return srv.auth.AuthenticateClientOnlyLegacy(ctx, args)
}

func (srv *Server) ResolveACL(args structs.RequestWithIdentity) (*acl.ACL, error) {
	return srv.auth.ResolveACL(args)
}

func (srv *Server) VerifyClaim(token string) (*structs.IdentityClaims, error) {
	return srv.auth.VerifyClaim(token)
}

func (srv *Server) ResolveToken(secretID string) (*acl.ACL, error) {
	return srv.auth.ResolveToken(secretID)
}

func (srv *Server) ResolvePoliciesForClaims(claims *structs.IdentityClaims) ([]*structs.ACLPolicy, error) {
	return srv.auth.ResolvePoliciesForClaims(claims)
}

func (srv *Server) ResolveSecretToken(secretID string) (*structs.ACLToken, error) {
	return srv.auth.ResolveSecretToken(secretID)
}
