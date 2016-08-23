package nomad

import (
	"encoding/base64"
	"fmt"
  "testing"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/testutil"
)

const (
	key = "H1dfkSZOVnP/JUnaBfTzXg=="
)

func TestKeyringOperationEndpoint_SingleNodeCluster(t *testing.T) {
	keyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	s1 := testServer(t, func(c *Config) {
		c.SerfConfig.MemberlistConfig.SecretKey = keyBytes
	})
	defer s1.Shutdown()
	codec := rpcClient(t, s1)
	defer codec.Close()

	testutil.WaitForLeader(t, s1.RPC)

	var out structs.KeyringResponses
	req := &structs.KeyringRequest{
		Operation:  structs.KeyringList,
		Datacenter: "dc1",
	}
	if err := msgpackrpc.CallWithCodec(codec, "KeyringOperation.Execute", req, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// 1 response (wan pool) from single-node cluster
	if len(out.Responses) != 1 {
		t.Fatalf("bad: %#v", out)
	}
	if _, ok := out.Responses[0].Keys[key]; !ok {
		t.Fatalf("bad: %#v", out)
	}
	if !out.Responses[0].WAN {
		t.Fatalf("should have one wan response")
	}
}


func TestKeyringOperationEndpoint_CrossDCCluster(t *testing.T) {
	keyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	s1 := testServer(t, func(c *Config) {
		c.SerfConfig.MemberlistConfig.SecretKey = keyBytes
	})
	defer s1.Shutdown()

	testutil.WaitForLeader(t, s1.RPC)

	// Start a second agent to test cross-dc queries
	s2 := testServer(t, func(c *Config) {
		c.SerfConfig.MemberlistConfig.SecretKey = keyBytes
		c.Datacenter = "dc2"
	})
	defer s2.Shutdown()
	codec := rpcClient(t, s2)
	defer codec.Close()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfConfig.MemberlistConfig.BindPort)
	if _, err := s2.Join([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	var out2 structs.KeyringResponses
	req2 := &structs.KeyringRequest{
		Operation: structs.KeyringList,
	}
	if err := msgpackrpc.CallWithCodec(codec, "KeyringOperation.Execute", req2, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}

	// 1 response from WAN in two-node cluster
	if len(out2.Responses) != 1 {
		t.Fatalf("bad: %#v", out2)
	}
	if !out2.Responses[0].WAN {
		t.Fatalf("should have one wan response")
	}
}
