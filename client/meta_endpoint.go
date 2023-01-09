package client

import (
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
)

type NodeMeta struct {
	c *Client
}

func newNodeMetaEndpoint(c *Client) *NodeMeta {
	n := &NodeMeta{c: c}
	return n
}

func (n *NodeMeta) Set(args *structs.NodeMetaSetRequest, reply *structs.NodeMetaResponse) error {
	defer metrics.MeasureSince([]string{"client", "node_meta", "set"}, time.Now())

	// Check node write permissions
	if aclObj, err := n.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeWrite() {
		return nstructs.ErrPermissionDenied
	}

	var stateErr error
	var dynamic map[string]*string

	newNode := n.c.UpdateNode(func(node *structs.Node) {
		// First update the Client's state store. This must be done
		// atomically with updating the metadata inmemory to avoid
		// interleaving updates causing incoherency between the state
		// store and inmemory.
		if dynamic, stateErr = n.c.stateDB.GetNodeMeta(); stateErr != nil {
			return
		}

		if dynamic == nil {
			// DevMode/NoopDB returns a nil map, so initialize it
			dynamic = make(map[string]*string)
		}

		for k, v := range args.Meta {
			dynamic[k] = v
		}

		if stateErr = n.c.stateDB.PutNodeMeta(dynamic); stateErr != nil {
			return
		}

		for k, v := range args.Meta {
			if v == nil {
				delete(node.Meta, k)
				continue
			}

			node.Meta[k] = *v
		}
	})

	if stateErr != nil {
		return stateErr
	}

	// Trigger an async node update
	n.c.updateNode()

	reply.Meta = newNode.Meta
	reply.Dynamic = dynamic
	return nil
}

func (n *NodeMeta) Read(args *structs.NodeSpecificRequest, resp *structs.NodeMetaResponse) error {
	defer metrics.MeasureSince([]string{"client", "node_meta", "read"}, time.Now())

	// Check node read permissions
	if aclObj, err := n.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeRead() {
		return nstructs.ErrPermissionDenied
	}

	// Must acquire configLock to ensure reads aren't interleaved with
	// writes
	n.c.configLock.Lock()
	defer n.c.configLock.Unlock()

	dynamic, err := n.c.stateDB.GetNodeMeta()
	if err != nil {
		return err
	}

	resp.Meta = n.c.config.Node.Meta
	resp.Dynamic = dynamic
	return nil
}
