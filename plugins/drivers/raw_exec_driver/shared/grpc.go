package shared

import (
	"github.com/hashicorp/nomad/plugins/drivers/raw_exec_driver/proto"
	"golang.org/x/net/context"
)

type GRPCClient struct{ client proto.RawExecClient }

func (m *GRPCClient) NewStart(ctx *proto.ExecContext, taskInfo *proto.TaskInfo) (*proto.StartResponse, error) {
	req := &proto.StartRequest{
		ExecContext: ctx,
		TaskInfo:    taskInfo,
	}
	resp, err := m.client.NewStart(context.Background(), req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type GRPCServer struct {
	Impl RawExec
}

func (m *GRPCServer) NewStart(
	ctx context.Context,
	req *proto.StartRequest) (*proto.StartResponse, error) {
	resp, err := m.Impl.NewStart(req.ExecContext, req.TaskInfo)
	return resp, err
}
