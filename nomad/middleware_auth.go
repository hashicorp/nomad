package nomad

import (
	"bytes"
	"fmt"
	"net"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

// authenticationMiddleware attaches an AuthenticatedIdentity to the request, so
// we know whether the request comes from an ACLToken, Workload Identity, client
// node, or server node.
func authenticationMiddleware[
	Req RPCRequest, Resp any]() MiddlewareFunc[Req, Resp] {
	return func(
		ctx *RPCHandlerContext,
		args Req,
		reply Resp,
		handler RPCHandlerFunc[Req, Resp],
	) error {
		identity, err := resolveIdentity(ctx, args.RequestToken())
		if err != nil {
			return err
		}
		args.SetIdentity(identity)
		return handler(ctx, args, reply)
	}
}

// resolveIdentity extracts an AuthenticatedIdentity from the request context or
// provided token.
func resolveIdentity(ctx *RPCHandlerContext, secretID string) (*structs.AuthenticatedIdentity, error) {

	// get the user ACLToken or anonymous token
	aclToken, err := ctx.srv.ResolveSecretToken(secretID)

	identity := &structs.AuthenticatedIdentity{ACLToken: aclToken}

	switch err {
	case structs.ErrTokenNotFound:
		if secretID == ctx.srv.getLeaderAcl() {
			identity.ServerID = ctx.srv.config.NodeID
			identity.LeaderACL = secretID
			return identity, nil
		}
		node, err := ctx.srv.State().NodeBySecretID(nil, secretID)
		if err != nil {
			// this is a go-memdb error; shouldn't happen
			return nil, fmt.Errorf("could not resolve node secret: %v", err)
		}
		if node != nil {
			identity.ClientID = node.ID
			return identity, nil
		}
		claims, err := ctx.srv.VerifyClaim(secretID)
		if err == nil {
			// unlike the state queries, errors here are invalid tokens
			identity.Claims = claims
			return identity, nil
		}

	case nil:
		if aclToken != nil && aclToken.AccessorID != structs.AnonymousACLToken.AccessorID {
			return identity, nil
		}

	default: // any other error
		return nil, fmt.Errorf("could not resolve user: %v", err)

	}

	// At this point we have an anonymous token or an invalid one; fall back to
	// the connection NodeID or finding the server ID by raft peer or serf address

	if ctx.rpcCtx.NodeID != "" {
		identity.ServerID = ctx.rpcCtx.NodeID
		return identity, nil
	}

	var remoteAddr *net.TCPAddr

	if ctx.rpcCtx.Session != nil {
		remoteAddr = ctx.rpcCtx.Session.RemoteAddr().(*net.TCPAddr)
	}
	if ctx.rpcCtx.Conn != nil {
		remoteAddr = ctx.rpcCtx.Conn.RemoteAddr().(*net.TCPAddr)
	}
	if remoteAddr != nil {

		// check in raft first
		future := ctx.srv.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			return nil, err
		}
		for _, server := range future.Configuration().Servers {
			if remoteAddr.String() == string(server.Address) {
				identity.ServerID = string(server.ID)
				return identity, nil
			}
		}

		// fallback to serf
		members := ctx.srv.Members()
		for _, member := range members {
			if bytes.Equal(member.Addr, remoteAddr.IP) {
				identity.ServerID = member.Name
				return identity, nil
			}
		}
	}

	return identity, nil
}

func authorizationMiddleware[
	Req RPCRequest, Resp any](allowFunc func(*acl.ACL) bool) MiddlewareFunc[Req, Resp] {
	return func(
		ctx *RPCHandlerContext,
		args Req,
		reply Resp,
		handler RPCHandlerFunc[Req, Resp],
	) error {

		if !ctx.srv.config.ACLEnabled {
			return handler(ctx, args, reply)
		}

		aclObj, err := ctx.srv.ResolveToken(args.RequestToken())
		if err != nil {
			return err
		}
		if aclObj != nil {
			ctx.aclObj = aclObj
		}
		if !allowFunc(aclObj) {
			return structs.ErrPermissionDenied
		}

		return handler(ctx, args, reply)
	}
}

func authorizationManagementOnlyMiddleware[
	Req RPCRequest, Resp any]() MiddlewareFunc[Req, Resp] {
	return func(
		ctx *RPCHandlerContext,
		args Req,
		reply Resp,
		handler RPCHandlerFunc[Req, Resp],
	) error {

		if !ctx.srv.config.ACLEnabled {
			return handler(ctx, args, reply)
		}

		if aclObj, err := ctx.srv.ResolveToken(args.RequestToken()); err != nil {
			return err
		} else if aclObj != nil && !aclObj.IsManagement() {
			return structs.ErrPermissionDenied
		}

		return handler(ctx, args, reply)
	}
}
