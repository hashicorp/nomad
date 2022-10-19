package nomad

import (
	"context"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

type RPCRequest interface {
	RequestToken() string
	RequestNamespace() string
	SetRegion(string)
	SetIdentity(*structs.AuthenticatedIdentity)
	GetIdentity() *structs.AuthenticatedIdentity

	structs.RPCInfo
}

type RPCHandlerContext struct {
	rpcCtx   *RPCContext
	srv      *Server
	ctx      context.Context
	aclObj   *acl.ACL
	identity *structs.AuthenticatedIdentity
}

func NewRPCHandlerContext(ctx *RPCContext, srv *Server) *RPCHandlerContext {
	return &RPCHandlerContext{
		rpcCtx: ctx,
		srv:    srv,
		ctx:    context.TODO(),
	}
}

// RPCHandlerFunc is a handler that returns an error and is generic
// over a request and response object.
type RPCHandlerFunc[Req RPCRequest, Resp any] func(*RPCHandlerContext, Req, Resp) error

// MiddlewareFunc is a function that returns an error and is generic
// over a request and response object, and which calls the provided
// handler.
type MiddlewareFunc[Req RPCRequest, Resp any] func(*RPCHandlerContext, Req, Resp, RPCHandlerFunc[Req, Resp]) error

// Wrap takes a middleware and RPC handler and constructs a new RPC
// handler that when called will call a middleware that in turn calls
// the original handler.
func Wrap[Req RPCRequest, Resp any](middleware MiddlewareFunc[Req, Resp], handler RPCHandlerFunc[Req, Resp]) RPCHandlerFunc[Req, Resp] {
	return func(ctx *RPCHandlerContext, args Req, reply Resp) error {
		return middleware(ctx, args, reply, handler)
	}
}

// Chain creates one middleware function out of a list of middleware
// functions, by calling wrap on each of them to return a RPC handler
// that, when called, calls a middleware that calls a middleware, etc,
// until it reaches the inner handler.
func Chain[Req RPCRequest, Resp any](middlewares ...MiddlewareFunc[Req, Resp]) MiddlewareFunc[Req, Resp] {
	return func(ctx *RPCHandlerContext, args Req, reply Resp, handlerFn RPCHandlerFunc[Req, Resp]) error {
		wrappedHandler := handlerFn
		for i := len(middlewares) - 1; i >= 0; i-- {
			wrappedHandler = Wrap(middlewares[i], wrappedHandler)
		}
		return wrappedHandler(ctx, args, reply)
	}
}
