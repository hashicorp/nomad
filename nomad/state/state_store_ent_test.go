// +build ent

package state

import (
	"sort"
	"testing"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/stretchr/testify/assert"
)

func TestStateStore_UpsertSentinelPolicy(t *testing.T) {
	state := testStateStore(t)
	policy := mock.SentinelPolicy()
	policy2 := mock.SentinelPolicy()

	ws := memdb.NewWatchSet()
	if _, err := state.SentinelPolicyByName(ws, policy.Name); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := state.SentinelPolicyByName(ws, policy2.Name); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := state.UpsertSentinelPolicies(1000,
		[]*structs.SentinelPolicy{policy, policy2}); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	ws = memdb.NewWatchSet()
	out, err := state.SentinelPolicyByName(ws, policy.Name)
	assert.Equal(t, nil, err)
	assert.Equal(t, policy, out)

	out, err = state.SentinelPolicyByName(ws, policy2.Name)
	assert.Equal(t, nil, err)
	assert.Equal(t, policy2, out)

	iter, err := state.SentinelPolicies(ws)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we see both policies
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	if count != 2 {
		t.Fatalf("bad: %d", count)
	}

	iter, err = state.SentinelPoliciesByScope(ws, "submit-job")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we see both policies
	count = 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	if count != 2 {
		t.Fatalf("bad: %d", count)
	}

	index, err := state.Index("sentinel_policy")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1000 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_DeleteSentinelPolicy(t *testing.T) {
	state := testStateStore(t)
	policy := mock.SentinelPolicy()
	policy2 := mock.SentinelPolicy()

	// Create the policy
	if err := state.UpsertSentinelPolicies(1000,
		[]*structs.SentinelPolicy{policy, policy2}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a watcher
	ws := memdb.NewWatchSet()
	if _, err := state.SentinelPolicyByName(ws, policy.Name); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Delete the policy
	if err := state.DeleteSentinelPolicies(1001,
		[]string{policy.Name, policy2.Name}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure watching triggered
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Ensure we don't get the object back
	ws = memdb.NewWatchSet()
	out, err := state.SentinelPolicyByName(ws, policy.Name)
	assert.Equal(t, nil, err)
	if out != nil {
		t.Fatalf("bad: %#v", out)
	}

	iter, err := state.SentinelPolicies(ws)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we see both policies
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	if count != 0 {
		t.Fatalf("bad: %d", count)
	}

	index, err := state.Index("sentinel_policy")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if index != 1001 {
		t.Fatalf("bad: %d", index)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_SentinelPolicyByNamePrefix(t *testing.T) {
	state := testStateStore(t)
	names := []string{
		"foo",
		"bar",
		"foobar",
		"foozip",
		"zip",
	}

	// Create the policies
	var baseIndex uint64 = 1000
	for _, name := range names {
		p := mock.SentinelPolicy()
		p.Name = name
		if err := state.UpsertSentinelPolicies(baseIndex, []*structs.SentinelPolicy{p}); err != nil {
			t.Fatalf("err: %v", err)
		}
		baseIndex++
	}

	// Scan by prefix
	iter, err := state.SentinelPolicyByNamePrefix(nil, "foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we see both policies
	count := 0
	out := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
		out = append(out, raw.(*structs.SentinelPolicy).Name)
	}
	if count != 3 {
		t.Fatalf("bad: %d %v", count, out)
	}
	sort.Strings(out)

	expect := []string{"foo", "foobar", "foozip"}
	assert.Equal(t, expect, out)
}

func TestStateStore_RestoreSentinelPolicy(t *testing.T) {
	state := testStateStore(t)
	policy := mock.SentinelPolicy()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.SentinelPolicyRestore(policy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.SentinelPolicyByName(ws, policy.Name)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, policy, out)
}

func TestStateStore_NamespaceByQuota(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)

	qs := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(999, []*structs.QuotaSpec{qs}))

	ns1 := mock.Namespace()
	ns2 := mock.Namespace()
	ns2.Quota = qs.Name
	assert.Nil(state.UpsertNamespaces(1000, []*structs.Namespace{ns1, ns2}))

	// Create a watchset so we can test that getters don't cause it to fire
	ws := memdb.NewWatchSet()
	iter, err := state.NamespacesByQuota(ws, ns2.Quota)
	assert.Nil(err)

	gatherNamespaces := func(iter memdb.ResultIterator) []*structs.Namespace {
		var namespaces []*structs.Namespace
		for {
			raw := iter.Next()
			if raw == nil {
				break
			}
			ns := raw.(*structs.Namespace)
			namespaces = append(namespaces, ns)
		}
		return namespaces
	}

	namespaces := gatherNamespaces(iter)
	assert.Len(namespaces, 1)
	assert.Equal(ns2.Name, namespaces[0].Name)
	assert.False(watchFired(ws))

	iter, err = state.NamespacesByQuota(ws, "bar")
	assert.Nil(err)

	namespaces = gatherNamespaces(iter)
	assert.Empty(namespaces)
}

func TestStateStore_UpsertAllocs_Quota_NewAlloc(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)

	// 1. Create a QuotaSpec
	qs := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs}))

	// 2. Create a namespace with a quota
	ns1 := mock.Namespace()
	ns1.Quota = qs.Name
	assert.Nil(state.UpsertNamespaces(1001, []*structs.Namespace{ns1}))

	// 3. Create some allocations in the namespace
	a1 := mock.Alloc()
	a2 := mock.Alloc()
	a1.Namespace = ns1.Name
	a2.Namespace = ns1.Name
	allocs := []*structs.Allocation{a1, a2}
	assert.Nil(state.UpsertAllocs(1002, allocs))

	// 4. Assert that the QuotaUsage is updated.
	usage, err := state.QuotaUsageByName(nil, qs.Name)
	assert.Nil(err)
	assert.NotNil(usage)
	assert.EqualValues(1000, usage.CreateIndex)
	assert.EqualValues(1002, usage.ModifyIndex)
	assert.Len(usage.Used, 1)

	// Grab the usage
	used := usage.Used[string(qs.Limits[0].Hash)]
	assert.NotNil(used)
	assert.Equal("global", used.Region)

	expected := &structs.Resources{}
	r := mock.Alloc().Resources
	expected.Add(r)
	expected.Add(r)
	expected.Networks = nil
	expected.DiskMB = 0
	expected.IOPS = 0
	assert.Equal(expected, used.RegionLimit)
}

