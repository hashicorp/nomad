// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package grpcutils

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/plugins/base/structs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HandleReqCtxGrpcErr is used to handle a non io.EOF error in a GRPC request
// where a user supplied context is used. It handles detecting if the plugin has
// shutdown via the passeed pluginCtx. The parameters are:
// - err: the error returned from the streaming RPC
// - reqCtx: the user context passed to the request
// - pluginCtx: the plugins done ctx used to detect the plugin dying
//
// The return values are:
// - ErrPluginShutdown if the error is because the plugin shutdown
// - context.Canceled if the reqCtx is canceled
// - The original error
func HandleReqCtxGrpcErr(err error, reqCtx, pluginCtx context.Context) error {
	if err == nil {
		return nil
	}

	// Determine if the error is because the plugin shutdown
	if errStatus, ok := status.FromError(err); ok &&
		(errStatus.Code() == codes.Unavailable || errStatus.Code() == codes.Canceled) {
		// Potentially wait a little before returning an error so we can detect
		// the exit
		select {
		case <-pluginCtx.Done():
			err = structs.ErrPluginShutdown
		case <-reqCtx.Done():
			err = reqCtx.Err()

			// There is no guarantee that the select will choose the
			// doneCtx first so we have to double check
			select {
			case <-pluginCtx.Done():
				err = structs.ErrPluginShutdown
			default:
			}
		case <-time.After(3 * time.Second):
			// Its okay to wait a while since the connection isn't available and
			// on local host it is likely shutting down. It is not expected for
			// this to ever reach even close to 3 seconds.
		}

		// It is an error we don't know how to handle, so return it
		return err
	}

	// Context was cancelled
	if errStatus := status.FromContextError(reqCtx.Err()); errStatus.Code() == codes.Canceled {
		return context.Canceled
	}

	return err
}

// HandleGrpcErr is used to handle errors made to a remote gRPC plugin. It
// handles detecting if the plugin has shutdown via the passeed pluginCtx. The
// parameters are:
// - err: the error returned from the streaming RPC
// - pluginCtx: the plugins done ctx used to detect the plugin dying
//
// The return values are:
// - ErrPluginShutdown if the error is because the plugin shutdown
// - The original error
func HandleGrpcErr(err error, pluginCtx context.Context) error {
	if err == nil {
		return nil
	}

	if errStatus := status.FromContextError(pluginCtx.Err()); errStatus.Code() == codes.Canceled {
		// See if the plugin shutdown
		select {
		case <-pluginCtx.Done():
			err = structs.ErrPluginShutdown
		default:
		}
	}

	// Determine if the error is because the plugin shutdown
	if errStatus, ok := status.FromError(err); ok && errStatus.Code() == codes.Unavailable {
		// Potentially wait a little before returning an error so we can detect
		// the exit
		select {
		case <-pluginCtx.Done():
			err = structs.ErrPluginShutdown
		case <-time.After(3 * time.Second):
			// Its okay to wait a while since the connection isn't available and
			// on local host it is likely shutting down. It is not expected for
			// this to ever reach even close to 3 seconds.
		}

		// It is an error we don't know how to handle, so return it
		return err
	}

	return err
}
