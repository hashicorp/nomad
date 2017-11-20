// +build pro ent

package nomad

import (
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/acl"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestClientEndpoint_GetAllocs_ACL_Pro(t *testing.T) {
	t.Parallel()
	s1, root := testACLServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)
	assert := assert.New(t)

	// Create the namespaces
	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	ns1.Name = "altnamespace"
	ns2.Name = "should-only-be-displayed-for-root-ns"

	// Create the allocs
	allocDefaultNS := mock.Alloc()
	allocAltNS := mock.Alloc()
	allocAltNS.Namespace = ns1.Name
	allocOtherNS := mock.Alloc()
	allocOtherNS.Namespace = ns2.Name

	node := mock.Node()
	allocDefaultNS.NodeID = node.ID
	allocAltNS.NodeID = node.ID
	allocOtherNS.NodeID = node.ID
	state := s1.fsm.State()
	assert.Nil(state.UpsertNamespaces(1, []*structs.Namespace{ns1, ns2}), "UpsertNamespaces")
	assert.Nil(state.UpsertNode(2, node), "UpsertNode")
	assert.Nil(state.UpsertJobSummary(3, mock.JobSummary(allocDefaultNS.JobID)), "UpsertJobSummary")
	assert.Nil(state.UpsertJobSummary(4, mock.JobSummary(allocAltNS.JobID)), "UpsertJobSummary")
	assert.Nil(state.UpsertJobSummary(5, mock.JobSummary(allocOtherNS.JobID)), "UpsertJobSummary")
	allocs := []*structs.Allocation{allocDefaultNS, allocAltNS, allocOtherNS}
	assert.Nil(state.UpsertAllocs(6, allocs), "UpsertAllocs")

	// Create the namespace policy and tokens
	validDefaultToken := mock.CreatePolicyAndToken(t, state, 1001, "test-default-valid", mock.NodePolicy(acl.PolicyRead)+
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))
	validNoNSToken := mock.CreatePolicyAndToken(t, state, 1003, "test-alt-valid", mock.NodePolicy(acl.PolicyRead))
	invalidToken := mock.CreatePolicyAndToken(t, state, 1004, "test-invalid",
		mock.NamespacePolicy(structs.DefaultNamespace, "", []string{acl.NamespaceCapabilityReadJob}))

	// Lookup the node without a token and expect failure
	req := &structs.NodeSpecificRequest{
		NodeID:       node.ID,
		QueryOptions: structs.QueryOptions{Region: "global"},
	}
	{
		var resp structs.NodeAllocsResponse
		err := msgpackrpc.CallWithCodec(codec, "Node.GetAllocs", req, &resp)
		assert.NotNil(err, "RPC")
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a valid token for the default namespace
	req.AuthToken = validDefaultToken.SecretID
	{
		var resp structs.NodeAllocsResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.GetAllocs", req, &resp), "RPC")
		assert.Len(resp.Allocs, 1)
		assert.Equal(allocDefaultNS.ID, resp.Allocs[0].ID)
	}

	// Try with a valid token for a namespace with no allocs on this node
	req.AuthToken = validNoNSToken.SecretID
	{
		var resp structs.NodeAllocsResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.GetAllocs", req, &resp), "RPC")
		assert.Len(resp.Allocs, 0)
	}

	// Try with a invalid token
	req.AuthToken = invalidToken.SecretID
	{
		var resp structs.NodeAllocsResponse
		err := msgpackrpc.CallWithCodec(codec, "Node.GetAllocs", req, &resp)
		assert.NotNil(err, "RPC")
		assert.Equal(err.Error(), structs.ErrPermissionDenied.Error())
	}

	// Try with a root token
	req.AuthToken = root.SecretID
	{
		var resp structs.NodeAllocsResponse
		assert.Nil(msgpackrpc.CallWithCodec(codec, "Node.GetAllocs", req, &resp), "RPC")
		assert.Len(resp.Allocs, 3)
		for _, alloc := range resp.Allocs {
			switch alloc.ID {
			case allocDefaultNS.ID, allocAltNS.ID, allocOtherNS.ID:
				// expected
			default:
				t.Errorf("unexpected alloc %q for namespace %q", alloc.ID, alloc.Namespace)
			}
		}
	}
}