// This should no-op
func TestStateStore_UpsertAllocs_Quota_UpdateAlloc(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)

	// 1. Create a QuotaSpec
	qs := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs}))

	// 2. Create a namespace with a quota
	ns1 := mock.Namespace()
	ns1.Quota = qs.Name
	assert.Nil(state.UpsertNamespaces(1001, []*structs.Namespace{ns1}))

	// 3. Create some allocations in the namespace
	a1 := mock.Alloc()
	a2 := mock.Alloc()
	a1.Namespace = ns1.Name
	a2.Namespace = ns1.Name
	allocs := []*structs.Allocation{a1, a2}
	assert.Nil(state.UpsertAllocs(1002, allocs))

	// 4. Update the allocs
	j := mock.Alloc().Job
	j.Meta = map[string]string{"foo": "bar"}
	a3 := a1.Copy()
	a4 := a2.Copy()
	a3.Job = j
	a4.Job = j
	allocs = []*structs.Allocation{a3, a4}
	assert.Nil(state.UpsertAllocs(1003, allocs))

	// 5. Assert that the QuotaUsage is updated.
	usage, err := state.QuotaUsageByName(nil, qs.Name)
	assert.Nil(err)
	assert.NotNil(usage)
	assert.EqualValues(1000, usage.CreateIndex)
	assert.EqualValues(1002, usage.ModifyIndex)
	assert.Len(usage.Used, 1)

	// Grab the usage
	used := usage.Used[string(qs.Limits[0].Hash)]
	assert.NotNil(used)
	assert.Equal("global", used.Region)

	expected := &structs.Resources{}
	r := mock.Alloc().Resources
	expected.Add(r)
	expected.Add(r)
	expected.Networks = nil
	expected.DiskMB = 0
	expected.IOPS = 0
	assert.Equal(expected, used.RegionLimit)
}

