// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ClientHostVolume is the client RPC endpoint for host volumes
type ClientHostVolume struct {
	srv    *Server
	ctx    *RPCContext
	logger log.Logger
}

func NewClientHostVolumeEndpoint(srv *Server, ctx *RPCContext) *ClientHostVolume {
	return &ClientHostVolume{srv: srv, ctx: ctx, logger: srv.logger.Named("client_host_volume")}
}

func (c *ClientHostVolume) Create(args *cstructs.ClientHostVolumeCreateRequest, reply *cstructs.ClientHostVolumeCreateResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_host_node", "create"}, time.Now())
	return c.sendVolumeRPC(
		args.NodeID,
		"HostVolume.Create",
		"ClientHostVolume.Create",
		structs.RateMetricWrite,
		args,
		reply,
	)
}

func (c *ClientHostVolume) Delete(args *cstructs.ClientHostVolumeDeleteRequest, reply *cstructs.ClientHostVolumeDeleteResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_host_volume", "delete"}, time.Now())
	return c.sendVolumeRPC(
		args.NodeID,
		"HostVolume.Delete",
		"ClientHostVolume.Delete",
		structs.RateMetricWrite,
		args,
		reply,
	)
}

func (c *ClientHostVolume) sendVolumeRPC(nodeID, method, fwdMethod, op string, args any, reply any) error {
	// client requests aren't RequestWithIdentity, so we use a placeholder here
	// to populate the identity data for metrics
	identityReq := &structs.GenericRequest{}
	aclObj, err := c.srv.AuthenticateServerOnly(c.ctx, identityReq)
	c.srv.MeasureRPCRate("client_host_volume", op, identityReq)

	if err != nil || !aclObj.AllowServerOp() {
		return structs.ErrPermissionDenied
	}

	// Make sure Node is valid and new enough to support RPC
	snap, err := c.srv.State().Snapshot()
	if err != nil {
		return err
	}

	_, err = getNodeForRpc(snap, nodeID)
	if err != nil {
		return err
	}

	// Get the connection to the client
	state, ok := c.srv.getNodeConn(nodeID)
	if !ok {
		return findNodeConnAndForward(c.srv, nodeID, fwdMethod, args, reply)
	}

	// Make the RPC
	if err := NodeRpc(state.Session, method, args, reply); err != nil {
		return fmt.Errorf("%s error: %w", method, err)
	}
	return nil
}
