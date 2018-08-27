package device

import (
	"context"
	"io"

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
}

// Fingerprint is used to retrieve the set of devices and their health from the
// device plugin. An error may be immediately returned if the fingerprint call
// could not be made or as part of the streaming response. If the context is
// cancelled, the error will be propogated.
func (d *devicePluginClient) Fingerprint(ctx context.Context) (<-chan *FingerprintResponse, error) {
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
			// Handle a non-graceful stream error
			if err != io.EOF {
				if errStatus := status.FromContextError(ctx.Err()); errStatus.Code() == codes.Canceled {
					err = context.Canceled
				}

				out <- &FingerprintResponse{
					Error: err,
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
func (d *devicePluginClient) Stats(ctx context.Context) (<-chan *StatsResponse, error) {
	var req proto.StatsRequest
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
			// Handle a non-graceful stream error
			if err != io.EOF {
				if errStatus := status.FromContextError(ctx.Err()); errStatus.Code() == codes.Canceled {
					err = context.Canceled
				}

				out <- &StatsResponse{
					Error: err,
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