func TestStateStore_UpsertAllocs_Quota_StopAlloc(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)

	// 1. Create a QuotaSpec
	qs := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs}))

	// 2. Create a namespace with a quota
	ns1 := mock.Namespace()
	ns1.Quota = qs.Name
	assert.Nil(state.UpsertNamespaces(1001, []*structs.Namespace{ns1}))

	// 3. Create some allocations in the namespace
	a1 := mock.Alloc()
	a2 := mock.Alloc()
	a1.Namespace = ns1.Name
	a2.Namespace = ns1.Name
	allocs := []*structs.Allocation{a1, a2}
	assert.Nil(state.UpsertAllocs(1002, allocs))

	// 4. Stop the allocs
	a3 := a1.Copy()
	a4 := a2.Copy()
	a3.DesiredStatus = structs.AllocDesiredStatusStop
	a4.DesiredStatus = structs.AllocDesiredStatusStop
	allocs = []*structs.Allocation{a3, a4}
	assert.Nil(state.UpsertAllocs(1003, allocs))

	// 5. Assert that the QuotaUsage is updated.
	usage, err := state.QuotaUsageByName(nil, qs.Name)
	assert.Nil(err)
	assert.NotNil(usage)
	assert.EqualValues(1000, usage.CreateIndex)
	assert.EqualValues(1003, usage.ModifyIndex)
	assert.Len(usage.Used, 1)

	// Grab the usage
	used := usage.Used[string(qs.Limits[0].Hash)]
	assert.NotNil(used)
	assert.Equal("global", used.Region)
	expected := &structs.Resources{}
	assert.Equal(expected, used.RegionLimit)
}

// This should no-op
func TestStateStore_UpdateAllocsFromClient_Quota_UpdateAlloc(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)

	// 1. Create a QuotaSpec
	qs := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs}))

	// 2. Create a namespace with a quota
	ns1 := mock.Namespace()
	ns1.Quota = qs.Name
	assert.Nil(state.UpsertNamespaces(1001, []*structs.Namespace{ns1}))

	// 3. Create some allocations in the namespace
	a1 := mock.Alloc()
	a2 := mock.Alloc()
	a1.Namespace = ns1.Name
	a2.Namespace = ns1.Name
	allocs := []*structs.Allocation{a1, a2}
	assert.Nil(state.UpsertAllocs(1002, allocs))

	// 4. Update the allocs
	a3 := a1.Copy()
	a4 := a2.Copy()
	a3.ClientStatus = structs.AllocClientStatusRunning
	a4.ClientStatus = structs.AllocClientStatusRunning
	allocs = []*structs.Allocation{a3, a4}
	assert.Nil(state.UpdateAllocsFromClient(1003, allocs))

	// 5. Assert that the QuotaUsage is updated.
	usage, err := state.QuotaUsageByName(nil, qs.Name)
	assert.Nil(err)
	assert.NotNil(usage)
	assert.EqualValues(1000, usage.CreateIndex)
	assert.EqualValues(1002, usage.ModifyIndex)
	assert.Len(usage.Used, 1)

	// Grab the usage
	used := usage.Used[string(qs.Limits[0].Hash)]
	assert.NotNil(used)
	assert.Equal("global", used.Region)

	expected := &structs.Resources{}
	r := mock.Alloc().Resources
	expected.Add(r)
	expected.Add(r)
	expected.Networks = nil
	expected.DiskMB = 0
	expected.IOPS = 0
	assert.Equal(expected, used.RegionLimit)
}

