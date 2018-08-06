package shared

import (
	"net/rpc"

	"github.com/hashicorp/nomad/plugins/drivers/raw-exec/proto"
)

type RPCClient struct{ client *rpc.Client }

func (m *RPCClient) Start(req *proto.StartRequest) (*proto.StartResponse, error) {
	var resp proto.StartResponse
	err := m.client.Call("Plugin.Start", req, &resp)
	return &resp, err
}

func (m *RPCClient) Stop(req *proto.StopRequest) (*proto.StopResponse, error) {
	var resp proto.StopResponse
	err := m.client.Call("Plugin.Stop", req, &resp)
	return &resp, err
}

type RPCServer struct {
	Impl RawExec
}

func (m *RPCServer) Start(req *proto.StartRequest, resp *proto.StartResponse) error {
	v, err := m.Impl.Start(req.ExecContext, req.TaskInfo)
	resp = v
	return err
}

func (m *RPCServer) Stop(req *proto.StopRequest, resp *proto.StopResponse) error {
	v, err := m.Impl.Stop(req.TaskState)
	resp = v
	return err
}

func (m *RPCServer) Restore(req *proto.RestoreRequest, resp *proto.RestoreResponse) error {
	v, err := m.Impl.Restore(req.TaskStates)
	resp = v
	return err
}
