package device

import (
	"context"
	"io"
	"time"

	"github.com/LK4D4/joincontext"
	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/nomad/helper/pluginutils/grpcutils"
	"github.com/hashicorp/nomad/plugins/base"
	"github.com/hashicorp/nomad/plugins/device/proto"
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
// cancelled, the error will be propagated.
func (d *devicePluginClient) Fingerprint(ctx context.Context) (<-chan *FingerprintResponse, error) {
	// Join the passed context and the shutdown context
	joinedCtx, _ := joincontext.Join(ctx, d.doneCtx)

	var req proto.FingerprintRequest
	stream, err := d.client.Fingerprint(joinedCtx, &req)
	if err != nil {
		return nil, grpcutils.HandleReqCtxGrpcErr(err, ctx, d.doneCtx)
	}

	out := make(chan *FingerprintResponse, 1)
	go d.handleFingerprint(ctx, stream, out)
	return out, nil
}

// handleFingerprint should be launched in a goroutine and handles converting
// the gRPC stream to a channel. Exits either when context is cancelled or the
// stream has an error.
func (d *devicePluginClient) handleFingerprint(
	reqCtx context.Context,
	stream proto.DevicePlugin_FingerprintClient,
	out chan *FingerprintResponse) {

	defer close(out)
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				out <- &FingerprintResponse{
					Error: grpcutils.HandleReqCtxGrpcErr(err, reqCtx, d.doneCtx),
				}
			}

			// End the stream
			return
		}

		// Send the response
		f := &FingerprintResponse{
			Devices: convertProtoDeviceGroups(resp.GetDeviceGroup()),
		}
		select {
		case <-reqCtx.Done():
			return
		case out <- f:
		}
	}
}

func (d *devicePluginClient) Reserve(deviceIDs []string) (*ContainerReservation, error) {
	// Build the request
	req := &proto.ReserveRequest{
		DeviceIds: deviceIDs,
	}

	// Make the request
	resp, err := d.client.Reserve(d.doneCtx, req)
	if err != nil {
		return nil, grpcutils.HandleGrpcErr(err, d.doneCtx)
	}

	// Convert the response
	out := convertProtoContainerReservation(resp.GetContainerRes())
	return out, nil
}

// Stats is used to retrieve device statistics from the device plugin. An error
// may be immediately returned if the stats call could not be made or as part of
// the streaming response. If the context is cancelled, the error will be
// propagated.
func (d *devicePluginClient) Stats(ctx context.Context, interval time.Duration) (<-chan *StatsResponse, error) {
	// Join the passed context and the shutdown context
	joinedCtx, _ := joincontext.Join(ctx, d.doneCtx)

	req := proto.StatsRequest{
		CollectionInterval: ptypes.DurationProto(interval),
	}
	stream, err := d.client.Stats(joinedCtx, &req)
	if err != nil {
		return nil, grpcutils.HandleReqCtxGrpcErr(err, ctx, d.doneCtx)
	}

	out := make(chan *StatsResponse, 1)
	go d.handleStats(ctx, stream, out)
	return out, nil
}

// handleStats should be launched in a goroutine and handles converting
// the gRPC stream to a channel. Exits either when context is cancelled or the
// stream has an error.
func (d *devicePluginClient) handleStats(
	reqCtx context.Context,
	stream proto.DevicePlugin_StatsClient,
	out chan *StatsResponse) {

	defer close(out)
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				out <- &StatsResponse{
					Error: grpcutils.HandleReqCtxGrpcErr(err, reqCtx, d.doneCtx),
				}
			}

			// End the stream
			return
		}

		// Send the response
		s := &StatsResponse{
			Groups: convertProtoDeviceGroupsStats(resp.GetGroups()),
		}
		select {
		case <-reqCtx.Done():
			return
		case out <- s:
		}
	}
}