func TestStateStore_UpdateAllocsFromClient_Quota_StopAlloc(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)

	// 1. Create a QuotaSpec
	qs := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs}))

	// 2. Create a namespace with a quota
	ns1 := mock.Namespace()
	ns1.Quota = qs.Name
	assert.Nil(state.UpsertNamespaces(1001, []*structs.Namespace{ns1}))

	// 3. Create some allocations in the namespace
	a1 := mock.Alloc()
	a2 := mock.Alloc()
	a1.Namespace = ns1.Name
	a2.Namespace = ns1.Name
	allocs := []*structs.Allocation{a1, a2}
	assert.Nil(state.UpsertAllocs(1002, allocs))

	// 4. Stop the allocs
	a3 := a1.Copy()
	a4 := a2.Copy()
	a3.ClientStatus = structs.AllocClientStatusFailed
	a4.ClientStatus = structs.AllocClientStatusFailed
	allocs = []*structs.Allocation{a3, a4}
	assert.Nil(state.UpdateAllocsFromClient(1003, allocs))

	// 5. Assert that the QuotaUsage is updated.
	usage, err := state.QuotaUsageByName(nil, qs.Name)
	assert.Nil(err)
	assert.NotNil(usage)
	assert.EqualValues(1000, usage.CreateIndex)
	assert.EqualValues(1003, usage.ModifyIndex)
	assert.Len(usage.Used, 1)

	// Grab the usage
	used := usage.Used[string(qs.Limits[0].Hash)]
	assert.NotNil(used)
	assert.Equal("global", used.Region)
	expected := &structs.Resources{}
	assert.Equal(expected, used.RegionLimit)
}

func TestStateStore_UpsertNamespaces_BadQuota(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	ns1 := mock.Namespace()
	ns1.Quota = "foo"
	assert.NotNil(state.UpsertNamespaces(1000, []*structs.Namespace{ns1}))
}

func TestStateStore_UpsertNamespaces_NewQuota(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)

	// 1. Create a namespace
	ns1 := mock.Namespace()
	assert.Nil(state.UpsertNamespaces(1000, []*structs.Namespace{ns1}))

	// expected is the expected quota usage
	expected := &structs.Resources{}

	// 2. Create some allocations in the namespace
	var allocs []*structs.Allocation

	// Create a pending alloc
	a1 := mock.Alloc()
	a1.DesiredStatus = structs.AllocDesiredStatusRun
	a1.ClientStatus = structs.AllocClientStatusPending
	a1.Namespace = ns1.Name
	expected.Add(a1.Resources)

	// Create a running alloc
	a2 := mock.Alloc()
	a2.DesiredStatus = structs.AllocDesiredStatusRun
	a2.ClientStatus = structs.AllocClientStatusRunning
	a2.Namespace = ns1.Name
	expected.Add(a2.Resources)

	// Create a run/complete alloc
	a3 := mock.Alloc()
	a3.DesiredStatus = structs.AllocDesiredStatusRun
	a3.ClientStatus = structs.AllocClientStatusComplete
	a3.Namespace = ns1.Name
	allocs = append(allocs, a1, a2, a3)
	assert.Nil(state.UpsertAllocs(1001, allocs))

	// 3. Create a QuotaSpec and attach it to the namespace
	qs := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(1002, []*structs.QuotaSpec{qs}))
	ns2 := mock.Namespace()
	ns2.Name = ns1.Name
	ns2.Quota = qs.Name
	ns2.SetHash()
	assert.Nil(state.UpsertNamespaces(1003, []*structs.Namespace{ns2}))

	// 4. Assert that the QuotaUsage is updated.
	usage, err := state.QuotaUsageByName(nil, qs.Name)
	assert.Nil(err)
	assert.NotNil(usage)
	assert.EqualValues(1002, usage.CreateIndex)
	assert.EqualValues(1003, usage.ModifyIndex)
	assert.Len(usage.Used, 1)

	// Grab the usage
	used := usage.Used[string(qs.Limits[0].Hash)]
	assert.NotNil(used)
	assert.Equal("global", used.Region)
	expected.Networks = nil
	expected.DiskMB = 0
	expected.IOPS = 0
	assert.Equal(expected, used.RegionLimit)

}

