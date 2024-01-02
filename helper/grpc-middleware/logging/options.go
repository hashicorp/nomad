// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package logging

import (
	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc/codes"
)

type options struct {
	levelFunc CodeToLevel
}

var defaultOptions = &options{}

type Option func(*options)

func evaluateClientOpt(opts []Option) *options {
	optCopy := &options{}
	*optCopy = *defaultOptions
	optCopy.levelFunc = DefaultCodeToLevel
	for _, o := range opts {
		o(optCopy)
	}
	return optCopy
}

func WithStatusCodeToLevelFunc(fn CodeToLevel) Option {
	return func(opts *options) {
		opts.levelFunc = fn
	}
}

// CodeToLevel function defines the mapping between gRPC return codes and hclog level.
type CodeToLevel func(code codes.Code) hclog.Level

func DefaultCodeToLevel(code codes.Code) hclog.Level {
	switch code {
	// Trace Logs -- Useful for Nomad developers but not necessarily always wanted
	case codes.OK:
		return hclog.Trace

	// Debug logs
	case codes.Canceled:
		return hclog.Debug
	case codes.InvalidArgument:
		return hclog.Debug
	case codes.ResourceExhausted:
		return hclog.Debug
	case codes.FailedPrecondition:
		return hclog.Debug
	case codes.Aborted:
		return hclog.Debug
	case codes.OutOfRange:
		return hclog.Debug
	case codes.NotFound:
		return hclog.Debug
	case codes.AlreadyExists:
		return hclog.Debug

	// Info Logs - More curious/interesting than debug, but not necessarily critical
	case codes.Unknown:
		return hclog.Info
	case codes.DeadlineExceeded:
		return hclog.Info
	case codes.PermissionDenied:
		return hclog.Info
	case codes.Unauthenticated:
		// unauthenticated requests are probably usually fine?
		return hclog.Info
	case codes.Unavailable:
		// unavailable errors indicate the upstream is not currently available. Info
		// because I would guess these are usually transient and will be handled by
		// retry mechanisms before being served as a higher level warning.
		return hclog.Info

	// Warn Logs - These are almost definitely bad in most cases - usually because
	//             the upstream is broken.
	case codes.Unimplemented:
		return hclog.Warn
	case codes.Internal:
		return hclog.Warn
	case codes.DataLoss:
		return hclog.Warn

	default:
		// Codes that aren't implemented as part of a CodeToLevel case are probably
		// unknown and should be surfaced.
		return hclog.Info
	}
}
