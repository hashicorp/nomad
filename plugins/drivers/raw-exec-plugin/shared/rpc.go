package shared

import (
	"net/rpc"

	"github.com/hashicorp/nomad/plugins/drivers/raw-exec-plugin/proto"
)

type RPCClient struct{ client *rpc.Client }

func (m *RPCClient) Start(req *proto.StartRequest) (*proto.StartResponse, error) {
	var resp proto.StartResponse
	err := m.client.Call("Plugin.Start", req, &resp)
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
