// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestStateStore_RestoreNode(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	node := mock.Node()

	restore, err := state.Restore()
	must.NoError(t, err)

	must.NoError(t, restore.NodeRestore(node))
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.NodeByID(ws, node.ID)
	must.NoError(t, err)
	must.Eq(t, node, out)
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
	must.NoError(t, err)

	must.NoError(t, restore.PeriodicLaunchRestore(launch))
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.PeriodicLaunchByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Eq(t, launch, out)
	must.False(t, watchFired(ws))
}

func TestStateStore_RestoreJobVersion(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	job := mock.Job()

	restore, err := state.Restore()
	must.NoError(t, err)

	must.NoError(t, restore.JobVersionRestore(job))
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.JobByIDAndVersion(ws, job.Namespace, job.ID, job.Version)
	must.NoError(t, err)
	must.Eq(t, job, out)
	must.False(t, watchFired(ws))
}

func TestStateStore_RestoreDeployment(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	d := mock.Deployment()

	restore, err := state.Restore()
	must.NoError(t, err)

	must.NoError(t, restore.DeploymentRestore(d))
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.DeploymentByID(ws, d.ID)
	must.NoError(t, err)
	must.Eq(t, d, out)
	must.False(t, watchFired(ws))
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
	must.NoError(t, err)

	must.NoError(t, restore.JobSummaryRestore(jobSummary))
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.JobSummaryByID(ws, job.Namespace, job.ID)
	must.NoError(t, err)
	must.Eq(t, jobSummary, out)
	must.False(t, watchFired(ws))
}

func TestStateStore_RestoreCSIPlugin(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	plugin := mock.CSIPlugin()

	restore, err := state.Restore()
	must.NoError(t, err)

	err = restore.CSIPluginRestore(plugin)
	must.NoError(t, err)
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.CSIPluginByID(ws, plugin.ID)
	must.NoError(t, err)
	must.Eq(t, plugin, out)
	must.False(t, watchFired(ws))
}

func TestStateStore_RestoreCSIVolume(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	plugin := mock.CSIPlugin()
	volume := mock.CSIVolume(plugin)

	restore, err := state.Restore()
	must.NoError(t, err)

	err = restore.CSIVolumeRestore(volume)
	must.NoError(t, err)
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.CSIVolumeByID(ws, "default", volume.ID)
	must.NoError(t, err)
	must.Eq(t, volume, out)
}

func TestStateStore_RestoreIndex(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)

	restore, err := state.Restore()
	must.NoError(t, err)

	index := &IndexEntry{"jobs", 1000}

	must.NoError(t, restore.IndexRestore(index))
	must.NoError(t, restore.Commit())

	out, err := state.Index("jobs")
	must.NoError(t, err)
	must.Eq(t, 1000, out)
}

func TestStateStore_RestoreEval(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	eval := mock.Eval()

	restore, err := state.Restore()
	must.NoError(t, err)

	must.NoError(t, restore.EvalRestore(eval))
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.EvalByID(ws, eval.ID)
	must.NoError(t, err)
	must.Eq(t, eval, out)
}

func TestStateStore_RestoreAlloc(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	alloc := mock.Alloc()

	restore, err := state.Restore()
	must.NoError(t, err)

	must.NoError(t, restore.AllocRestore(alloc))
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.AllocByID(ws, alloc.ID)
	must.NoError(t, err)
	must.Eq(t, alloc, out)
	must.False(t, watchFired(ws))
}

func TestStateStore_RestoreVaultAccessor(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	a := mock.VaultAccessor()

	restore, err := state.Restore()
	must.NoError(t, err)

	err = restore.VaultAccessorRestore(a)
	must.NoError(t, err)
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.VaultAccessor(ws, a.Accessor)
	must.NoError(t, err)
	must.Eq(t, a, out)
	must.False(t, watchFired(ws))
}

func TestStateStore_RestoreSITokenAccessor(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	a1 := mock.SITokenAccessor()

	restore, err := state.Restore()
	must.NoError(t, err)

	must.NoError(t, restore.SITokenAccessorRestore(a1))
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	result, err := state.SITokenAccessor(ws, a1.AccessorID)
	must.NoError(t, err)
	must.Eq(t, a1, result)
	must.False(t, watchFired(ws))
}

func TestStateStore_RestoreACLPolicy(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	policy := mock.ACLPolicy()

	restore, err := state.Restore()
	must.NoError(t, err)

	err = restore.ACLPolicyRestore(policy)
	must.NoError(t, err)
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.ACLPolicyByName(ws, policy.Name)
	must.NoError(t, err)
	must.Eq(t, policy, out)
}

func TestStateStore_RestoreACLToken(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	token := mock.ACLToken()

	restore, err := state.Restore()
	must.NoError(t, err)

	err = restore.ACLTokenRestore(token)
	must.NoError(t, err)
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.ACLTokenByAccessorID(ws, token.AccessorID)
	must.NoError(t, err)
	must.Eq(t, token, out)
}

