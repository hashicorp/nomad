package nomad

import (
	"time"

	metrics "github.com/armon/go-metrics"
)

func metricsMiddleware[Req RPCRequest, Resp any](labels []string) MiddlewareFunc[Req, Resp] {
	return func(
		ctx *RPCHandlerContext,
		args Req,
		reply Resp,
		handler RPCHandlerFunc[Req, Resp],
	) error {
		defer metrics.MeasureSince(labels, time.Now())
		return handler(ctx, args, reply)
	}
}
