package nomad

import "github.com/hashicorp/nomad/nomad/structs"

// Region is used to query and list the known regions
type Region struct {
	srv *Server
}

// List is used to list all of the known regions.
func (r *Region) List(args *structs.GenericRequest, reply *[]string) error {
	if done, err := r.srv.forward("Region.List", args, args, reply); done {
		return err
	}

	*reply = r.srv.Regions()
	return nil
}