func TestStateStore_ClusterMetadataRestore(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	clusterID := "12345678-1234-1234-1234-1234567890"
	now := time.Now().UnixNano()
	meta := &structs.ClusterMetadata{ClusterID: clusterID, CreateTime: now}

	restore, err := state.Restore()
	must.NoError(t, err)

	err = restore.ClusterMetadataRestore(meta)
	must.NoError(t, err)
	must.NoError(t, restore.Commit())

	out, err := state.ClusterMetadata(nil)
	must.NoError(t, err)
	must.Eq(t, clusterID, out.ClusterID)
	must.Eq(t, now, out.CreateTime)
}

func TestStateStore_RestoreScalingPolicy(t *testing.T) {
	ci.Parallel(t)

	state := testStateStore(t)
	scalingPolicy := mock.ScalingPolicy()

	restore, err := state.Restore()
	must.NoError(t, err)

	err = restore.ScalingPolicyRestore(scalingPolicy)
	must.NoError(t, err)
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, err := state.ScalingPolicyByID(ws, scalingPolicy.ID)
	must.NoError(t, err)
	must.Eq(t, scalingPolicy, out)
}

func TestStateStore_RestoreScalingEvents(t *testing.T) {
	ci.Parallel(t)

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
	must.NoError(t, err)

	err = restore.ScalingEventsRestore(jobScalingEvents)
	must.NoError(t, err)
	must.NoError(t, restore.Commit())

	ws := memdb.NewWatchSet()
	out, _, err := state.ScalingEventsByJob(ws, jobScalingEvents.Namespace,
		jobScalingEvents.JobID)
	must.NoError(t, err)
	must.NotNil(t, out)
	must.Eq(t, jobScalingEvents.ScalingEvents, out)
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

	restore, err := state.Restore()
	must.NoError(t, err)

	err = restore.SchedulerConfigRestore(schedConfig)
	must.NoError(t, err)

	must.NoError(t, restore.Commit())

	modIndex, out, err := state.SchedulerConfig()
	must.NoError(t, err)
	must.Eq(t, schedConfig.ModifyIndex, modIndex)
	must.Eq(t, schedConfig, out)
}

func TestStateStore_ServiceRegistrationRestore(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Set up our test registrations and index.
	expectedIndex := uint64(13)
	serviceRegs := mock.ServiceRegistrations()

	restore, err := testState.Restore()
	must.NoError(t, err)

	// Iterate the service registrations, restore, and commit. Set the indexes
	// on the objects, so we can check these.
	for i := range serviceRegs {
		serviceRegs[i].ModifyIndex = expectedIndex
		serviceRegs[i].CreateIndex = expectedIndex
		must.NoError(t, restore.ServiceRegistrationRestore(serviceRegs[i]))
	}
	must.NoError(t, restore.Commit())

	// Check the state is now populated as we expect and that we can find the
	// restored registrations.
	ws := memdb.NewWatchSet()

	for i := range serviceRegs {
		out, err := testState.GetServiceRegistrationByID(ws, serviceRegs[i].Namespace, serviceRegs[i].ID)
		must.NoError(t, err)
		must.Eq(t, serviceRegs[i], out)
	}
}

func TestStateStore_VariablesRestore(t *testing.T) {
	ci.Parallel(t)
	testState := testStateStore(t)

	// Set up our test variables and index.
	expectedIndex := uint64(13)
	svs := mock.VariablesEncrypted(5, 5)

	restore, err := testState.Restore()
	must.NoError(t, err)

	// Iterate the variables, restore, and commit. Set the indexes
	// on the objects, so we can check these.
	for i := range svs {
		svs[i].ModifyIndex = expectedIndex
		svs[i].CreateIndex = expectedIndex
		must.NoError(t, restore.VariablesRestore(svs[i]))
	}
	must.NoError(t, restore.Commit())

	// Check the state is now populated as we expect and that we can find the
	// restored variables.
	ws := memdb.NewWatchSet()

	for i := range svs {
		out, err := testState.GetVariable(ws, svs[i].Namespace, svs[i].Path)
		must.NoError(t, err)
		must.Eq(t, svs[i], out)
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
	must.NoError(t, err)
	must.NoError(t, restore.ACLRoleRestore(aclRole))
	must.NoError(t, restore.Commit())

	// Check the state is now populated as we expect and that we can find the
	// restored registrations.
	ws := memdb.NewWatchSet()
	out, err := testState.GetACLRoleByName(ws, aclRole.Name)
	must.NoError(t, err)
	must.Eq(t, aclRole, out)
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
	must.NoError(t, err)
	must.NoError(t, restore.ACLAuthMethodRestore(authMethod))
	must.NoError(t, restore.Commit())

	// Check the state is now populated as we expect and that we can find the
	// restored registrations.
	ws := memdb.NewWatchSet()
	out, err := testState.GetACLAuthMethodByName(ws, authMethod.Name)
	must.NoError(t, err)
	must.Eq(t, authMethod, out)
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