func TestStateStore_UpsertNamespaces_RemoveQuota(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)

	// 1. Create a QuotaSpec
	qs := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs}))

	// 2. Create a namespace
	ns1 := mock.Namespace()
	assert.Nil(state.UpsertNamespaces(1001, []*structs.Namespace{ns1}))

	// 3. Create a allocation in the namespace
	a1 := mock.Alloc()
	a1.DesiredStatus = structs.AllocDesiredStatusRun
	a1.ClientStatus = structs.AllocClientStatusPending
	a1.Namespace = ns1.Name
	assert.Nil(state.UpsertAllocs(1002, []*structs.Allocation{a1}))

	// 4. Create a QuotaSpec and attach it to the namespace
	ns2 := mock.Namespace()
	ns2.Name = ns1.Name
	ns2.Quota = qs.Name
	ns2.SetHash()
	assert.Nil(state.UpsertNamespaces(1003, []*structs.Namespace{ns2}))

	// 5. Remove the spec from the namespace
	ns3 := mock.Namespace()
	ns3.Name = ns1.Name
	ns3.SetHash()
	assert.Nil(state.UpsertNamespaces(1004, []*structs.Namespace{ns3}))

	// 6. Assert that the QuotaUsage is empty.
	usage, err := state.QuotaUsageByName(nil, qs.Name)
	assert.Nil(err)
	assert.NotNil(usage)
	assert.EqualValues(1000, usage.CreateIndex)
	assert.EqualValues(1004, usage.ModifyIndex)
	assert.Len(usage.Used, 1)

	// Grab the usage
	used := usage.Used[string(qs.Limits[0].Hash)]
	assert.NotNil(used)
	assert.Equal("global", used.Region)
	assert.Equal(&structs.Resources{}, used.RegionLimit)
}

func TestStateStore_UpsertNamespaces_ChangeQuota(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)

	// 1. Create two QuotaSpecs
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()
	assert.Nil(state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs1, qs2}))

	// 2. Create a namespace
	ns1 := mock.Namespace()
	assert.Nil(state.UpsertNamespaces(1001, []*structs.Namespace{ns1}))

	// 3. Create a allocation in the namespace
	a1 := mock.Alloc()
	a1.DesiredStatus = structs.AllocDesiredStatusRun
	a1.ClientStatus = structs.AllocClientStatusPending
	a1.Namespace = ns1.Name
	assert.Nil(state.UpsertAllocs(1002, []*structs.Allocation{a1}))

	// 4. Create a QuotaSpec and attach it to the namespace
	ns2 := mock.Namespace()
	ns2.Name = ns1.Name
	ns2.Quota = qs1.Name
	ns2.SetHash()
	assert.Nil(state.UpsertNamespaces(1003, []*structs.Namespace{ns2}))

	// 5. Change the spec on the namespace
	ns3 := mock.Namespace()
	ns3.Name = ns1.Name
	ns3.Quota = qs2.Name
	ns3.SetHash()
	assert.Nil(state.UpsertNamespaces(1004, []*structs.Namespace{ns3}))

	// 6. Assert that the QuotaUsage for original spec is empty.
	usage, err := state.QuotaUsageByName(nil, qs1.Name)
	assert.Nil(err)
	assert.NotNil(usage)
	assert.EqualValues(1000, usage.CreateIndex)
	assert.EqualValues(1004, usage.ModifyIndex)
	assert.Len(usage.Used, 1)

	// Grab the usage
	used := usage.Used[string(qs1.Limits[0].Hash)]
	assert.NotNil(used)
	assert.Equal("global", used.Region)
	assert.Equal(&structs.Resources{}, used.RegionLimit)

	// 7. Assert that the QuotaUsage for new spec is populated.
	usage, err = state.QuotaUsageByName(nil, qs2.Name)
	assert.Nil(err)
	assert.NotNil(usage)
	assert.EqualValues(1000, usage.CreateIndex)
	assert.EqualValues(1004, usage.ModifyIndex)
	assert.Len(usage.Used, 1)

	// Grab the usage
	used = usage.Used[string(qs2.Limits[0].Hash)]
	assert.NotNil(used)
	assert.Equal("global", used.Region)
	a1.Resources.Networks = nil
	a1.Resources.DiskMB = 0
	a1.Resources.IOPS = 0
	assert.Equal(a1.Resources, used.RegionLimit)
}

