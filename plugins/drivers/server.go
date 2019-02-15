package drivers

import (
	"errors"
	"fmt"
	"io"

	"github.com/golang/protobuf/ptypes"
	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/plugins/drivers/proto"
	dstructs "github.com/hashicorp/nomad/plugins/shared/structs"
	sproto "github.com/hashicorp/nomad/plugins/shared/structs/proto"
	context "golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	default:
		resp.Capabilities.FsIsolation = proto.DriverCapabilities_NONE
	}
	return resp, nil
}

func (b *driverPluginServer) Fingerprint(req *proto.FingerprintRequest, srv proto.Driver_FingerprintServer) error {
	ctx := srv.Context()
	ch, err := b.impl.Fingerprint(ctx)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case f, ok := <-ch:

			if !ok {
				return nil
			}
			resp := &proto.FingerprintResponse{
				Attributes:        dstructs.ConvertStructAttributeMap(f.Attributes),
				Health:            healthStateToProto(f.Health),
				HealthDescription: f.HealthDescription,
			}

			if err := srv.Send(resp); err != nil {
				return err
			}
		}
	}
}

func (b *driverPluginServer) RecoverTask(ctx context.Context, req *proto.RecoverTaskRequest) (*proto.RecoverTaskResponse, error) {
	err := b.impl.RecoverTask(taskHandleFromProto(req.Handle))
	if err != nil {
		return nil, err
	}

	return &proto.RecoverTaskResponse{}, nil
}

func (b *driverPluginServer) StartTask(ctx context.Context, req *proto.StartTaskRequest) (*proto.StartTaskResponse, error) {
	handle, net, err := b.impl.StartTask(taskConfigFromProto(req.Task))
	if err != nil {
		if rec, ok := err.(structs.Recoverable); ok {
			st := status.New(codes.FailedPrecondition, rec.Error())
			st, err := st.WithDetails(&sproto.RecoverableError{Recoverable: rec.IsRecoverable()})
			if err != nil {
				// If this error, it will always error
				panic(err)
			}
			return nil, st.Err()
		}
		return nil, err
	}

	var pbNet *proto.NetworkOverride
	if net != nil {
		pbNet = &proto.NetworkOverride{
			PortMap:       map[string]int32{},
			Addr:          net.IP,
			AutoAdvertise: net.AutoAdvertise,
		}
		for k, v := range net.PortMap {
			pbNet.PortMap[k] = int32(v)
		}
	}

	resp := &proto.StartTaskResponse{
		Handle:          taskHandleToProto(handle),
		NetworkOverride: pbNet,
	}

	return resp, nil
}

func (b *driverPluginServer) WaitTask(ctx context.Context, req *proto.WaitTaskRequest) (*proto.WaitTaskResponse, error) {
	ch, err := b.impl.WaitTask(ctx, req.TaskId)
	if err != nil {
		return nil, err
	}

	var ok bool
	var result *ExitResult
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result, ok = <-ch:
		if !ok {
			return &proto.WaitTaskResponse{
				Err: "channel closed",
			}, nil
		}
	}

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

	var pbNet *proto.NetworkOverride
	if status.NetworkOverride != nil {
		pbNet = &proto.NetworkOverride{
			PortMap:       map[string]int32{},
			Addr:          status.NetworkOverride.IP,
			AutoAdvertise: status.NetworkOverride.AutoAdvertise,
		}
		for k, v := range status.NetworkOverride.PortMap {
			pbNet.PortMap[k] = int32(v)
		}
	}

	resp := &proto.InspectTaskResponse{
		Task: protoStatus,
		Driver: &proto.TaskDriverStatus{
			Attributes: status.DriverAttributes,
		},
		NetworkOverride: pbNet,
	}

	return resp, nil
}

func (b *driverPluginServer) TaskStats(req *proto.TaskStatsRequest, srv proto.Driver_TaskStatsServer) error {
	interval, err := ptypes.Duration(req.CollectionInterval)
	if err != nil {
		return fmt.Errorf("failed to parse collection interval: %v", err)
	}

	ch, err := b.impl.TaskStats(srv.Context(), req.TaskId, interval)
	if err != nil {
		if rec, ok := err.(structs.Recoverable); ok {
			st := status.New(codes.FailedPrecondition, rec.Error())
			st, err := st.WithDetails(&sproto.RecoverableError{Recoverable: rec.IsRecoverable()})
			if err != nil {
				// If this error, it will always error
				panic(err)
			}
			return st.Err()
		}
		return err
	}

	for stats := range ch {
		pb, err := TaskStatsToProto(stats)
		if err != nil {
			return fmt.Errorf("failed to encode task stats: %v", err)
		}

		if err = srv.Send(&proto.TaskStatsResponse{Stats: pb}); err == io.EOF {
			break
		} else if err != nil {
			return err
		}

	}

	return nil
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

func (b *driverPluginServer) ExecTaskStreaming(server proto.Driver_ExecTaskStreamingServer) error {
	// first received message should always be message
	msg, err := server.Recv()
	if msg.Setup != nil {
		return errors.New("first message should always be setup")
	}

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()
	resize := make(chan TerminalSize, 2)

	// go routines for managing output
	go func() {
		bytes := make([]byte, 1024)
		typ := proto.ExecTaskStreamingResponse_Output_STDOUT

		for {

			n, err := outReader.Read(bytes)
			if err != nil {
				return
			}

			server.Send(&proto.ExecTaskStreamingResponse{
				Output: &proto.ExecTaskStreamingResponse_Output{
					Type:  typ,
					Value: bytes[:n],
				},
			})
		}
	}()
	go func() {
		bytes := make([]byte, 1024)
		typ := proto.ExecTaskStreamingResponse_Output_STDERR

		for {

			n, err := errReader.Read(bytes)
			if err != nil {
				return
			}

			server.Send(&proto.ExecTaskStreamingResponse{
				Output: &proto.ExecTaskStreamingResponse_Output{
					Type:  typ,
					Value: bytes[:n],
				},
			})
		}
	}()

	for {
		msg, err = server.Recv()
		if err != nil {
			return err
		}

		switch {
		case msg.Resize != nil:
			resize <- TerminalSize{
				Height: int(msg.Resize.Height),
				Width:  int(msg.Resize.Width),
			}
		case msg.Input != nil:
			inWriter.Write(msg.Input.Value)
		}
	}

	result, err := b.impl.ExecTaskStreaming(server.Context(),
		msg.Setup.TaskId, ExecOptions{
			Command: msg.Setup.Command,
			Tty:     msg.Setup.Tty,
		},
		inReader, outWriter, errWriter, resize)

	if err != nil {
		return err
	}

	err = server.Send(&proto.ExecTaskStreamingResponse{
		Result: exitResultToProto(result),
	})

	return err
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
	ch, err := b.impl.TaskEvents(srv.Context())
	if err != nil {
		return err
	}

	for {
		event := <-ch
		if event == nil {
			break
		}
		pbTimestamp, err := ptypes.TimestampProto(event.Timestamp)
		if err != nil {
			return err
		}

		pbEvent := &proto.DriverTaskEvent{
			TaskId:      event.TaskID,
			AllocId:     event.AllocID,
			TaskName:    event.TaskName,
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
