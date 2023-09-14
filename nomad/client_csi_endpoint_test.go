// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"
	"net/rpc"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

// MockClientCSI is a mock for the nomad.ClientCSI RPC server (see
// nomad/client_csi_endpoint.go). This can be used with a TestRPCOnlyClient to
// return specific plugin responses back to server RPCs for testing. Note that
// responses that have no bodies have no "Next*Response" field and will always
// return an empty response body.
type MockClientCSI struct {
	NextValidateError                  error
	NextAttachError                    error
	NextAttachResponse                 *cstructs.ClientCSIControllerAttachVolumeResponse
	NextDetachError                    error
	NextCreateError                    error
	NextCreateResponse                 *cstructs.ClientCSIControllerCreateVolumeResponse
	NextDeleteError                    error
	NextListExternalError              error
	NextListExternalResponse           *cstructs.ClientCSIControllerListVolumesResponse
	NextCreateSnapshotError            error
	NextCreateSnapshotResponse         *cstructs.ClientCSIControllerCreateSnapshotResponse
	NextDeleteSnapshotError            error
	NextListExternalSnapshotsError     error
	NextListExternalSnapshotsResponse  *cstructs.ClientCSIControllerListSnapshotsResponse
	NextControllerExpandVolumeError    error
	NextControllerExpandVolumeResponse *cstructs.ClientCSIControllerExpandVolumeResponse
	NextNodeDetachError                error
}

func newMockClientCSI() *MockClientCSI {
	return &MockClientCSI{
		NextAttachResponse:                 &cstructs.ClientCSIControllerAttachVolumeResponse{},
		NextCreateResponse:                 &cstructs.ClientCSIControllerCreateVolumeResponse{},
		NextListExternalResponse:           &cstructs.ClientCSIControllerListVolumesResponse{},
		NextCreateSnapshotResponse:         &cstructs.ClientCSIControllerCreateSnapshotResponse{},
		NextListExternalSnapshotsResponse:  &cstructs.ClientCSIControllerListSnapshotsResponse{},
		NextControllerExpandVolumeResponse: &cstructs.ClientCSIControllerExpandVolumeResponse{},
	}
}

func (c *MockClientCSI) ControllerValidateVolume(req *cstructs.ClientCSIControllerValidateVolumeRequest, resp *cstructs.ClientCSIControllerValidateVolumeResponse) error {
	return c.NextValidateError
}

func (c *MockClientCSI) ControllerAttachVolume(req *cstructs.ClientCSIControllerAttachVolumeRequest, resp *cstructs.ClientCSIControllerAttachVolumeResponse) error {
	*resp = *c.NextAttachResponse
	return c.NextAttachError
}

func (c *MockClientCSI) ControllerDetachVolume(req *cstructs.ClientCSIControllerDetachVolumeRequest, resp *cstructs.ClientCSIControllerDetachVolumeResponse) error {
	return c.NextDetachError
}

func (c *MockClientCSI) ControllerCreateVolume(req *cstructs.ClientCSIControllerCreateVolumeRequest, resp *cstructs.ClientCSIControllerCreateVolumeResponse) error {
	*resp = *c.NextCreateResponse
	return c.NextCreateError
}

func (c *MockClientCSI) ControllerDeleteVolume(req *cstructs.ClientCSIControllerDeleteVolumeRequest, resp *cstructs.ClientCSIControllerDeleteVolumeResponse) error {
	return c.NextDeleteError
}

func (c *MockClientCSI) ControllerListVolumes(req *cstructs.ClientCSIControllerListVolumesRequest, resp *cstructs.ClientCSIControllerListVolumesResponse) error {
	*resp = *c.NextListExternalResponse
	return c.NextListExternalError
}

func (c *MockClientCSI) ControllerCreateSnapshot(req *cstructs.ClientCSIControllerCreateSnapshotRequest, resp *cstructs.ClientCSIControllerCreateSnapshotResponse) error {
	*resp = *c.NextCreateSnapshotResponse
	return c.NextCreateSnapshotError
}

func (c *MockClientCSI) ControllerDeleteSnapshot(req *cstructs.ClientCSIControllerDeleteSnapshotRequest, resp *cstructs.ClientCSIControllerDeleteSnapshotResponse) error {
	return c.NextDeleteSnapshotError
}

func (c *MockClientCSI) ControllerListSnapshots(req *cstructs.ClientCSIControllerListSnapshotsRequest, resp *cstructs.ClientCSIControllerListSnapshotsResponse) error {
	*resp = *c.NextListExternalSnapshotsResponse
	return c.NextListExternalSnapshotsError
}

func (c *MockClientCSI) ControllerExpandVolume(req *cstructs.ClientCSIControllerExpandVolumeRequest, resp *cstructs.ClientCSIControllerExpandVolumeResponse) error {
	*resp = *c.NextControllerExpandVolumeResponse
	return c.NextControllerExpandVolumeError
}

func (c *MockClientCSI) NodeDetachVolume(req *cstructs.ClientCSINodeDetachVolumeRequest, resp *cstructs.ClientCSINodeDetachVolumeResponse) error {
	return c.NextNodeDetachError
}

