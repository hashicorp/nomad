package device

import (
	"context"
	"io"
	"time"

	"github.com/LK4D4/joincontext"
	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device/proto"
	netctx "golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// devicePluginClient implements the client side of a remote device plugin, using
// gRPC to communicate to the remote plugin.
type devicePluginClient struct {
	// basePluginClient is embedded to give access to the base plugin methods.
	*base.BasePluginClient

	client proto.DevicePluginClient

	// doneCtx is closed when the plugin exits
	doneCtx context.Context
}

// Fingerprint is used to retrieve the set of devices and their health from the
// device plugin. An error may be immediately returned if the fingerprint call
// could not be made or as part of the streaming response. If the context is
// cancelled, the error will be propogated.
func (d *devicePluginClient) Fingerprint(ctx context.Context) (<-chan *FingerprintResponse, error) {
	// Join the passed context and the shutdown context
	ctx, _ = joincontext.Join(ctx, d.doneCtx)

	var req proto.FingerprintRequest
	stream, err := d.client.Fingerprint(ctx, &req)
	if err != nil {
		return nil, err
	}

	out := make(chan *FingerprintResponse, 1)
	go d.handleFingerprint(ctx, stream, out)
	return out, nil
}

// handleFingerprint should be launched in a goroutine and handles converting
// the gRPC stream to a channel. Exits either when context is cancelled or the
// stream has an error.
func (d *devicePluginClient) handleFingerprint(
	ctx netctx.Context,
	stream proto.DevicePlugin_FingerprintClient,
	out chan *FingerprintResponse) {

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				out <- &FingerprintResponse{
					Error: d.handleStreamErr(err, ctx),
				}
			}

			// End the stream
			close(out)
			return
		}

		// Send the response
		out <- &FingerprintResponse{
			Devices: convertProtoDeviceGroups(resp.GetDeviceGroup()),
		}
	}
}

func (d *devicePluginClient) Reserve(deviceIDs []string) (*ContainerReservation, error) {
	// Build the request
	req := &proto.ReserveRequest{
		DeviceIds: deviceIDs,
	}

	// Make the request
	resp, err := d.client.Reserve(context.Background(), req)
	if err != nil {
		return nil, err
	}

	// Convert the response
	out := convertProtoContainerReservation(resp.GetContainerRes())
	return out, nil
}

// Stats is used to retrieve device statistics from the device plugin. An error
// may be immediately returned if the stats call could not be made or as part of
// the streaming response. If the context is cancelled, the error will be
// propogated.
func (d *devicePluginClient) Stats(ctx context.Context, interval time.Duration) (<-chan *StatsResponse, error) {
	// Join the passed context and the shutdown context
	ctx, _ = joincontext.Join(ctx, d.doneCtx)

	req := proto.StatsRequest{
		CollectionInterval: ptypes.DurationProto(interval),
	}
	stream, err := d.client.Stats(ctx, &req)
	if err != nil {
		return nil, err
	}

	out := make(chan *StatsResponse, 1)
	go d.handleStats(ctx, stream, out)
	return out, nil
}

// handleStats should be launched in a goroutine and handles converting
// the gRPC stream to a channel. Exits either when context is cancelled or the
// stream has an error.
func (d *devicePluginClient) handleStats(
	ctx netctx.Context,
	stream proto.DevicePlugin_StatsClient,
	out chan *StatsResponse) {

	for {
		resp, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				out <- &StatsResponse{
					Error: d.handleStreamErr(err, ctx),
				}
			}

			// End the stream
			close(out)
			return
		}

		// Send the response
		out <- &StatsResponse{
			Groups: convertProtoDeviceGroupsStats(resp.GetGroups()),
		}
	}
}

// handleStreamErr is used to handle a non io.EOF error in a stream. It handles
// detecting if the plugin has shutdown
func (d *devicePluginClient) handleStreamErr(err error, ctx context.Context) error {
	if err == nil {
		return nil
	}

	// Determine if the error is because the plugin shutdown
	if errStatus, ok := status.FromError(err); ok && errStatus.Code() == codes.Unavailable {
		// Potentially wait a little before returning an error so we can detect
		// the exit
		select {
		case <-d.doneCtx.Done():
			err = base.ErrPluginShutdown
		case <-ctx.Done():
			err = ctx.Err()

			// There is no guarantee that the select will choose the
			// doneCtx first so we have to double check
			select {
			case <-d.doneCtx.Done():
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
	if errStatus := status.FromContextError(ctx.Err()); errStatus.Code() == codes.Canceled {
		return context.Canceled
	}

	return err
}
