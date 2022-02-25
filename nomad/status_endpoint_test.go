package nomad

import (
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusPing(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)

	arg := struct{}{}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Status.Ping", arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestStatusLeader(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	arg := &structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region:     "global",
			AllowStale: true,
		},
	}
	var leader string
	if err := msgpackrpc.CallWithCodec(codec, "Status.Leader", arg, &leader); err != nil {
		t.Fatalf("err: %v", err)
	}
	if leader == "" {
		t.Fatalf("unexpected leader: %v", leader)
	}
}

func TestStatusPeers(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)

	arg := &structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region:     "global",
			AllowStale: true,
		},
	}
	var peers []string
	if err := msgpackrpc.CallWithCodec(codec, "Status.Peers", arg, &peers); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("no peers: %v", peers)
	}
}

func TestStatusMembers(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	assert := assert.New(t)

	arg := &structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region:     "global",
			AllowStale: true,
		},
	}

	var out structs.ServerMembersResponse
	assert.Nil(msgpackrpc.CallWithCodec(codec, "Status.Members", arg, &out))
	assert.Len(out.Members, 1)
}

func TestStatusMembers_ACL(t *testing.T) {
	t.Parallel()

	s1, root, cleanupS1 := TestACLServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	assert := assert.New(t)
	state := s1.fsm.State()

	// Create the namespace policy and tokens
	validToken := mock.CreatePolicyAndToken(t, state, 1001, "test-valid", mock.NodePolicy(acl.PolicyRead))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1003, "test-invalid", mock.AgentPolicy(acl.PolicyRead))

	arg := &structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region:     "global",
			AllowStale: true,
		},
	}

	// Try without a token and expect failure
	{
		var out structs.ServerMembersResponse
		err := msgpackrpc.CallWithCodec(codec, "Status.Members", arg, &out)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with an invalid token and expect failure
	{
		arg.AuthToken = invalidToken.SecretID
		var out structs.ServerMembersResponse
		err := msgpackrpc.CallWithCodec(codec, "Status.Members", arg, &out)
		assert.NotNil(err)
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a valid token
	{
		arg.AuthToken = validToken.SecretID
		var out structs.ServerMembersResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Status.Members", arg, &out))
		assert.Len(out.Members, 1)
	}

	// Try with a management token
	{
		arg.AuthToken = root.SecretID
		var out structs.ServerMembersResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Status.Members", arg, &out))
		assert.Len(out.Members, 1)
	}
}

func TestStatus_HasClientConn(t *testing.T) {
	t.Parallel()

	s1, cleanupS1 := TestServer(t, nil)
	defer cleanupS1()
	codec := rpcClient(t, s1)
	require := require.New(t)

	arg := &structs.NodeSpecificRequest{
		QueryOptions: structs.QueryOptions{
			Region:     "global",
			AllowStale: true,
		},
	}

	// Try without setting a node id
	var out structs.NodeConnQueryResponse
	require.NotNil(msgpackrpc.CallWithCodec(codec, "Status.HasNodeConn", arg, &out))

	// Set a bad node id
	arg.NodeID = uuid.Generate()
	var out2 structs.NodeConnQueryResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Status.HasNodeConn", arg, &out2))
	require.False(out2.Connected)

	// Create a connection on that node
	s1.addNodeConn(&RPCContext{
		NodeID: arg.NodeID,
	})
	var out3 structs.NodeConnQueryResponse
	require.Nil(msgpackrpc.CallWithCodec(codec, "Status.HasNodeConn", arg, &out3))
	require.True(out3.Connected)
	require.NotZero(out3.Established)
}
