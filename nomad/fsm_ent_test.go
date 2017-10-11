// +build ent

package nomad

import (
	"fmt"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
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

func TestFSM_UpsertQuotaSpecs(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	fsm := testFSM(t)

	spec := mock.QuotaSpec()
	req := structs.QuotaSpecUpsertRequest{
		Quotas: []*structs.QuotaSpec{spec},
	}
	buf, err := structs.Encode(structs.QuotaSpecUpsertRequestType, req)
	assert.Nil(err)

	resp := fsm.Apply(makeLog(buf))
	assert.Nil(resp)

	// Verify we are registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().QuotaSpecByName(ws, spec.Name)
	assert.Nil(err)
	assert.NotNil(out)

	usage, err := fsm.State().QuotaUsageByName(ws, spec.Name)
	assert.Nil(err)
	assert.NotNil(usage)
}

// This test checks that unblocks are triggered when a quota changes
func TestFSM_UpsertQuotaSpecs_Modify(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	fsm := testFSM(t)
	state := fsm.State()
	fsm.blockedEvals.SetEnabled(true)

	// Create a quota specs
	qs1 := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(1, []*structs.QuotaSpec{qs1}))

	// Block an eval for that namespace
	e := mock.Eval()
	e.QuotaLimitReached = qs1.Name
	fsm.blockedEvals.Block(e)

	bstats := fsm.blockedEvals.Stats()
	assert.Equal(1, bstats.TotalBlocked)
	assert.Equal(1, bstats.TotalQuotaLimit)

	// Update the namespace to use the new spec
	qs2 := qs1.Copy()
	req := structs.QuotaSpecUpsertRequest{
		Quotas: []*structs.QuotaSpec{qs2},
	}
	buf, err := structs.Encode(structs.QuotaSpecUpsertRequestType, req)
	assert.Nil(err)
	assert.Nil(fsm.Apply(makeLog(buf)))

	// Verify we unblocked
	testutil.WaitForResult(func() (bool, error) {
		bStats := fsm.blockedEvals.Stats()
		if bStats.TotalBlocked != 0 {
			return false, fmt.Errorf("bad: %#v", bStats)
		}
		if bStats.TotalQuotaLimit != 0 {
			return false, fmt.Errorf("bad: %#v", bStats)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestFSM_DeleteQuotaSpecs(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	fsm := testFSM(t)

	spec := mock.QuotaSpec()
	assert.Nil(fsm.State().UpsertQuotaSpecs(1000, []*structs.QuotaSpec{spec}))

	req := structs.QuotaSpecDeleteRequest{
		Names: []string{spec.Name},
	}
	buf, err := structs.Encode(structs.QuotaSpecDeleteRequestType, req)
	assert.Nil(err)

	resp := fsm.Apply(makeLog(buf))
	assert.Nil(resp)

	// Verify we are NOT registered
	ws := memdb.NewWatchSet()
	out, err := fsm.State().QuotaSpecByName(ws, spec.Name)
	assert.Nil(err)
	assert.Nil(out)

	usage, err := fsm.State().QuotaUsageByName(ws, spec.Name)
	assert.Nil(err)
	assert.Nil(usage)
}

func TestFSM_SnapshotRestore_QuotaSpec(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs1, qs2}))

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.QuotaSpecByName(ws, qs1.Name)
	out2, _ := state2.QuotaSpecByName(ws, qs2.Name)
	assert.Equal(qs1, out1)
	assert.Equal(qs2, out2)
}

func TestFSM_SnapshotRestore_QuotaUsage(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	// Add some state
	fsm := testFSM(t)
	state := fsm.State()
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(999, []*structs.QuotaSpec{qs1, qs2}))
	qu1 := mock.QuotaUsage()
	qu2 := mock.QuotaUsage()
	qu1.Name = qs1.Name
	qu2.Name = qs2.Name
	assert.Nil(state.UpsertQuotaUsages(1000, []*structs.QuotaUsage{qu1, qu2}))

	// Verify the contents
	fsm2 := testSnapshotRestore(t, fsm)
	state2 := fsm2.State()
	ws := memdb.NewWatchSet()
	out1, _ := state2.QuotaUsageByName(ws, qu1.Name)
	out2, _ := state2.QuotaUsageByName(ws, qu2.Name)
	assert.Equal(qu1, out1)
	assert.Equal(qu2, out2)
}

