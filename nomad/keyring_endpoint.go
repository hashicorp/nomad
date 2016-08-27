package nomad

import (
	"errors"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/serf/serf"
)

type Keyring struct {
	srv *Server
}

// Execute will query the WAN gossip keyrings of all servers.
func (k *Keyring) Execute(
	args *structs.KeyringRequest,
	reply *structs.KeyringResponses) error {

	var serfResp *serf.KeyResponse
	var err error
	mgr := k.srv.KeyManager()

	switch args.Operation {
	case structs.KeyringList:
		serfResp, err = mgr.ListKeys()
	case structs.KeyringInstall:
		serfResp, err = mgr.InstallKey(args.Key)
	case structs.KeyringUse:
		serfResp, err = mgr.UseKey(args.Key)
	case structs.KeyringRemove:
		serfResp, err = mgr.RemoveKey(args.Key)
	default:
		err = errors.New("Invalid keyring operation")
	}

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	reply.Responses = append(reply.Responses, &structs.KeyringResponse{
		WAN:        true,
		Datacenter: k.srv.config.Datacenter,
		Messages:   serfResp.Messages,
		Keys:       serfResp.Keys,
		NumNodes:   serfResp.NumNodes,
		Error:      errStr,
	})
	return nil
}
