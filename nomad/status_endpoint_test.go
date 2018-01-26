package nomad

import (
	"testing"

	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestStatusVersion(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)

	arg := &structs.GenericRequest{
		QueryOptions: structs.QueryOptions{
			Region:     "global",
			AllowStale: true,
		},
	}
	var out structs.VersionResponse
	if err := msgpackrpc.CallWithCodec(codec, "Status.Version", arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if out.Build == "" {
		t.Fatalf("bad: %#v", out)
	}
	if out.Versions[structs.ProtocolVersion] != ProtocolVersionMax {
		t.Fatalf("bad: %#v", out)
	}
	if out.Versions[structs.APIMajorVersion] != structs.ApiMajorVersion {
		t.Fatalf("bad: %#v", out)
	}
	if out.Versions[structs.APIMinorVersion] != structs.ApiMinorVersion {
		t.Fatalf("bad: %#v", out)
	}
}

func TestStatusPing(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)

	arg := struct{}{}
	var out struct{}
	if err := msgpackrpc.CallWithCodec(codec, "Status.Ping", arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestStatusLeader(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
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
