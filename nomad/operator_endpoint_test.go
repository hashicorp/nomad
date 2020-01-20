package nomad

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper/freeport"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperator_RaftGetConfiguration(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	arg := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: s1.config.Region,
		},
	}
	var reply structs.RaftConfigurationResponse
	if err := msgpackrpc.CallWithCodec(codec, "Operator.RaftGetConfiguration", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	future := s1.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(future.Configuration().Servers) != 1 {
		t.Fatalf("bad: %v", future.Configuration().Servers)
	}
	me := future.Configuration().Servers[0]
	expected := structs.RaftConfigurationResponse{
		Servers: []*structs.RaftServer{
			{
				ID:           me.ID,
				Node:         fmt.Sprintf("%v.%v", s1.config.NodeName, s1.config.Region),
				Address:      me.Address,
				Leader:       true,
				Voter:        true,
				RaftProtocol: fmt.Sprintf("%d", s1.config.RaftConfig.ProtocolVersion),
			},
		},
		Index: future.Index(),
	}
	if !reflect.DeepEqual(reply, expected) {
		t.Fatalf("bad: got %+v; want %+v", reply, expected)
	}
}

func TestOperator_RaftGetConfiguration_ACL(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)
	state := s1.fsm.State()

	// Create ACL token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid", mock.NodePolicy(acl.PolicyWrite))

	arg := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: s1.config.Region,
		},
	}

	// Try with no token and expect permission denied
	{
		var reply structs.RaftConfigurationResponse
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftGetConfiguration", &arg, &reply)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect permission denied
	{
		arg.AuthToken = invalidToken.SecretID
		var reply structs.RaftConfigurationResponse
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftGetConfiguration", &arg, &reply)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Use management token
	{
		arg.AuthToken = root.SecretID
		var reply structs.RaftConfigurationResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Operator.RaftGetConfiguration", &arg, &reply))

		future := s1.raft.GetConfiguration()
		assert.Nil(future.Error())
		assert.Len(future.Configuration().Servers, 1)

		me := future.Configuration().Servers[0]
		expected := structs.RaftConfigurationResponse{
			Servers: []*structs.RaftServer{
				{
					ID:           me.ID,
					Node:         fmt.Sprintf("%v.%v", s1.config.NodeName, s1.config.Region),
					Address:      me.Address,
					Leader:       true,
					Voter:        true,
					RaftProtocol: fmt.Sprintf("%d", s1.config.RaftConfig.ProtocolVersion),
				},
			},
			Index: future.Index(),
		}
		assert.Equal(expected, reply)
	}
}

func TestOperator_RaftRemovePeerByAddress(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = raft.ProtocolVersion(2)
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	ports := freeport.MustTake(1)
	defer freeport.Return(ports)

	// Try to remove a peer that's not there.
	arg := structs.RaftPeerByAddressRequest{
		Address: raft.ServerAddress(fmt.Sprintf("127.0.0.1:%d", ports[0])),
	}
	arg.Region = s1.config.Region
	var reply struct{}
	err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByAddress", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "not found in the Raft configuration") {
		t.Fatalf("err: %v", err)
	}

	// Add it manually to Raft.
	{
		future := s1.raft.AddPeer(arg.Address)
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Make sure it's there.
	{
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		configuration := future.Configuration()
		if len(configuration.Servers) != 2 {
			t.Fatalf("bad: %v", configuration)
		}
	}

	// Remove it, now it should go through.
	if err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByAddress", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it's not there.
	{
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		configuration := future.Configuration()
		if len(configuration.Servers) != 1 {
			t.Fatalf("bad: %v", configuration)
		}
	}
}

func TestOperator_RaftRemovePeerByAddress_ACL(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = raft.ProtocolVersion(2)
	})

	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)
	state := s1.fsm.State()

	// Create ACL token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid", mock.NodePolicy(acl.PolicyWrite))

	ports := freeport.MustTake(1)
	defer freeport.Return(ports)

	arg := structs.RaftPeerByAddressRequest{
		Address: raft.ServerAddress(fmt.Sprintf("127.0.0.1:%d", ports[0])),
	}
	arg.Region = s1.config.Region

	// Add peer manually to Raft.
	{
		future := s1.raft.AddPeer(arg.Address)
		assert.Nil(future.Error())
	}

	var reply struct{}

	// Try with no token and expect permission denied
	{
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByAddress", &arg, &reply)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect permission denied
	{
		arg.AuthToken = invalidToken.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByAddress", &arg, &reply)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a management token
	{
		arg.AuthToken = root.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByAddress", &arg, &reply)
		assert.Nil(err)
	}
}

func TestOperator_RaftRemovePeerByID(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Try to remove a peer that's not there.
	arg := structs.RaftPeerByIDRequest{
		ID: raft.ServerID("e35bde83-4e9c-434f-a6ef-453f44ee21ea"),
	}
	arg.Region = s1.config.Region
	var reply struct{}
	err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByID", &arg, &reply)
	if err == nil || !strings.Contains(err.Error(), "not found in the Raft configuration") {
		t.Fatalf("err: %v", err)
	}

	ports := freeport.MustTake(1)
	defer freeport.Return(ports)

	// Add it manually to Raft.
	{
		future := s1.raft.AddVoter(arg.ID, raft.ServerAddress(fmt.Sprintf("127.0.0.1:%d", ports[0])), 0, 0)
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Make sure it's there.
	{
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		configuration := future.Configuration()
		if len(configuration.Servers) != 2 {
			t.Fatalf("bad: %v", configuration)
		}
	}

	// Remove it, now it should go through.
	if err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByID", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it's not there.
	{
		future := s1.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			t.Fatalf("err: %v", err)
		}
		configuration := future.Configuration()
		if len(configuration.Servers) != 1 {
			t.Fatalf("bad: %v", configuration)
		}
	}
}

func TestOperator_RaftRemovePeerByID_ACL(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)
	state := s1.fsm.State()

	// Create ACL token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid", mock.NodePolicy(acl.PolicyWrite))

	arg := structs.RaftPeerByIDRequest{
		ID: raft.ServerID("e35bde83-4e9c-434f-a6ef-453f44ee21ea"),
	}
	arg.Region = s1.config.Region

	ports := freeport.MustTake(1)
	defer freeport.Return(ports)

	// Add peer manually to Raft.
	{
		future := s1.raft.AddVoter(arg.ID, raft.ServerAddress(fmt.Sprintf("127.0.0.1:%d", ports[0])), 0, 0)
		assert.Nil(future.Error())
	}

	var reply struct{}

	// Try with no token and expect permission denied
	{
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByID", &arg, &reply)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect permission denied
	{
		arg.AuthToken = invalidToken.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByID", &arg, &reply)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a management token
	{
		arg.AuthToken = root.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.RaftRemovePeerByID", &arg, &reply)
		assert.Nil(err)
	}
}

func TestOperator_SchedulerGetConfiguration(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.Build = "0.9.0+unittest"
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	arg := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: s1.config.Region,
		},
	}
	var reply structs.SchedulerConfigurationResponse
	if err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerGetConfiguration", &arg, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}
	require := require.New(t)
	require.NotZero(reply.Index)
	require.True(reply.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
}

func TestOperator_SchedulerSetConfiguration(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, func(c *Config) {
		c.Build = "0.9.0+unittest"
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	require := require.New(t)

	// Disable preemption
	arg := structs.SchedulerSetConfigRequest{
		Config: structs.SchedulerConfiguration{
			PreemptionConfig: structs.PreemptionConfig{
				SystemSchedulerEnabled: false,
			},
		},
	}
	arg.Region = s1.config.Region

	var setResponse structs.SchedulerSetConfigurationResponse
	err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerSetConfiguration", &arg, &setResponse)
	require.Nil(err)
	require.NotZero(setResponse.Index)

	// Read and verify that preemption is disabled
	readConfig := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: s1.config.Region,
		},
	}
	var reply structs.SchedulerConfigurationResponse
	if err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerGetConfiguration", &readConfig, &reply); err != nil {
		t.Fatalf("err: %v", err)
	}

	require.NotZero(reply.Index)
	require.False(reply.SchedulerConfig.PreemptionConfig.SystemSchedulerEnabled)
}

