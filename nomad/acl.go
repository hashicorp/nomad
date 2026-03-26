// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (s *Server) Authenticate(ctx *RPCContext, args structs.RequestWithIdentity) error {
	return s.auth.Authenticate(ctx, args)
}

func (s *Server) AllowClientOpInCallerPool(ctx *RPCContext, aclObj *acl.ACL, args structs.RequestWithIdentity) error {
	if aclObj == nil {
		return structs.ErrPermissionDenied
	}

	pool, err := resolveCallerNodePool(s, ctx, args.GetIdentity())
	if err != nil {
		return err
	}

	if !aclObj.AllowClientOp(pool) {
		return structs.ErrPermissionDenied
	}

	return nil
}

func (s *Server) AuthenticateServerOnly(ctx *RPCContext, args structs.RequestWithIdentity) (*acl.ACL, error) {
	return s.auth.AuthenticateServerOnly(ctx, args)
}

func (s *Server) AuthenticateNodeIdentityGenerator(ctx *RPCContext, args structs.RequestWithIdentity) error {
	return s.auth.AuthenticateNodeIdentityGenerator(ctx, args)
}

func (s *Server) AuthenticateClientOnly(ctx *RPCContext, args structs.RequestWithIdentity) (*acl.ACL, error) {
	return s.auth.AuthenticateClientOnly(ctx, args)
}

func (s *Server) ResolveACL(args structs.RequestWithIdentity) (*acl.ACL, error) {
	return s.auth.ResolveACL(args)
}

func (s *Server) VerifyClaim(token string) (*structs.IdentityClaims, error) {
	return s.auth.VerifyClaim(token)
}

func (s *Server) ResolvePoliciesForClaims(claims *structs.IdentityClaims) ([]*structs.ACLPolicy, error) {
	return s.auth.ResolvePoliciesForClaims(claims)
}
