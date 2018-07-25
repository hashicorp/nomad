package shared

import (
	"net/rpc"

	"github.com/hashicorp/nomad/plugins/drivers/raw_exec_driver/proto"
)

type RPCClient struct{ client *rpc.Client }

func (m *RPCClient) NewStart(req *proto.StartRequest) (*proto.StartResponse, error) {
	var resp proto.StartResponse
	err := m.client.Call("Plugin.NewStart", req, &resp)
	return &resp, err
}

type RPCServer struct {
	Impl RawExec
}

func (m *RPCServer) NewStart(req *proto.StartRequest, resp *proto.StartResponse) error {
	v, err := m.Impl.NewStart(req.ExecContext, req.TaskInfo)
	resp = v
	return err
}
