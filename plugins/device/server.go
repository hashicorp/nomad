package device

import (
	context "golang.org/x/net/context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/device/proto"
)

// devicePluginServer wraps a device plugin and exposes it via gRPC.
type devicePluginServer struct {
	broker *plugin.GRPCBroker
	impl   DevicePlugin
}

func (d *devicePluginServer) Fingerprint(req *proto.FingerprintRequest, stream proto.DevicePlugin_FingerprintServer) error {
	ctx := stream.Context()
	outCh, err := d.impl.Fingerprint(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case resp, ok := <-outCh:
			// The output channel has been closed, end the stream
			if !ok {
				return nil
			}

			// Handle any error
			if resp.Error != nil {
				return resp.Error
			}

			// Convert the devices
			out := convertStructDeviceGroups(resp.Devices)

			// Build the response
			presp := &proto.FingerprintResponse{
				DeviceGroup: out,
			}

			// Send the devices
			if err := stream.Send(presp); err != nil {
				return err
			}
		}
	}
}

func (d *devicePluginServer) Reserve(ctx context.Context, req *proto.ReserveRequest) (*proto.ReserveResponse, error) {
	resp, err := d.impl.Reserve(req.GetDeviceIds())
	if err != nil {
		return nil, err
	}

	// Make the response
	presp := &proto.ReserveResponse{
		ContainerRes: convertStructContainerReservation(resp),
	}

	return presp, nil
}

func (d *devicePluginServer) Stats(req *proto.StatsRequest, stream proto.DevicePlugin_StatsServer) error {
	ctx := stream.Context()
	outCh, err := d.impl.Stats(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case resp, ok := <-outCh:
			// The output channel has been closed, end the stream
			if !ok {
				return nil
			}

			// Handle any error
			if resp.Error != nil {
				return resp.Error
			}

			// Convert the devices
			out := convertStructDeviceGroupsStats(resp.Groups)

			// Build the response
			presp := &proto.StatsResponse{
				Groups: out,
			}

			// Send the devices
			if err := stream.Send(presp); err != nil {
				return err
			}
		}
	}
}
