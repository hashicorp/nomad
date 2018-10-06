package docklog

import (
	"golang.org/x/net/context"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/nomad/drivers/docker/docklog/proto"
)

// docklogServer is the server side translation between the protobuf and native interfaces
type docklogServer struct {
	broker *plugin.GRPCBroker
	impl   Docklog
}

// Start proxies the protobuf Start RPC to the Start fun of the Docklog interface
func (s *docklogServer) Start(ctx context.Context, req *proto.StartRequest) (*proto.StartResponse, error) {
	opts := &StartOpts{
		Endpoint:    req.Endpoint,
		ContainerID: req.ContainerId,
		Stdout:      req.StdoutFifo,
		Stderr:      req.StderrFifo,

		TLSCert: req.TlsCert,
		TLSKey:  req.TlsKey,
		TLSCA:   req.TlsCa,
	}
	err := s.impl.Start(opts)
	if err != nil {
		return nil, err
	}
	resp := &proto.StartResponse{}
	return resp, nil
}

// Stop proxies the protobuf Stop RPC to the Stop fun of the Docklog interface
func (s *docklogServer) Stop(ctx context.Context, req *proto.StopRequest) (*proto.StopResponse, error) {
	return &proto.StopResponse{}, s.impl.Stop()
}
