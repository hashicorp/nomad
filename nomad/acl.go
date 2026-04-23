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

func (s *Server) AuthenticateServerOnly(ctx *RPCContext, args structs.RequestWithIdentity) (*acl.ACL, error) {
	return s.auth.AuthenticateServerOnly(ctx, args)
}

func (s *Server) AuthenticateNodeIdentityGenerator(ctx *RPCContext, args structs.RequestWithIdentity) error {
	return s.auth.AuthenticateNodeIdentityGenerator(ctx, args)
}

func (s *Server) AuthenticateClientOnly(ctx *RPCContext, args structs.RequestWithIdentity) (*acl.ACL, error) {
	return s.auth.AuthenticateClientOnly(ctx, args)
}

func (s *Server) ResolveAuthorizedClientNodePoolByNodeID(aclObj *acl.ACL, nodeID string) (string, error) {
	return s.auth.ResolveAuthorizedClientNodePoolByNodeID(aclObj, nodeID)
}

func (s *Server) AuthorizeClientAllocation(
	aclObj *acl.ACL,
	alloc *structs.Allocation,
	allowNsOp func(*acl.ACL, string) bool,
) error {
	return s.auth.AuthorizeClientAllocation(aclObj, alloc, allowNsOp)
}

func (s *Server) ResolveAuthorizedClientNodePoolByServiceRegistrationID(
	aclObj *acl.ACL,
	namespace, id string,
) (string, error) {
	return s.auth.ResolveAuthorizedClientNodePoolByServiceRegistrationID(aclObj, namespace, id)
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