func TestStateStore_UpsertQuotaSpec(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()

	ws := memdb.NewWatchSet()
	out, err := state.QuotaSpecByName(ws, qs1.Name)
	assert.Nil(out)
	assert.Nil(err)
	out, err = state.QuotaSpecByName(ws, qs2.Name)
	assert.Nil(out)
	assert.Nil(err)

	assert.Nil(state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs1, qs2}))
	assert.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err = state.QuotaSpecByName(ws, qs1.Name)
	assert.Nil(err)
	assert.Equal(qs1, out)

	out, err = state.QuotaSpecByName(ws, qs2.Name)
	assert.Nil(err)
	assert.Equal(qs2, out)

	// Assert there are corresponding usage objects
	usage, err := state.QuotaUsageByName(ws, qs1.Name)
	assert.Nil(err)
	assert.NotNil(usage)

	usage, err = state.QuotaUsageByName(ws, qs2.Name)
	assert.Nil(err)
	assert.NotNil(usage)

	iter, err := state.QuotaSpecs(ws)
	assert.Nil(err)

	// Ensure we see both specs
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	assert.Equal(2, count)

	index, err := state.Index(TableQuotaSpec)
	assert.Nil(err)
	assert.EqualValues(1000, index)
	assert.False(watchFired(ws))
}

func TestStateStore_UpsertQuotaSpec_Usage(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)

	// Create a quota specification with no limits
	qs := mock.QuotaSpec()
	limits := qs.Limits
	qs.Limits = nil
	assert.Nil(state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs}))

	// Create two namespaces and have one attach the quota specification
	ns1 := mock.Namespace()
	ns1.Quota = qs.Name
	ns2 := mock.Namespace()
	namespaces := []*structs.Namespace{ns1, ns2}
	assert.Nil(state.UpsertNamespaces(1001, namespaces))

	// expected is the expected quota usage
	expected := &structs.Resources{}

	// Create allocations in various states for both namespaces
	var allocs []*structs.Allocation
	for _, ns := range namespaces {
		// Create a pending alloc
		a1 := mock.Alloc()
		a1.DesiredStatus = structs.AllocDesiredStatusRun
		a1.ClientStatus = structs.AllocClientStatusPending
		a1.Namespace = ns.Name
		if ns.Quota != "" {
			expected.Add(a1.Resources)
		}

		// Create a running alloc
		a2 := mock.Alloc()
		a2.DesiredStatus = structs.AllocDesiredStatusRun
		a2.ClientStatus = structs.AllocClientStatusRunning
		a2.Namespace = ns.Name
		if ns.Quota != "" {
			expected.Add(a2.Resources)
		}

		// Create a run/complete alloc
		a3 := mock.Alloc()
		a3.DesiredStatus = structs.AllocDesiredStatusRun
		a3.ClientStatus = structs.AllocClientStatusComplete
		a3.Namespace = ns.Name

		// Create a stop/complete alloc
		a4 := mock.Alloc()
		a4.DesiredStatus = structs.AllocDesiredStatusStop
		a4.ClientStatus = structs.AllocClientStatusComplete
		a4.Namespace = ns.Name

		// Create a run/failed alloc
		a5 := mock.Alloc()
		a5.DesiredStatus = structs.AllocDesiredStatusRun
		a5.ClientStatus = structs.AllocClientStatusFailed
		a5.Namespace = ns.Name

		// Create a stop/failed alloc
		a6 := mock.Alloc()
		a6.DesiredStatus = structs.AllocDesiredStatusStop
		a6.ClientStatus = structs.AllocClientStatusFailed
		a6.Namespace = ns.Name

		// Create a lost alloc
		a7 := mock.Alloc()
		a7.DesiredStatus = structs.AllocDesiredStatusStop
		a7.ClientStatus = structs.AllocClientStatusLost
		a7.Namespace = ns.Name

		allocs = append(allocs, a1, a2, a3, a4, a5, a6, a7)
	}
	assert.Nil(state.UpsertAllocs(1002, allocs))

	// Add limits to the spec
	qs2 := mock.QuotaSpec()
	qs2.Name = qs.Name
	qs2.Limits = limits
	assert.Nil(state.UpsertQuotaSpecs(1003, []*structs.QuotaSpec{qs2}))

	// Assert the usage is built properly
	usage, err := state.QuotaUsageByName(nil, qs2.Name)
	assert.Nil(err)
	assert.NotNil(usage)
	assert.EqualValues(1000, usage.CreateIndex)
	assert.EqualValues(1003, usage.ModifyIndex)
	assert.Len(usage.Used, 1)

	// Grab the usage
	used := usage.Used[string(limits[0].Hash)]
	assert.NotNil(used)
	assert.Equal("global", used.Region)
	expected.Networks = nil
	expected.DiskMB = 0
	expected.IOPS = 0
	assert.Equal(expected, used.RegionLimit)
}

