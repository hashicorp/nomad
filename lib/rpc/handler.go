package rpc

import (
	"context"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

type HandlerContext struct {
	rpcCtx   *RPCContext
	srv      Server
	ctx      context.Context
	aclObj   *acl.ACL
	identity *structs.AuthenticatedIdentity
}
