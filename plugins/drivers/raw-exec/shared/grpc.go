package shared

import (
	"github.com/hashicorp/nomad/plugins/drivers/raw-exec/proto"
	"golang.org/x/net/context"
)

type GRPCClient struct{ client proto.RawExecClient }

func (m *GRPCClient) Start(ctx *proto.ExecContext, taskInfo *proto.TaskInfo) (*proto.StartResponse, error) {
	req := &proto.StartRequest{
		ExecContext: ctx,
		TaskInfo:    taskInfo,
	}
	resp, err := m.client.Start(context.Background(), req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (m *GRPCClient) Stop(tc *proto.TaskState) (*proto.StopResponse, error) {
	req := &proto.StopRequest{
		TaskState: tc,
	}
	resp, err := m.client.Stop(context.Background(), req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type GRPCServer struct {
	Impl RawExec
}

func (m *GRPCServer) Start(
	ctx context.Context,
	req *proto.StartRequest) (*proto.StartResponse, error) {
	resp, err := m.Impl.Start(req.ExecContext, req.TaskInfo)
	return resp, err
}

func (m *GRPCServer) Stop(
	ctx context.Context,
	req *proto.StopRequest) (*proto.StopResponse, error) {
	resp, err := m.Impl.Stop(req.TaskState)
	return resp, err
}
