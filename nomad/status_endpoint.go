package nomad

import "github.com/hashicorp/nomad/nomad/structs"

// Status endpoint is used to check on server status
type Status struct {
	srv *Server
}

// Version is used to allow clients to determine the capabilities
// of the server
func (s *Status) Version(args *structs.GenericRequest, reply *structs.VersionResponse) error {
	if done, err := s.srv.forward("Status.Version", args, args, reply); done {
		return err
	}

	conf := s.srv.config
	reply.Build = conf.Build
	reply.Versions = map[string]int{
		structs.ProtocolVersion: int(conf.ProtocolVersion),
		structs.APIMajorVersion: apiMajorVersion,
		structs.APIMinorVersion: apiMinorVersion,
	}
	return nil
}

// Ping is used to just check for connectivity
func (s *Status) Ping(args struct{}, reply *struct{}) error {
	return nil
}

// Leader is used to get the address of the leader
func (s *Status) Leader(args struct{}, reply *string) error {
	leader := s.srv.raft.Leader()
	if leader != "" {
		*reply = leader
	} else {
		*reply = ""
	}
	return nil
}

// Peers is used to get all the Raft peers
func (s *Status) Peers(args struct{}, reply *[]string) error {
	peers, err := s.srv.raftPeers.Peers()
	if err != nil {
		return err
	}

	*reply = peers
	return nil
}
