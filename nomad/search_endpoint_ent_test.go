// +build ent

package nomad

import (
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/stretchr/testify/assert"
)

func TestSearch_PrefixSearch_Quota(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	s := testServer(t, func(c *Config) {
		c.NumSchedulers = 0
	})

	defer s.Shutdown()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	qs := mock.QuotaSpec()
	assert.Nil(s.fsm.State().UpsertQuotaSpecs(2000, []*structs.QuotaSpec{qs}))

	prefix := qs.Name[:len(qs.Name)-2]

	req := &structs.SearchRequest{
		Prefix:  prefix,
		Context: structs.Quotas,
		QueryOptions: structs.QueryOptions{
			Region: "global",
		},
	}

	var resp structs.SearchResponse
	if err := msgpackrpc.CallWithCodec(codec, "Search.PrefixSearch", req, &resp); err != nil {
		t.Fatalf("err: %v", err)
	}

	assert.Equal(1, len(resp.Matches[structs.Quotas]))
	assert.Equal(qs.Name, resp.Matches[structs.Quotas][0])
	assert.Equal(resp.Truncations[structs.Quotas], false)

	assert.Equal(uint64(2000), resp.Index)
}