// This test checks that unblocks are triggered when an alloc is updated and it
// has an associated quota.
func TestFSM_AllocClientUpdate_Quota(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	fsm := testFSM(t)
	state := fsm.State()
	fsm.blockedEvals.SetEnabled(true)

	// Create a quota specs
	qs1 := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(1, []*structs.QuotaSpec{qs1}))

	// Create a namespace
	ns := mock.Namespace()
	assert.Nil(state.UpsertNamespaces(2, []*structs.Namespace{ns}))

	// Create the node
	node := mock.Node()
	state.UpsertNode(3, node)

	// Block an eval for that namespace
	e := mock.Eval()
	e.Namespace = ns.Name
	e.QuotaLimitReached = qs1.Name
	fsm.blockedEvals.Block(e)

	bstats := fsm.blockedEvals.Stats()
	assert.Equal(1, bstats.TotalBlocked)
	assert.Equal(1, bstats.TotalQuotaLimit)

	// Create an alloc to update
	alloc := mock.Alloc()
	alloc.Namespace = ns.Name
	alloc.NodeID = node.ID
	alloc2 := mock.Alloc()
	alloc2.Namespace = ns.Name
	alloc2.NodeID = node.ID
	state.UpsertAllocs(10, []*structs.Allocation{alloc, alloc2})

	clientAlloc := alloc.Copy()
	clientAlloc.ClientStatus = structs.AllocClientStatusComplete
	update2 := &structs.Allocation{
		ID:           alloc2.ID,
		NodeID:       node.ID,
		Namespace:    ns.Name,
		ClientStatus: structs.AllocClientStatusRunning,
	}

	req := structs.AllocUpdateRequest{
		Alloc: []*structs.Allocation{clientAlloc, update2},
	}
	buf, err := structs.Encode(structs.AllocClientUpdateRequestType, req)
	assert.Nil(err)

	resp := fsm.Apply(makeLog(buf))
	assert.Nil(resp)

	// Verify we unblocked
	testutil.WaitForResult(func() (bool, error) {
		bStats := fsm.blockedEvals.Stats()
		if bStats.TotalBlocked != 0 {
			return false, fmt.Errorf("bad: %#v", bStats)
		}
		if bStats.TotalQuotaLimit != 0 {
			return false, fmt.Errorf("bad: %#v", bStats)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

// This test checks that unblocks are triggered when a namespace changes its
// quota
func TestFSM_UpsertNamespaces_ModifyQuota(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()
	fsm := testFSM(t)
	state := fsm.State()
	fsm.blockedEvals.SetEnabled(true)

	// Create two quota specs
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(1, []*structs.QuotaSpec{qs1, qs2}))

	// Create a namepace
	ns1 := mock.Namespace()
	ns1.Quota = qs1.Name
	assert.Nil(state.UpsertNamespaces(2, []*structs.Namespace{ns1}))

	// Block an eval for that namespace
	e := mock.Eval()
	e.QuotaLimitReached = qs1.Name
	fsm.blockedEvals.Block(e)

	bstats := fsm.blockedEvals.Stats()
	assert.Equal(1, bstats.TotalBlocked)
	assert.Equal(1, bstats.TotalQuotaLimit)

	// Update the namespace to use the new spec
	ns2 := ns1.Copy()
	ns2.Quota = qs2.Name
	req := structs.NamespaceUpsertRequest{
		Namespaces: []*structs.Namespace{ns2},
	}
	buf, err := structs.Encode(structs.NamespaceUpsertRequestType, req)
	assert.Nil(err)
	assert.Nil(fsm.Apply(makeLog(buf)))

	// Verify we unblocked
	testutil.WaitForResult(func() (bool, error) {
		bStats := fsm.blockedEvals.Stats()
		if bStats.TotalBlocked != 0 {
			return false, fmt.Errorf("bad: %#v", bStats)
		}
		if bStats.TotalQuotaLimit != 0 {
			return false, fmt.Errorf("bad: %#v", bStats)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}