func TestClientCSIController_AttachVolume_Local(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupLocal(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerAttachVolumeRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerAttachVolume", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_AttachVolume_Forwarded(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupForward(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerAttachVolumeRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerAttachVolume", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_DetachVolume_Local(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupLocal(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerDetachVolumeRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerDetachVolume", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_DetachVolume_Forwarded(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupForward(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerDetachVolumeRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerDetachVolume", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_ValidateVolume_Local(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupLocal(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerValidateVolumeRequest{
		VolumeID:           "test",
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerValidateVolume", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_ValidateVolume_Forwarded(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupForward(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerValidateVolumeRequest{
		VolumeID:           "test",
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerValidateVolume", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_CreateVolume_Local(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupLocal(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerCreateVolumeRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerCreateVolume", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_CreateVolume_Forwarded(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupForward(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerCreateVolumeRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerCreateVolume", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_DeleteVolume_Local(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupLocal(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerDeleteVolumeRequest{
		ExternalVolumeID:   "test",
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerDeleteVolume", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_DeleteVolume_Forwarded(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupForward(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerDeleteVolumeRequest{
		ExternalVolumeID:   "test",
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerDeleteVolume", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_ListVolumes_Local(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupLocal(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerListVolumesRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerListVolumes", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_ListVolumes_Forwarded(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupForward(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerListVolumesRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerListVolumes", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_CreateSnapshot_Local(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupLocal(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerCreateSnapshotRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerCreateSnapshot", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_CreateSnapshot_Forwarded(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupForward(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerCreateSnapshotRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerCreateSnapshot", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_DeleteSnapshot_Local(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupLocal(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerDeleteSnapshotRequest{
		ID:                 "test",
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerDeleteSnapshot", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_DeleteSnapshot_Forwarded(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupForward(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerDeleteSnapshotRequest{
		ID:                 "test",
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerDeleteSnapshot", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_ListSnapshots_Local(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupLocal(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerListSnapshotsRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerListSnapshots", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_ListSnapshots_Forwarded(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)
	codec, cleanup := setupForward(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerListSnapshotsRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerListSnapshots", req, &resp)
	require.Error(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSI_NodeForControllerPlugin(t *testing.T) {
	ci.Parallel(t)
	srv, shutdown := TestServer(t, func(c *Config) {})
	testutil.WaitForLeader(t, srv.RPC)
	defer shutdown()

	plugins := map[string]*structs.CSIInfo{
		"minnie": {PluginID: "minnie",
			Healthy:                  true,
			ControllerInfo:           &structs.CSIControllerInfo{},
			NodeInfo:                 &structs.CSINodeInfo{},
			RequiresControllerPlugin: true,
		},
	}
	state := srv.fsm.State()

	node1 := mock.Node()
	node1.Attributes["nomad.version"] = "0.11.0" // client RPCs not supported on early versions
	node1.CSIControllerPlugins = plugins
	node2 := mock.Node()
	node2.CSIControllerPlugins = plugins
	node2.ID = uuid.Generate()
	node3 := mock.Node()
	node3.ID = uuid.Generate()

	err := state.UpsertNode(structs.MsgTypeTestSetup, 1002, node1)
	require.NoError(t, err)
	err = state.UpsertNode(structs.MsgTypeTestSetup, 1003, node2)
	require.NoError(t, err)
	err = state.UpsertNode(structs.MsgTypeTestSetup, 1004, node3)
	require.NoError(t, err)

	ws := memdb.NewWatchSet()

	plugin, err := state.CSIPluginByID(ws, "minnie")
	require.NoError(t, err)

	clientCSI := NewClientCSIEndpoint(srv, nil)
	nodeIDs, err := clientCSI.clientIDsForController(plugin.ID)
	require.NoError(t, err)
	require.Equal(t, 1, len(nodeIDs))
	// only node1 has both the controller and a recent Nomad version
	require.Equal(t, nodeIDs[0], node1.ID)
}

// sets up a pair of servers, each with one client, and registers a plugin to the clients.
// returns a RPC client to the leader and a cleanup function.
func setupForward(t *testing.T) (rpc.ClientCodec, func()) {

	s1, cleanupS1 := TestServer(t, func(c *Config) { c.BootstrapExpect = 2 })
	s2, cleanupS2 := TestServer(t, func(c *Config) { c.BootstrapExpect = 2 })
	TestJoin(t, s1, s2)

	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)
	codec := rpcClient(t, s1)

	c1, cleanupC1 := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s1.config.RPCAddr.String()}
	})

	// Wait for client initialization
	select {
	case <-c1.Ready():
	case <-time.After(10 * time.Second):
		cleanupC1()
		cleanupS1()
		cleanupS2()
		t.Fatal("client timedout on initialize")
	}

	c2, cleanupC2 := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s2.config.RPCAddr.String()}
	})
	select {
	case <-c2.Ready():
	case <-time.After(10 * time.Second):
		cleanupC1()
		cleanupC2()
		cleanupS1()
		cleanupS2()
		t.Fatal("client timedout on initialize")
	}

	s1.nodeConnsLock.Lock()
	delete(s1.nodeConns, c2.NodeID())
	s1.nodeConnsLock.Unlock()

	s2.nodeConnsLock.Lock()
	delete(s2.nodeConns, c1.NodeID())
	s2.nodeConnsLock.Unlock()

	waitForNodes(t, s2, 1, 2)

	plugins := map[string]*structs.CSIInfo{
		"minnie": {PluginID: "minnie",
			Healthy:                  true,
			ControllerInfo:           &structs.CSIControllerInfo{},
			NodeInfo:                 &structs.CSINodeInfo{},
			RequiresControllerPlugin: true,
		},
	}

	// update w/ plugin
	node1 := c1.Node()
	node1.Attributes["nomad.version"] = "0.11.0" // client RPCs not supported on early versions
	node1.CSIControllerPlugins = plugins

	s1.fsm.state.UpsertNode(structs.MsgTypeTestSetup, 1000, node1)

	cleanup := func() {
		cleanupC1()
		cleanupC2()
		cleanupS2()
		cleanupS1()
	}

	return codec, cleanup
}

// sets up a single server with a client, and registers a plugin to the client.
func setupLocal(t *testing.T) (rpc.ClientCodec, func()) {
	var err error
	s1, cleanupS1 := TestServer(t, func(c *Config) { c.BootstrapExpect = 1 })

	testutil.WaitForLeader(t, s1.RPC)
	codec := rpcClient(t, s1)

	mockCSI := newMockClientCSI()
	mockCSI.NextValidateError = fmt.Errorf("no plugins registered for type")
	mockCSI.NextAttachError = fmt.Errorf("no plugins registered for type")
	mockCSI.NextDetachError = fmt.Errorf("no plugins registered for type")
	mockCSI.NextCreateError = fmt.Errorf("no plugins registered for type")
	mockCSI.NextDeleteError = fmt.Errorf("no plugins registered for type")
	mockCSI.NextListExternalError = fmt.Errorf("no plugins registered for type")
	mockCSI.NextCreateSnapshotError = fmt.Errorf("no plugins registered for type")
	mockCSI.NextDeleteSnapshotError = fmt.Errorf("no plugins registered for type")
	mockCSI.NextListExternalSnapshotsError = fmt.Errorf("no plugins registered for type")

	c1, cleanupC1 := client.TestClientWithRPCs(t,
		func(c *config.Config) {
			c.Servers = []string{s1.config.RPCAddr.String()}
		},
		map[string]interface{}{"CSI": mockCSI},
	)

	if err != nil {
		cleanupC1()
		cleanupS1()
		require.NoError(t, err, "could not setup test client")
	}

	node1 := c1.UpdateConfig(func(c *config.Config) {
		c.Node.Attributes["nomad.version"] = "0.11.0" // client RPCs not supported on early versions
	}).Node

	req := &structs.NodeRegisterRequest{
		Node:         node1,
		WriteRequest: structs.WriteRequest{Region: "global"},
	}
	var resp structs.NodeUpdateResponse
	err = c1.RPC("Node.Register", req, &resp)
	if err != nil {
		cleanupC1()
		cleanupS1()
		require.NoError(t, err, "could not register client node")
	}

	waitForNodes(t, s1, 1, 1)

	plugins := map[string]*structs.CSIInfo{
		"minnie": {PluginID: "minnie",
			Healthy:                  true,
			ControllerInfo:           &structs.CSIControllerInfo{},
			NodeInfo:                 &structs.CSINodeInfo{},
			RequiresControllerPlugin: true,
		},
	}

	// update w/ plugin
	node1 = c1.UpdateConfig(func(c *config.Config) {
		c.Node.CSIControllerPlugins = plugins
	}).Node
	s1.fsm.state.UpsertNode(structs.MsgTypeTestSetup, 1000, node1)

	cleanup := func() {
		cleanupC1()
		cleanupS1()
	}

	return codec, cleanup
}

// waitForNodes waits until the server is connected to connectedNodes
// clients and totalNodes clients are in the state store
func waitForNodes(t *testing.T, s *Server, connectedNodes, totalNodes int) {
	codec := rpcClient(t, s)

	testutil.WaitForResult(func() (bool, error) {
		connNodes := s.connectedNodes()
		if len(connNodes) != connectedNodes {
			return false, fmt.Errorf("expected %d connected nodes but found %d", connectedNodes, len(connNodes))
		}

		get := &structs.NodeListRequest{
			QueryOptions: structs.QueryOptions{Region: "global"},
		}
		var resp structs.NodeListResponse
		err := msgpackrpc.CallWithCodec(codec, "Node.List", get, &resp)
		if err != nil {
			return false, err
		}

		if err != nil {
			return false, fmt.Errorf("failed to list nodes: %v", err)
		}
		if len(resp.Nodes) != totalNodes {
			return false, fmt.Errorf("expected %d total nodes but found %d", totalNodes, len(resp.Nodes))
		}
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}
