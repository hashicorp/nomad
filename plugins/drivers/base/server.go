package base

import (
	"io"

	"golang.org/x/net/context"

	"github.com/golang/protobuf/ptypes"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/plugins/drivers/base/proto"
)

type driverPluginServer struct {
	broker *plugin.GRPCBroker
	impl   DriverPlugin
}

func (b *driverPluginServer) TaskConfigSchema(ctx context.Context, req *proto.TaskConfigSchemaRequest) (*proto.TaskConfigSchemaResponse, error) {
	spec, err := b.impl.TaskConfigSchema()
	if err != nil {
		return nil, err
	}

	resp := &proto.TaskConfigSchemaResponse{
		Spec: spec,
	}
	return resp, nil
}

func (b *driverPluginServer) Capabilities(ctx context.Context, req *proto.CapabilitiesRequest) (*proto.CapabilitiesResponse, error) {
	caps, err := b.impl.Capabilities()
	if err != nil {
		return nil, err
	}
	resp := &proto.CapabilitiesResponse{
		Capabilities: &proto.DriverCapabilities{
			SendSignals: caps.SendSignals,
			Exec:        caps.Exec,
		},
	}

	switch caps.FSIsolation {
	case FSIsolationNone:
		resp.Capabilities.FsIsolation = proto.DriverCapabilities_NONE
	case FSIsolationChroot:
		resp.Capabilities.FsIsolation = proto.DriverCapabilities_CHROOT
	case FSIsolationImage:
		resp.Capabilities.FsIsolation = proto.DriverCapabilities_IMAGE
	}
	return resp, nil
}

func (b *driverPluginServer) Fingerprint(req *proto.FingerprintRequest, srv proto.Driver_FingerprintServer) error {
	ch, err := b.impl.Fingerprint()
	if err != nil {
		return err
	}

	for {
		f := <-ch
		resp := &proto.FingerprintResponse{
			Attributes:        f.Attributes,
			Health:            healthStateToProto(f.Health),
			HealthDescription: f.HealthDescription,
		}

		if err := srv.Send(resp); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (b *driverPluginServer) RecoverTask(ctx context.Context, req *proto.RecoverTaskRequest) (*proto.RecoverTaskResponse, error) {
	err := b.impl.RecoverTask(taskHandleFromProto(req.Handle))
	if err != nil {
		return nil, err
	}

	return &proto.RecoverTaskResponse{}, nil
}

func (b *driverPluginServer) StartTask(ctx context.Context, req *proto.StartTaskRequest) (*proto.StartTaskResponse, error) {
	handle, err := b.impl.StartTask(taskConfigFromProto(req.Task))
	if err != nil {
		return nil, err
	}

	resp := &proto.StartTaskResponse{
		Handle: taskHandleToProto(handle),
	}

	return resp, nil
}

func (b *driverPluginServer) WaitTask(ctx context.Context, req *proto.WaitTaskRequest) (*proto.WaitTaskResponse, error) {
	ch := b.impl.WaitTask(ctx, req.TaskId)
	result := <-ch
	var errStr string
	if result.Err != nil {
		errStr = result.Err.Error()
	}

	resp := &proto.WaitTaskResponse{
		Err: errStr,
		Result: &proto.ExitResult{
			ExitCode:  int32(result.ExitCode),
			Signal:    int32(result.Signal),
			OomKilled: result.OOMKilled,
		},
	}

	return resp, nil
}

func (b *driverPluginServer) StopTask(ctx context.Context, req *proto.StopTaskRequest) (*proto.StopTaskResponse, error) {
	timeout, err := ptypes.Duration(req.Timeout)
	if err != nil {
		return nil, err
	}

	err = b.impl.StopTask(req.TaskId, timeout, req.Signal)
	if err != nil {
		return nil, err
	}
	return &proto.StopTaskResponse{}, nil
}

func (b *driverPluginServer) DestroyTask(ctx context.Context, req *proto.DestroyTaskRequest) (*proto.DestroyTaskResponse, error) {
	err := b.impl.DestroyTask(req.TaskId, req.Force)
	if err != nil {
		return nil, err
	}
	return &proto.DestroyTaskResponse{}, nil
}

func (b *driverPluginServer) InspectTask(ctx context.Context, req *proto.InspectTaskRequest) (*proto.InspectTaskResponse, error) {
	status, err := b.impl.InspectTask(req.TaskId)
	if err != nil {
		return nil, err
	}

	protoStatus, err := taskStatusToProto(status)
	if err != nil {
		return nil, err
	}

	resp := &proto.InspectTaskResponse{
		Task: protoStatus,
		Driver: &proto.TaskDriverStatus{
			Attributes: status.DriverAttributes,
		},
		NetworkOverride: &proto.NetworkOverride{
			PortMap:       status.NetworkOverride.PortMap,
			Addr:          status.NetworkOverride.Addr,
			AutoAdvertise: status.NetworkOverride.AutoAdvertise,
		},
	}

	return resp, nil
}

func (b *driverPluginServer) TaskStats(ctx context.Context, req *proto.TaskStatsRequest) (*proto.TaskStatsResponse, error) {
	stats, err := b.impl.TaskStats(req.TaskId)
	if err != nil {
		return nil, err
	}

	pb, err := taskStatsToProto(stats)
	if err != nil {
		return nil, err
	}

	resp := &proto.TaskStatsResponse{
		Stats: pb,
	}

	return resp, nil
}

func (b *driverPluginServer) ExecTask(ctx context.Context, req *proto.ExecTaskRequest) (*proto.ExecTaskResponse, error) {
	timeout, err := ptypes.Duration(req.Timeout)
	if err != nil {
		return nil, err
	}

	result, err := b.impl.ExecTask(req.TaskId, req.Command, timeout)
	if err != nil {
		return nil, err
	}
	resp := &proto.ExecTaskResponse{
		Stdout: result.Stdout,
		Stderr: result.Stderr,
		Result: exitResultToProto(result.ExitResult),
	}

	return resp, nil
}

func (b *driverPluginServer) SignalTask(ctx context.Context, req *proto.SignalTaskRequest) (*proto.SignalTaskResponse, error) {
	err := b.impl.SignalTask(req.TaskId, req.Signal)
	if err != nil {
		return nil, err
	}

	resp := &proto.SignalTaskResponse{}
	return resp, nil
}

func (b *driverPluginServer) TaskEvents(req *proto.TaskEventsRequest, srv proto.Driver_TaskEventsServer) error {
	ch, err := b.impl.TaskEvents()
	if err != nil {
		return err
	}

	for {
		event := <-ch
		pbTimestamp, err := ptypes.TimestampProto(event.Timestamp)
		if err != nil {
			return err
		}

		pbEvent := &proto.DriverTaskEvent{
			TaskId:      event.TaskID,
			Timestamp:   pbTimestamp,
			Message:     event.Message,
			Annotations: event.Annotations,
		}

		if err = srv.Send(pbEvent); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}
	return nil
}
