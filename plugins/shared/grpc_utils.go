package shared

import (
	"context"
	"time"

	"github.com/hashicorp/nomad/plugins/base"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HandleStreamErr is used to handle a non io.EOF error in a stream. It handles
// detecting if the plugin has shutdown via the passeed pluginCtx. The
// parameters are:
// - err: the error returned from the streaming RPC
// - reqCtx: the context passed to the streaming request
// - pluginCtx: the plugins done ctx used to detect the plugin dying
//
// The return values are:
// - base.ErrPluginShutdown if the error is because the plugin shutdown
// - context.Canceled if the reqCtx is canceled
// - The original error
func HandleStreamErr(err error, reqCtx, pluginCtx context.Context) error {
	if err == nil {
		return nil
	}

	// Determine if the error is because the plugin shutdown
	if errStatus, ok := status.FromError(err); ok && errStatus.Code() == codes.Unavailable {
		// Potentially wait a little before returning an error so we can detect
		// the exit
		select {
		case <-pluginCtx.Done():
			err = base.ErrPluginShutdown
		case <-reqCtx.Done():
			err = reqCtx.Err()

			// There is no guarantee that the select will choose the
			// doneCtx first so we have to double check
			select {
			case <-pluginCtx.Done():
				err = base.ErrPluginShutdown
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
