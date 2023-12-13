// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package logging

import (
	"context"
	"path"
	"time"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryClientInterceptor returns a new unary client interceptor that logs the execution of gRPC calls.
func UnaryClientInterceptor(logger hclog.Logger, opts ...Option) grpc.UnaryClientInterceptor {
	o := evaluateClientOpt(opts)
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		startTime := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		emitClientLog(logger, o, method, startTime, err, "finished client unary call")
		return err
	}
}

// StreamClientInterceptor returns a new streaming client interceptor that logs the execution of gRPC calls.
func StreamClientInterceptor(logger hclog.Logger, opts ...Option) grpc.StreamClientInterceptor {
	o := evaluateClientOpt(opts)
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		startTime := time.Now()
		clientStream, err := streamer(ctx, desc, cc, method, opts...)
		emitClientLog(logger, o, method, startTime, err, "finished client streaming call")
		return clientStream, err
	}
}

func emitClientLog(logger hclog.Logger, o *options, fullMethodString string, startTime time.Time, err error, msg string) {
	code := status.Code(err)
	logLevel := o.levelFunc(code)
	reqDuration := time.Since(startTime)
	service := path.Dir(fullMethodString)[1:]
	method := path.Base(fullMethodString)
	logger.Log(logLevel, msg, "grpc.code", code, "duration", reqDuration, "grpc.service", service, "grpc.method", method)
}
