package rpc

import (
	"fmt"

	"github.com/hashicorp/sentinel-sdk"
	"github.com/hashicorp/sentinel-sdk/encoding"
	"github.com/hashicorp/sentinel-sdk/proto/go"
	"golang.org/x/net/context"
)

// ImportGRPCClient is a gRPC server for Imports.
type ImportGRPCClient struct {
	Client proto.ImportClient

	instanceId uint64
}

func (m *ImportGRPCClient) Close() error {
	if m.instanceId > 0 {
		_, err := m.Client.Close(context.Background(), &proto.Close_Request{
			InstanceId: m.instanceId,
		})
		return err
	}

	return nil
}

func (m *ImportGRPCClient) Configure(config map[string]interface{}) error {
	v, err := encoding.GoToValue(config)
	if err != nil {
		return fmt.Errorf("config couldn't be encoded to plugin: %s", err)
	}

	resp, err := m.Client.Configure(context.Background(), &proto.Configure_Request{
		Config: v,
	})
	if err != nil {
		return err
	}

	m.instanceId = resp.InstanceId
	return nil
}

func (m *ImportGRPCClient) Get(rawReqs []*sdk.GetReq) ([]*sdk.GetResult, error) {
	reqs := make([]*proto.Get_Request, 0, len(rawReqs))
	for _, req := range rawReqs {
		var args []*proto.Value
		if req.Args != nil {
			args = make([]*proto.Value, len(req.Args))
			for i, raw := range req.Args {
				v, err := encoding.GoToValue(raw)
				if err != nil {
					return nil, err
				}

				args[i] = v
			}
		}

		reqs = append(reqs, &proto.Get_Request{
			InstanceId:   m.instanceId,
			ExecId:       req.ExecId,
			ExecDeadline: uint64(req.ExecDeadline.Unix()),
			Keys:         req.Keys,
			KeyId:        req.KeyId,
			Call:         args != nil,
			Args:         args,
		})
	}

	resp, err := m.Client.Get(context.Background(), &proto.Get_MultiRequest{
		Requests: reqs,
	})
	if err != nil {
		return nil, err
	}

	results := make([]*sdk.GetResult, 0, len(resp.Responses))
	for _, resp := range resp.Responses {
		v, err := encoding.ValueToGo(resp.Value, nil)
		if err != nil {
			return nil, err
		}

		results = append(results, &sdk.GetResult{
			KeyId: resp.KeyId,
			Keys:  resp.Keys,
			Value: v,
		})
	}

	return results, nil
}
