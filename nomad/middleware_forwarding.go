package nomad

func forwardingMiddleware[
	Req RPCRequest, Resp any](method string) MiddlewareFunc[Req, Resp] {
	return func(
		ctx *RPCHandlerContext,
		args Req,
		reply Resp,
		handler RPCHandlerFunc[Req, Resp],
	) error {
		if done, err := ctx.srv.forward(method, args, args, reply); done {
			return err
		}
		return handler(ctx, args, reply)
	}
}

func forwardingAuthoritativeRegionMiddleware[
	Req RPCRequest, Resp any](method, region string) MiddlewareFunc[Req, Resp] {
	return func(
		ctx *RPCHandlerContext,
		args Req,
		reply Resp,
		handler RPCHandlerFunc[Req, Resp],
	) error {
		args.SetRegion(region)
		if done, err := ctx.srv.forward(method, args, args, reply); done {
			return err
		}
		return handler(ctx, args, reply)
	}
}
