// +build pro ent

package nomad

import (
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestSearch_PrefixSearch_Namespace(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	ns := mock.Namespace()
	assert.Nil(s.fsm.State().UpsertNamespaces(2000, []*structs.Namespace{ns}))

	prefix := ns.Name[:len(ns.Name)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Namespaces,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Namespaces]))
	assert.Equal(ns.Name, resp.Matches[structs.Namespaces][0])
	assert.Equal(resp.Truncations[structs.Namespaces], false)

	assert.Equal(uint64(2000), resp.Index)
}
