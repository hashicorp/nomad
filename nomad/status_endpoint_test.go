package nomad

import (
	"net"
	"net/rpc"
	"testing"
	"time"

	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/testutil"
)

func rpcClient(t *testing.T, s *Server) rpc.ClientCodec {
	addr := s.config.RPCAddr
	conn, err := net.DialTimeout("tcp", addr.String(), time.Second)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// Write the Consul RPC byte to set the mode
	conn.Write([]byte{byte(rpcNomad)})
	return msgpackrpc.NewClientCodec(conn)
}

func TestStatusPing(t *testing.T) {
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
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)

	rpcFn := func(method string, arg, resp interface{}) error {
		return msgpackrpc.CallWithCodec(codec, method, arg, resp)
	}
	testutil.WaitForLeader(t, rpcFn)

	arg := struct{}{}
	var leader string
	if err := msgpackrpc.CallWithCodec(codec, "Status.Leader", arg, &leader); err != nil {
		t.Fatalf("err: %v", err)
	}
	if leader == "" {
		t.Fatalf("unexpected leader: %v", leader)
	}
}

func TestStatusPeers(t *testing.T) {
	s1 := testServer(t, nil)
	defer s1.Shutdown()
	codec := rpcClient(t, s1)

	arg := struct{}{}
	var peers []string
	if err := msgpackrpc.CallWithCodec(codec, "Status.Peers", arg, &peers); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("no peers: %v", peers)
	}
}
