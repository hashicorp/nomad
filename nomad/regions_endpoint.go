package nomad

import (
	log "github.com/hashicorp/go-hclog"

	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Region is used to query and list the known regions
type Region struct {
	srv    *Server
	logger log.Logger
	rpcCtx *RPCContext
}

// List is used to list all of the known regions. No leader forwarding is
// required for this endpoint because memberlist is used to populate the
// peers list we read from.
func (r *Region) List(args *structs.GenericRequest, reply *[]string) error {
	if err := r.srv.CheckRateLimit("Regions", acl.PolicyList, args.AuthToken, r.rpcCtx); err != nil {
		return err
	}

	*reply = r.srv.Regions()
	return nil
}
