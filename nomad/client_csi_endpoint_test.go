package nomad

import (
	"fmt"
	"net/rpc"
	"testing"
	"time"

	memdb "github.com/hashicorp/go-memdb"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/client"
	"github.com/hashicorp/nomad/client/config"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/require"
)

func TestClientCSIController_AttachVolume_Local(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	codec, cleanup := setupLocal(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerAttachVolumeRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerAttachVolume", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_AttachVolume_Forwarded(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	codec, cleanup := setupForward(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerAttachVolumeRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerAttachVolume", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_DetachVolume_Local(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	codec, cleanup := setupLocal(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerDetachVolumeRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerDetachVolume", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_DetachVolume_Forwarded(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	codec, cleanup := setupForward(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerDetachVolumeRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerDetachVolume", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_ValidateVolume_Local(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	codec, cleanup := setupLocal(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerValidateVolumeRequest{
		VolumeID:           "test",
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerValidateVolume", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSIController_ValidateVolume_Forwarded(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	codec, cleanup := setupForward(t)
	defer cleanup()

	req := &cstructs.ClientCSIControllerValidateVolumeRequest{
		VolumeID:           "test",
		CSIControllerQuery: cstructs.CSIControllerQuery{PluginID: "minnie"},
	}

	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerValidateVolume", req, &resp)
	require.NotNil(err)
	require.Contains(err.Error(), "no plugins registered for type")
}

func TestClientCSI_NodeForControllerPlugin(t *testing.T) {
	t.Parallel()
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

	err := state.UpsertNode(1002, node1)
	require.NoError(t, err)
	err = state.UpsertNode(1003, node2)
	require.NoError(t, err)
	err = state.UpsertNode(1004, node3)
	require.NoError(t, err)

	ws := memdb.NewWatchSet()

	plugin, err := state.CSIPluginByID(ws, "minnie")
	require.NoError(t, err)
	nodeIDs, err := srv.staticEndpoints.ClientCSI.clientIDsForController(plugin.ID)
	require.NoError(t, err)
	require.Equal(t, 1, len(nodeIDs))
	// only node1 has both the controller and a recent Nomad version
	require.Equal(t, nodeIDs[0], node1.ID)
}

// sets up a pair of servers, each with one client, and registers a plugin to the clients.
// returns a RPC client to the leader and a cleanup function.
func setupForward(t *testing.T) (rpc.ClientCodec, func()) {

	s1, cleanupS1 := TestServer(t, func(c *Config) { c.BootstrapExpect = 1 })

	testutil.WaitForLeader(t, s1.RPC)
	codec := rpcClient(t, s1)

	c1, cleanupC1 := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s1.config.RPCAddr.String()}
	})

	// Wait for client initialization
	select {
	case <-c1.Ready():
	case <-time.After(10 * time.Second):
		cleanupS1()
		cleanupC1()
		t.Fatal("client timedout on initialize")
	}

	waitForNodes(t, s1, 1, 1)

	s2, cleanupS2 := TestServer(t, func(c *Config) { c.BootstrapExpect = 2 })
	TestJoin(t, s1, s2)

	c2, cleanupC2 := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s2.config.RPCAddr.String()}
	})
	select {
	case <-c2.Ready():
	case <-time.After(10 * time.Second):
		cleanupS1()
		cleanupC1()
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

	s1.fsm.state.UpsertNode(1000, node1)

	cleanup := func() {
		cleanupS1()
		cleanupC1()
		cleanupS2()
		cleanupC2()
	}

	return codec, cleanup
}

// sets up a single server with a client, and registers a plugin to the client.
func setupLocal(t *testing.T) (rpc.ClientCodec, func()) {

	s1, cleanupS1 := TestServer(t, func(c *Config) { c.BootstrapExpect = 1 })

	testutil.WaitForLeader(t, s1.RPC)
	codec := rpcClient(t, s1)

	c1, cleanupC1 := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s1.config.RPCAddr.String()}
	})

	// Wait for client initialization
	select {
	case <-c1.Ready():
	case <-time.After(10 * time.Second):
		cleanupS1()
		cleanupC1()
		t.Fatal("client timedout on initialize")
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
	node1 := c1.Node()
	node1.Attributes["nomad.version"] = "0.11.0" // client RPCs not supported on early versions
	node1.CSIControllerPlugins = plugins

	s1.fsm.state.UpsertNode(1000, node1)

	cleanup := func() {
		cleanupS1()
		cleanupC1()
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
