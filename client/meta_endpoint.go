// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"net/http"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/nomad/nomad/structs"
	"golang.org/x/exp/maps"
)

type NodeMeta struct {
	c *Client
}

func newNodeMetaEndpoint(c *Client) *NodeMeta {
	n := &NodeMeta{c: c}
	return n
}

func (n *NodeMeta) Apply(args *structs.NodeMetaApplyRequest, reply *structs.NodeMetaResponse) error {
	defer metrics.MeasureSince([]string{"client", "node_meta", "apply"}, time.Now())

	// Check node write permissions
	if aclObj, err := n.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeWrite() {
		return structs.ErrPermissionDenied
	}

	if err := args.Validate(); err != nil {
		return structs.NewErrRPCCoded(http.StatusBadRequest, err.Error())
	}

	var stateErr error
	var dyn map[string]*string

	newNode := n.c.UpdateNode(func(node *structs.Node) {
		// First update the Client's state store. This must be done
		// atomically with updating the metadata inmemory to avoid
		// bad interleaving between concurrent updates.
		dyn = maps.Clone(n.c.metaDynamic)
		maps.Copy(dyn, args.Meta)

		if stateErr = n.c.stateDB.PutNodeMeta(dyn); stateErr != nil {
			return
		}

		// Apply updated dynamic metadata to client and node now that the part of
		// the operation that can fail succeeded (persistence). Must clone as dyn
		// is read outside of UpdateNode.
		n.c.metaDynamic = maps.Clone(dyn)

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
	reply.Dynamic = dyn
	reply.Static = n.c.metaStatic
	return nil
}

func (n *NodeMeta) Read(args *structs.NodeSpecificRequest, reply *structs.NodeMetaResponse) error {
	defer metrics.MeasureSince([]string{"client", "node_meta", "read"}, time.Now())

	// Check node read permissions
	if aclObj, err := n.c.ResolveToken(args.AuthToken); err != nil {
		return err
	} else if aclObj != nil && !aclObj.AllowNodeRead() {
		return structs.ErrPermissionDenied
	}

	// Must acquire configLock to ensure reads aren't interleaved with
	// writes
	n.c.configLock.Lock()
	defer n.c.configLock.Unlock()

	reply.Meta = n.c.config.Node.Meta
	reply.Dynamic = maps.Clone(n.c.metaDynamic)
	reply.Static = n.c.metaStatic
	return nil
}