func TestOperator_SchedulerGetConfiguration_ACL(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
		c.Build = "0.9.0+unittest"
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create ACL token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid", mock.NodePolicy(acl.PolicyWrite))

	arg := structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region: s1.config.Region,
		},
	}
	require := require.New(t)
	var reply structs.SchedulerConfigurationResponse

	// Try with no token and expect permission denied
	{
		err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerGetConfiguration", &arg, &reply)
		require.NotNil(err)
		require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect permission denied
	{
		arg.AuthToken = invalidToken.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerGetConfiguration", &arg, &reply)
		require.NotNil(err)
		require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with root token, should succeed
	{
		arg.AuthToken = root.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerGetConfiguration", &arg, &reply)
		require.Nil(err)
	}

}

func TestOperator_SchedulerSetConfiguration_ACL(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, func(c *Config) {
		c.RaftConfig.ProtocolVersion = 3
		c.Build = "0.9.0+unittest"
	})
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	state := s1.fsm.State()

	// Create ACL token
	invalidToken := mock.CreatePolicyAndToken(t, state, 1001, "test-invalid", mock.NodePolicy(acl.PolicyWrite))

	arg := structs.SchedulerSetConfigRequest{
		Config: structs.SchedulerConfiguration{
			PreemptionConfig: structs.PreemptionConfig{
				SystemSchedulerEnabled: true,
			},
		},
	}
	arg.Region = s1.config.Region

	require := require.New(t)
	var reply structs.SchedulerSetConfigurationResponse

	// Try with no token and expect permission denied
	{
		err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerSetConfiguration", &arg, &reply)
		require.NotNil(err)
		require.Equal(structs.ErrPermissionDenied.Error(), err.Error())
	}

	// Try with an invalid token and expect permission denied
	{
		arg.AuthToken = invalidToken.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerSetConfiguration", &arg, &reply)
		require.NotNil(err)
		require.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with root token, should succeed
	{
		arg.AuthToken = root.SecretID
		err := msgpackrpc.CallWithCodec(codec, "Operator.SchedulerSetConfiguration", &arg, &reply)
		require.Nil(err)
	}

}
