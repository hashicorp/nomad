package nomad

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/hashicorp/raft"
)

func TestOperator_RaftGetConfiguration(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
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
			&structs.RaftServer{
				ID:      me.ID,
				Node:    fmt.Sprintf("%v.%v", s1.config.NodeName, s1.config.Region),
				Address: me.Address,
				Leader:  true,
				Voter:   true,
			},
		},
		Index: future.Index(),
	}
	if !reflect.DeepEqual(reply, expected) {
		t.Fatalf("bad: got %+v; want %+v", reply, expected)
	}
}

func TestOperator_RaftRemovePeerByAddress(t *testing.T) {
	t.Parallel()
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	testutil.WaitForLeader(t, s1.RPC)

	// Try to remove a peer that's not there.
	arg := structs.RaftPeerByAddressRequest{
		Address: raft.ServerAddress(fmt.Sprintf("127.0.0.1:%d", getPort())),
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