func TestStateStore_DeleteQuotaSpecs(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()

	// Create the quota specs
	assert.Nil(state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs1, qs2}))

	// Create a watcher
	ws := memdb.NewWatchSet()
	_, err := state.QuotaSpecByName(ws, qs1.Name)
	assert.Nil(err)

	// Delete the spec
	assert.Nil(state.DeleteQuotaSpecs(1001, []string{qs1.Name, qs2.Name}))

	// Ensure watching triggered
	assert.True(watchFired(ws))

	// Ensure we don't get the object back or a usage
	ws = memdb.NewWatchSet()
	out, err := state.QuotaSpecByName(ws, qs1.Name)
	assert.Nil(err)
	assert.Nil(out)

	usage, err := state.QuotaUsageByName(ws, qs1.Name)
	assert.Nil(err)
	assert.Nil(usage)

	iter, err := state.QuotaSpecs(ws)
	assert.Nil(err)

	// Ensure we see both policies
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	assert.Zero(count)

	index, err := state.Index(TableQuotaSpec)
	assert.Nil(err)
	assert.EqualValues(1001, index)
	assert.False(watchFired(ws))
}

func TestStateStore_DeleteQuotaSpecs_Referenced(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	qs1 := mock.QuotaSpec()

	// Create the quota specs
	assert.Nil(state.UpsertQuotaSpecs(1000, []*structs.QuotaSpec{qs1}))

	// Create two namespaces that reference the spec
	ns1, ns2 := mock.Namespace(), mock.Namespace()
	ns1.Quota = qs1.Name
	ns2.Quota = qs1.Name
	assert.Nil(state.UpsertNamespaces(1001, []*structs.Namespace{ns1, ns2}))

	// Delete the spec
	err := state.DeleteQuotaSpecs(1002, []string{qs1.Name})
	assert.NotNil(err)
	assert.Contains(err.Error(), ns1.Name)
	assert.Contains(err.Error(), ns2.Name)
}

func TestStateStore_QuotaSpecsByNamePrefix(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	names := []string{
		"foo",
		"bar",
		"foobar",
		"foozip",
		"zip",
	}

	// Create the policies
	var baseIndex uint64 = 1000
	for _, name := range names {
		qs := mock.QuotaSpec()
		qs.Name = name
		assert.Nil(state.UpsertQuotaSpecs(baseIndex, []*structs.QuotaSpec{qs}))
		baseIndex++
	}

	// Scan by prefix
	iter, err := state.QuotaSpecByNamePrefix(nil, "foo")
	assert.Nil(err)

	// Ensure we see both policies
	count := 0
	out := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
		out = append(out, raw.(*structs.QuotaSpec).Name)
	}
	assert.Equal(3, count)
	sort.Strings(out)

	expect := []string{"foo", "foobar", "foozip"}
	assert.Equal(expect, out)
}

func TestStateStore_RestoreQuotaSpec(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	spec := mock.QuotaSpec()

	restore, err := state.Restore()
	assert.Nil(err)

	err = restore.QuotaSpecRestore(spec)
	assert.Nil(err)
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.QuotaSpecByName(ws, spec.Name)
	assert.Nil(err)
	assert.Equal(spec, out)
}

