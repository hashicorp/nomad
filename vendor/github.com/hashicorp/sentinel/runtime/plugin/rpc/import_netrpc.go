package rpc

import (
	"net/rpc"

	goplugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/sentinel/runtime/gobridge"
	"github.com/hashicorp/sentinel/runtime/plugin"
)

// Import implementation of plugin.Import that communicates over net/rpc.
type Import struct {
	Broker *goplugin.MuxBroker
	Client *rpc.Client
}

// Close closes the RPC connection. The Import is unusable after this.
func (i *Import) Close() error {
	return i.Client.Close()
}

func (i *Import) Configure(raw map[string]interface{}) error {
	var resp ImportConfigureResponse
	err := i.Client.Call("Plugin.Configure", raw, &resp)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		err = resp.Error
		return err
	}

	return nil
}

func (i *Import) Get(reqs []*gobridge.GetReq) ([]*gobridge.GetResult, error) {
	var resp ImportGetResponse
	args := &ImportGetArgs{
		Reqs: reqs,
	}
	err := i.Client.Call("Plugin.Get", args, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		err = resp.Error
		return nil, err
	}

	return resp.Results, nil
}

type ImportGetArgs struct {
	Reqs []*gobridge.GetReq
}

type ImportGetResponse struct {
	Results []*gobridge.GetResult
	Error   *goplugin.BasicError
}

type ImportConfigureResponse struct {
	Error *goplugin.BasicError
}

// ImportServer is a net/rpc compatible structure for serving an ImportServer.
// This should not be used directly.
type ImportServer struct {
	Broker *goplugin.MuxBroker
	Import plugin.Import
}

func (s *ImportServer) Configure(
	raw map[string]interface{},
	reply *ImportConfigureResponse) error {
	err := s.Import.Configure(raw)
	*reply = ImportConfigureResponse{
		Error: goplugin.NewBasicError(err),
	}

	return nil
}

func (s *ImportServer) Get(
	args *ImportGetArgs,
	reply *ImportGetResponse) error {
	results, err := s.Import.Get(args.Reqs)
	*reply = ImportGetResponse{
		Results: results,
		Error:   goplugin.NewBasicError(err),
	}

	return nil
}
