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

// UpdateStatus is used to update the status of a client node
func (c *Client) UpdateStatus(args *structs.UpdateStatusRequest, reply *structs.GenericResponse) error {
	if done, err := c.srv.forward("Client.UpdateStatus", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "update_status"}, time.Now())

	// Verify the arguments
	if args.NodeID == "" {
		return fmt.Errorf("missing node ID for client deregistration")
	}
	switch args.Status {
	case structs.NodeStatusInit, structs.NodeStatusReady,
		structs.NodeStatusMaint, structs.NodeStatusDown:
	default:
		return fmt.Errorf("invalid status for node")
	}

	// Commit this update via Raft
	_, index, err := c.srv.raftApply(structs.NodeUpdateStatusRequestType, args)
	if err != nil {
		c.srv.logger.Printf("[ERR] nomad.client: status update failed: %v", err)
		return err
	}

	// Set the reply index
	reply.Index = index
	return nil
}

// GetNode is used to request information about a specific ndoe
func (c *Client) GetNode(args *structs.NodeSpecificRequest,
	reply *structs.SingleNodeResponse) error {
	if done, err := c.srv.forward("Client.GetNode", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"nomad", "client", "get_node"}, time.Now())

	// Verify the arguments
	if args.NodeID == "" {
		return fmt.Errorf("missing node ID")
	}

	// Look for the node
	snap, err := c.srv.fsm.State().Snapshot()
	if err != nil {
		return err
	}
	out, err := snap.GetNodeByID(args.NodeID)
	if err != nil {
		return err
	}

	// Setup the output
	if out != nil {
		reply.Node = out
		reply.Index = out.ModifyIndex
	} else {
		// Use the last index that affected the nodes table
		index, err := snap.GetIndex("nodes")
		if err != nil {
			return err
		}
		reply.Index = index
	}

	// Set the query response
	c.srv.setQueryMeta(&reply.QueryMeta)
	return nil
}
