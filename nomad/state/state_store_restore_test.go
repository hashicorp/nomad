// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateStore_RestoreNode(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	node := mock.Node()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.NodeRestore(node)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	require.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, node) {
		t.Fatalf("Bad: %#v %#v", out, node)
	}
}

func TestStateStore_RestoreJob(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	mockJob1 := mock.Job()

	restore, err := state.Restore()
	must.NoError(t, err)

	err = restore.JobRestore(mockJob1)
	must.NoError(t, err)
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.JobByID(ws, mockJob1.Namespace, mockJob1.ID)
	must.NoError(t, err)
	must.Eq(t, mockJob1, out)

	// Test upgrade to 1.6 or greater to simulate restoring a job which does
	// not have a node pool set.
	mockJob2 := mock.Job()
	mockJob2.NodePool = ""

	restore, err = state.Restore()
	must.NoError(t, err)

	err = restore.JobRestore(mockJob2)
	must.NoError(t, err)
	must.NoError(t, restore.Commit())

	ws = memdb.NewWatchSet()
	out, err = state.JobByID(ws, mockJob2.Namespace, mockJob2.ID)
	must.NoError(t, err)
	must.Eq(t, structs.NodePoolDefault, out.NodePool)
}

func TestStateStore_RestorePeriodicLaunch(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.Job()
	launch := &structs.PeriodicLaunch{
		ID:        job.ID,
		Namespace: job.Namespace,
		Launch:    time.Now(),
	}

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.PeriodicLaunchRestore(launch)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	require.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.PeriodicLaunchByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, launch) {
		t.Fatalf("Bad: %#v %#v", out, job)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_RestoreJobVersion(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.Job()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.JobVersionRestore(job)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	require.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.JobByIDAndVersion(ws, job.Namespace, job.ID, job.Version)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, job) {
		t.Fatalf("Bad: %#v %#v", out, job)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_RestoreDeployment(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	d := mock.Deployment()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.DeploymentRestore(d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	require.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.DeploymentByID(ws, d.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, d) {
		t.Fatalf("Bad: %#v %#v", out, d)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_RestoreJobSummary(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.Job()
	jobSummary := &structs.JobSummary{
		JobID:     job.ID,
		Namespace: job.Namespace,
		Summary: map[string]structs.TaskGroupSummary{
			"web": {
				Starting: 10,
			},
		},
	}
	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.JobSummaryRestore(jobSummary)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	require.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, jobSummary) {
		t.Fatalf("Bad: %#v %#v", out, jobSummary)
	}
}

func TestStateStore_RestoreCSIPlugin(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	state := testStateStore(t)
	plugin := mock.CSIPlugin()

	restore, err := state.Restore()
	require.NoError(err)

	err = restore.CSIPluginRestore(plugin)
	require.NoError(err)
	require.NoError(restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.CSIPluginByID(ws, plugin.ID)
	require.NoError(err)
	require.EqualValues(out, plugin)
}

func TestStateStore_RestoreCSIVolume(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	state := testStateStore(t)
	plugin := mock.CSIPlugin()
	volume := mock.CSIVolume(plugin)

	restore, err := state.Restore()
	require.NoError(err)

	err = restore.CSIVolumeRestore(volume)
	require.NoError(err)
	restore.Commit()

	ws := memdb.NewWatchSet()
	out, err := state.CSIVolumeByID(ws, "default", volume.ID)
	require.NoError(err)
	require.EqualValues(out, volume)
}

func TestStateStore_RestoreIndex(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	index := &IndexEntry{"jobs", 1000}
	err = restore.IndexRestore(index)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	require.NoError(t, restore.Commit())

	out, err := state.Index("jobs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if out != 1000 {
		t.Fatalf("Bad: %#v %#v", out, 1000)
	}
}

func TestStateStore_RestoreEval(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	eval := mock.Eval()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.EvalRestore(eval)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	require.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, eval) {
		t.Fatalf("Bad: %#v %#v", out, eval)
	}
}

func TestStateStore_RestoreAlloc(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.AllocRestore(alloc)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	require.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, alloc) {
		t.Fatalf("Bad: %#v %#v", out, alloc)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_RestoreVaultAccessor(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	a := mock.VaultAccessor()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.VaultAccessorRestore(a)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	require.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.VaultAccessor(ws, a.Accessor)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(out, a) {
		t.Fatalf("Bad: %#v %#v", out, a)
	}

	if watchFired(ws) {
		t.Fatalf("bad")
	}
}

func TestStateStore_RestoreSITokenAccessor(t *testing.T) {
	ci.Parallel(t)
	r := require.New(t)

	state := testStateStore(t)
	a1 := mock.SITokenAccessor()

	restore, err := state.Restore()
	r.NoError(err)

	err = restore.SITokenAccessorRestore(a1)
	r.NoError(err)

	require.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	result, err := state.SITokenAccessor(ws, a1.AccessorID)
	r.NoError(err)
	r.Equal(a1, result)

	wsFired := watchFired(ws)
	r.False(wsFired)
}

func TestStateStore_RestoreACLPolicy(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	policy := mock.ACLPolicy()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.ACLPolicyRestore(policy)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	require.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.ACLPolicyByName(ws, policy.Name)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, policy, out)
}

func TestStateStore_RestoreACLToken(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	token := mock.ACLToken()

	restore, err := state.Restore()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = restore.ACLTokenRestore(token)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	require.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.ACLTokenByAccessorID(ws, token.AccessorID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, token, out)
}

func TestStateStore_ClusterMetadataRestore(t *testing.T) {
	require := require.New(t)

	state := testStateStore(t)
	clusterID := "12345678-1234-1234-1234-1234567890"
	now := time.Now().UnixNano()
	meta := &structs.ClusterMetadata{ClusterID: clusterID, CreateTime: now}

	restore, err := state.Restore()
	require.NoError(err)

	err = restore.ClusterMetadataRestore(meta)
	require.NoError(err)

	require.NoError(restore.Commit())

	out, err := state.ClusterMetadata(nil)
	require.NoError(err)
	require.Equal(clusterID, out.ClusterID)
	require.Equal(now, out.CreateTime)
}

func TestStateStore_RestoreScalingPolicy(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	state := testStateStore(t)
	scalingPolicy := mock.ScalingPolicy()

	restore, err := state.Restore()
	require.NoError(err)

	err = restore.ScalingPolicyRestore(scalingPolicy)
	require.NoError(err)
	require.NoError(restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.ScalingPolicyByID(ws, scalingPolicy.ID)
	require.NoError(err)
	require.EqualValues(out, scalingPolicy)
}

func TestStateStore_RestoreScalingEvents(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	state := testStateStore(t)
	jobScalingEvents := &structs.JobScalingEvents{
		Namespace: uuid.Generate(),
		JobID:     uuid.Generate(),
		ScalingEvents: map[string][]*structs.ScalingEvent{
			uuid.Generate(): {
				structs.NewScalingEvent(uuid.Generate()),
			},
		},
	}

	restore, err := state.Restore()
	require.NoError(err)

	err = restore.ScalingEventsRestore(jobScalingEvents)
	require.NoError(err)
	require.NoError(restore.Commit())

	ws := memdb.NewWatchSet()
	out, _, err := state.ScalingEventsByJob(ws, jobScalingEvents.Namespace,
		jobScalingEvents.JobID)
	require.NoError(err)
	require.NotNil(out)
	require.EqualValues(jobScalingEvents.ScalingEvents, out)
}

func TestStateStore_RestoreSchedulerConfig(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	schedConfig := &structs.SchedulerConfiguration{
		PreemptionConfig: structs.PreemptionConfig{
			SystemSchedulerEnabled: false,
		},
		CreateIndex: 100,
		ModifyIndex: 200,
	}

	require := require.New(t)
	restore, err := state.Restore()
	require.Nil(err)

	err = restore.SchedulerConfigRestore(schedConfig)
	require.Nil(err)

	require.NoError(restore.Commit())

	modIndex, out, err := state.SchedulerConfig()
	require.Nil(err)
	require.Equal(schedConfig.ModifyIndex, modIndex)

	require.Equal(schedConfig, out)
}

func TestStateStore_ServiceRegistrationRestore(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Set up our test registrations and index.
	expectedIndex := uint64(13)
	serviceRegs := mock.ServiceRegistrations()

	restore, err := testState.Restore()
	require.NoError(t, err)

	// Iterate the service registrations, restore, and commit. Set the indexes
	// on the objects, so we can check these.
	for i := range serviceRegs {
		serviceRegs[i].ModifyIndex = expectedIndex
		serviceRegs[i].CreateIndex = expectedIndex
		require.NoError(t, restore.ServiceRegistrationRestore(serviceRegs[i]))
	}
	require.NoError(t, restore.Commit())

	// Check the state is now populated as we expect and that we can find the
	// restored registrations.
	ws := memdb.NewWatchSet()

	for i := range serviceRegs {
		out, err := testState.GetServiceRegistrationByID(ws, serviceRegs[i].Namespace, serviceRegs[i].ID)
		require.NoError(t, err)
		require.Equal(t, serviceRegs[i], out)
	}
}

func TestStateStore_VariablesRestore(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Set up our test variables and index.
	expectedIndex := uint64(13)
	svs := mock.VariablesEncrypted(5, 5)

	restore, err := testState.Restore()
	require.NoError(t, err)

	// Iterate the variables, restore, and commit. Set the indexes
	// on the objects, so we can check these.
	for i := range svs {
		svs[i].ModifyIndex = expectedIndex
		svs[i].CreateIndex = expectedIndex
		require.NoError(t, restore.VariablesRestore(svs[i]))
	}
	require.NoError(t, restore.Commit())

	// Check the state is now populated as we expect and that we can find the
	// restored variables.
	ws := memdb.NewWatchSet()

	for i := range svs {
		out, err := testState.GetVariable(ws, svs[i].Namespace, svs[i].Path)
		require.NoError(t, err)
		require.Equal(t, svs[i], out)
	}
}

func TestStateStore_ACLRoleRestore(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Set up our test registrations and index.
	expectedIndex := uint64(13)
	aclRole := mock.ACLRole()
	aclRole.CreateIndex = expectedIndex
	aclRole.ModifyIndex = expectedIndex

	restore, err := testState.Restore()
	require.NoError(t, err)
	require.NoError(t, restore.ACLRoleRestore(aclRole))
	require.NoError(t, restore.Commit())

	// Check the state is now populated as we expect and that we can find the
	// restored registrations.
	ws := memdb.NewWatchSet()
	out, err := testState.GetACLRoleByName(ws, aclRole.Name)
	require.NoError(t, err)
	require.Equal(t, aclRole, out)
}

func TestStateStore_ACLAuthMethodRestore(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Set up our test registrations and index.
	expectedIndex := uint64(13)
	authMethod := mock.ACLOIDCAuthMethod()
	authMethod.CreateIndex = expectedIndex
	authMethod.ModifyIndex = expectedIndex

	restore, err := testState.Restore()
	require.NoError(t, err)
	require.NoError(t, restore.ACLAuthMethodRestore(authMethod))
	require.NoError(t, restore.Commit())

	// Check the state is now populated as we expect and that we can find the
	// restored registrations.
	ws := memdb.NewWatchSet()
	out, err := testState.GetACLAuthMethodByName(ws, authMethod.Name)
	require.NoError(t, err)
	require.Equal(t, authMethod, out)
}

func TestStateStore_ACLBindingRuleRestore(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Set up our test ACL binding rule and index.
	expectedIndex := uint64(13)
	aclBindingRule := mock.ACLBindingRule()
	aclBindingRule.CreateIndex = expectedIndex
	aclBindingRule.ModifyIndex = expectedIndex

	restore, err := testState.Restore()
	must.NoError(t, err)
	must.NoError(t, restore.ACLBindingRuleRestore(aclBindingRule))
	must.NoError(t, restore.Commit())

	// Check the state is now populated as we expect and that we can find the
	// restored ACL binding rule.
	ws := memdb.NewWatchSet()
	out, err := testState.GetACLBindingRule(ws, aclBindingRule.ID)
	must.NoError(t, err)
	must.Eq(t, aclBindingRule, out)
}
