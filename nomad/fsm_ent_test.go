// +build ent

package nomad

import (
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestFSM_UpsertSentinelPolicies(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)

	policy := mock.SentinelPolicy()
	req := structs.SentinelPolicyUpsertRequest{
		Policies: []*structs.SentinelPolicy{policy},
	}
	buf, err := structs.Encode(structs.SentinelPolicyUpsertRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().SentinelPolicyByName(ws, policy.Name)
	assert.Nil(t, err)
	assert.NotNil(t, out)
}

func TestFSM_DeleteSentinelPolicies(t *testing.T) {
	t.Parallel()
	fsm := testFSM(t)

	policy := mock.SentinelPolicy()
	err := fsm.State().UpsertSentinelPolicies(1000, []*structs.SentinelPolicy{policy})
	assert.Nil(t, err)

	req := structs.SentinelPolicyDeleteRequest{
		Names: []string{policy.Name},
	}
	buf, err := structs.Encode(structs.SentinelPolicyDeleteRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(makeLog(buf))
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().SentinelPolicyByName(ws, policy.Name)
	assert.Nil(t, err)
	assert.Nil(t, out)
}

func TestFSM_SnapshotRestore_SentinelPolicy(t *testing.T) {
	t.Parallel()
	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	p1 := mock.SentinelPolicy()
	p2 := mock.SentinelPolicy()
	state.UpsertSentinelPolicies(1000, []*structs.SentinelPolicy{p1, p2})

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.SentinelPolicyByName(ws, p1.Name)
	out2, _ := state2.SentinelPolicyByName(ws, p2.Name)
	assert.Equal(t, p1, out1)
	assert.Equal(t, p2, out2)
}
