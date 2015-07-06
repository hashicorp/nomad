package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
)

// Client endpoint is used for client interactions
type Client struct {
	srv *Server
}

// Register is used to upsert a client that is available for scheduling
func (c *Client) Register(args *structs.RegisterRequest, reply *structs.GenericResponse) error {
	if done, err := c.srv.forward("Client.Register", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "register"}, time.Now())

	// Validate the arguments
	if args.Node == nil {
		return fmt.Errorf("missing node for client registration")
	}
	if args.Node.ID == "" {
		return fmt.Errorf("missing node ID for client registration")
	}
	if args.Node.Datacenter == "" {
		return fmt.Errorf("missing datacenter for client registration")
	}
	if args.Node.Name == "" {
		return fmt.Errorf("missing node name for client registration")
	}

	// Default the status if none is given
	if args.Node.Status == "" {
		args.Node.Status = structs.NodeStatusInit
	}

	// Commit this update via Raft
	_, index, err := c.srv.raftApply(structs.RegisterRequestType, args)
	if err != nil {
		c.srv.logger.Printf("[ERR] nomad.client: Register failed: %v", err)
		return err
	}

	// Set the reply index
	reply.Index = index
	return nil
}

// Deregister is used to remove a client from the client. If a client should
// just be made unavailable for scheduling, a status update is prefered.
func (c *Client) Deregister(args *structs.DeregisterRequest, reply *structs.GenericResponse) error {
	if done, err := c.srv.forward("Client.Deregister", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "deregister"}, time.Now())

	// Verify the arguments
	if args.NodeID == "" {
		return fmt.Errorf("missing node ID for client deregistration")
	}

	// Commit this update via Raft
	_, index, err := c.srv.raftApply(structs.DeregisterRequestType, args)
	if err != nil {
		c.srv.logger.Printf("[ERR] nomad.client: Deregister failed: %v", err)
		return err
	}

	// Set the reply index
	reply.Index = index
	return nil
}