func TestStateStore_UpsertQuotaUsage(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()
	qu1 := mock.QuotaUsage()
	qu2 := mock.QuotaUsage()
	qu1.Name = qs1.Name
	qu2.Name = qs2.Name

	ws := memdb.NewWatchSet()
	out, err := state.QuotaUsageByName(ws, qu1.Name)
	assert.Nil(out)
	assert.Nil(err)
	out, err = state.QuotaUsageByName(ws, qu2.Name)
	assert.Nil(out)
	assert.Nil(err)

	assert.Nil(state.UpsertQuotaSpecs(999, []*structs.QuotaSpec{qs1, qs2}))
	assert.Nil(state.UpsertQuotaUsages(1000, []*structs.QuotaUsage{qu1, qu2}))
	assert.True(watchFired(ws))

	ws = memdb.NewWatchSet()
	out, err = state.QuotaUsageByName(ws, qu1.Name)
	assert.Nil(err)
	assert.Equal(qu1, out)

	out, err = state.QuotaUsageByName(ws, qu2.Name)
	assert.Nil(err)
	assert.Equal(qu2, out)

	iter, err := state.QuotaUsages(ws)
	assert.Nil(err)

	// Ensure we see both usages
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	assert.Equal(2, count)

	index, err := state.Index(TableQuotaUsage)
	assert.Nil(err)
	assert.EqualValues(1000, index)
	assert.False(watchFired(ws))
}

func TestStateStore_DeleteQuotaUsages(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	qs1 := mock.QuotaSpec()
	qs2 := mock.QuotaSpec()
	qu1 := mock.QuotaUsage()
	qu2 := mock.QuotaUsage()
	qu1.Name = qs1.Name
	qu2.Name = qs2.Name

	// Create the quota usages
	assert.Nil(state.UpsertQuotaSpecs(999, []*structs.QuotaSpec{qs1, qs2}))
	assert.Nil(state.UpsertQuotaUsages(1000, []*structs.QuotaUsage{qu1, qu2}))

	// Create a watcher
	ws := memdb.NewWatchSet()
	_, err := state.QuotaUsageByName(ws, qu1.Name)
	assert.Nil(err)

	// Delete the usage
	assert.Nil(state.DeleteQuotaUsages(1001, []string{qu1.Name, qu2.Name}))

	// Ensure watching triggered
	assert.True(watchFired(ws))

	// Ensure we don't get the object back
	ws = memdb.NewWatchSet()
	out, err := state.QuotaUsageByName(ws, qu1.Name)
	assert.Nil(err)
	assert.Nil(out)

	iter, err := state.QuotaUsages(ws)
	assert.Nil(err)

	// Ensure we see both policies
	count := 0
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
	}
	assert.Zero(count)

	index, err := state.Index(TableQuotaUsage)
	assert.Nil(err)
	assert.EqualValues(1001, index)
	assert.False(watchFired(ws))
}

func TestStateStore_QuotaUsagesByNamePrefix(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	names := []string{
		"foo",
		"bar",
		"foobar",
		"foozip",
		"zip",
	}

	// Create the policies
	var baseIndex uint64 = 1000
	for _, name := range names {
		qs := mock.QuotaSpec()
		qs.Name = name
		qu := mock.QuotaUsage()
		qu.Name = name
		assert.Nil(state.UpsertQuotaSpecs(baseIndex, []*structs.QuotaSpec{qs}))
		assert.Nil(state.UpsertQuotaUsages(baseIndex+1, []*structs.QuotaUsage{qu}))
		baseIndex += 2
	}

	// Scan by prefix
	iter, err := state.QuotaUsageByNamePrefix(nil, "foo")
	assert.Nil(err)

	// Ensure we see both policies
	count := 0
	out := []string{}
	for {
		raw := iter.Next()
		if raw == nil {
			break
		}
		count++
		out = append(out, raw.(*structs.QuotaUsage).Name)
	}
	assert.Equal(3, count)
	sort.Strings(out)

	expect := []string{"foo", "foobar", "foozip"}
	assert.Equal(expect, out)
}

func TestStateStore_RestoreQuotaUsage(t *testing.T) {
	assert := assert.New(t)
	state := testStateStore(t)
	usage := mock.QuotaUsage()

	restore, err := state.Restore()
	assert.Nil(err)

	err = restore.QuotaUsageRestore(usage)
	assert.Nil(err)
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.QuotaUsageByName(ws, usage.Name)
	assert.Nil(err)
	assert.Equal(usage, out)
}
