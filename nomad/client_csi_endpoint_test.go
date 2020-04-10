package nomad

import (
	"testing"

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

	// Start a server and client
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.config.RPCAddr.String()}
	})
	defer cleanupC()

	testutil.WaitForResult(func() (bool, error) {
		nodes := s.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		require.Fail("should have a client")
	})

	req := &cstructs.ClientCSIControllerAttachVolumeRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{ControllerNodeID: c.NodeID()},
	}

	// Fetch the response
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerAttachVolume", req, &resp)
	require.NotNil(err)
	// Should recieve an error from the client endpoint
	require.Contains(err.Error(), "must specify plugin name to dispense")
}

func TestClientCSIController_AttachVolume_Forwarded(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s1, cleanupS1 := TestServer(t, func(c *Config) { c.BootstrapExpect = 2 })
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) { c.BootstrapExpect = 2 })
	defer cleanupS2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)
	codec := rpcClient(t, s2)

	c, cleanupC := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s2.config.RPCAddr.String()}
		c.GCDiskUsageThreshold = 100.0
	})
	defer cleanupC()

	testutil.WaitForResult(func() (bool, error) {
		nodes := s2.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		require.Fail("should have a client")
	})

	// Force remove the connection locally in case it exists
	s1.nodeConnsLock.Lock()
	delete(s1.nodeConns, c.NodeID())
	s1.nodeConnsLock.Unlock()

	req := &cstructs.ClientCSIControllerAttachVolumeRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{ControllerNodeID: c.NodeID()},
	}

	// Fetch the response
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerAttachVolume", req, &resp)
	require.NotNil(err)
	// Should recieve an error from the client endpoint
	require.Contains(err.Error(), "must specify plugin name to dispense")
}

func TestClientCSIController_DetachVolume_Local(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s, cleanupS := TestServer(t, nil)
	defer cleanupS()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	c, cleanupC := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.config.RPCAddr.String()}
	})
	defer cleanupC()

	testutil.WaitForResult(func() (bool, error) {
		nodes := s.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		require.Fail("should have a client")
	})

	req := &cstructs.ClientCSIControllerDetachVolumeRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{ControllerNodeID: c.NodeID()},
	}

	// Fetch the response
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerDetachVolume", req, &resp)
	require.NotNil(err)
	// Should recieve an error from the client endpoint
	require.Contains(err.Error(), "must specify plugin name to dispense")
}

func TestClientCSIController_DetachVolume_Forwarded(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// Start a server and client
	s1, cleanupS1 := TestServer(t, func(c *Config) { c.BootstrapExpect = 2 })
	defer cleanupS1()
	s2, cleanupS2 := TestServer(t, func(c *Config) { c.BootstrapExpect = 2 })
	defer cleanupS2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)
	codec := rpcClient(t, s2)

	c, cleanupC := client.TestClient(t, func(c *config.Config) {
		c.Servers = []string{s2.config.RPCAddr.String()}
		c.GCDiskUsageThreshold = 100.0
	})
	defer cleanupC()

	testutil.WaitForResult(func() (bool, error) {
		nodes := s2.connectedNodes()
		return len(nodes) == 1, nil
	}, func(err error) {
		require.Fail("should have a client")
	})

	// Force remove the connection locally in case it exists
	s1.nodeConnsLock.Lock()
	delete(s1.nodeConns, c.NodeID())
	s1.nodeConnsLock.Unlock()

	req := &cstructs.ClientCSIControllerDetachVolumeRequest{
		CSIControllerQuery: cstructs.CSIControllerQuery{ControllerNodeID: c.NodeID()},
	}

	// Fetch the response
	var resp structs.GenericResponse
	err := msgpackrpc.CallWithCodec(codec, "ClientCSI.ControllerDetachVolume", req, &resp)
	require.NotNil(err)
	// Should recieve an error from the client endpoint
	require.Contains(err.Error(), "must specify plugin name to dispense")
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
	nodeID, err := srv.staticEndpoints.ClientCSI.nodeForController(plugin.ID, "")

	// only node1 has both the controller and a recent Nomad version
	require.Equal(t, nodeID, node1.ID)
}
