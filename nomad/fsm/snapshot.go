package fsm

import (
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/nomad/nomad/state"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/nomad/timetable"
	"github.com/hashicorp/raft"
)

// SnapshotType is prefixed to a record in the FSM snapshot
// so that we can determine the type for restore
type SnapshotType byte

const (
	NodeSnapshot                         SnapshotType = 0
	JobSnapshot                          SnapshotType = 1
	IndexSnapshot                        SnapshotType = 2
	EvalSnapshot                         SnapshotType = 3
	AllocSnapshot                        SnapshotType = 4
	TimeTableSnapshot                    SnapshotType = 5
	PeriodicLaunchSnapshot               SnapshotType = 6
	JobSummarySnapshot                   SnapshotType = 7
	VaultAccessorSnapshot                SnapshotType = 8
	JobVersionSnapshot                   SnapshotType = 9
	DeploymentSnapshot                   SnapshotType = 10
	ACLPolicySnapshot                    SnapshotType = 11
	ACLTokenSnapshot                     SnapshotType = 12
	SchedulerConfigSnapshot              SnapshotType = 13
	ClusterMetadataSnapshot              SnapshotType = 14
	ServiceIdentityTokenAccessorSnapshot SnapshotType = 15
	ScalingPolicySnapshot                SnapshotType = 16
	CSIPluginSnapshot                    SnapshotType = 17
	CSIVolumeSnapshot                    SnapshotType = 18
	ScalingEventsSnapshot                SnapshotType = 19
	EventSinkSnapshot                    SnapshotType = 20
	ServiceRegistrationSnapshot          SnapshotType = 21
	VariablesSnapshot                    SnapshotType = 22
	VariablesQuotaSnapshot               SnapshotType = 23
	RootKeyMetaSnapshot                  SnapshotType = 24
	ACLRoleSnapshot                      SnapshotType = 25

	// Namespace appliers were moved from enterprise and therefore start at 64
	NamespaceSnapshot SnapshotType = 64
)

// nomadSnapshot is used to provide a snapshot of the current
// state in a way that can be accessed concurrently with operations
// that may modify the live state.
type nomadSnapshot struct {
	snap      *state.StateSnapshot
	timetable *timetable.TimeTable
}

// snapshotHeader is the first entry in our snapshot
type snapshotHeader struct{}

func (s *nomadSnapshot) Persist(sink raft.SnapshotSink) error {
	defer metrics.MeasureSince([]string{"nomad", "fsm", "persist"}, time.Now())
	// Register the nodes
	encoder := codec.NewEncoder(sink, structs.MsgpackHandle)

	// Write the header
	header := snapshotHeader{}
	if err := encoder.Encode(&header); err != nil {
		sink.Cancel()
		return err
	}

	// Write the time table
	sink.Write([]byte{byte(TimeTableSnapshot)})
	if err := s.timetable.Serialize(encoder); err != nil {
		sink.Cancel()
		return err
	}

	// Write all the data out
	if err := s.persistIndexes(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistNodes(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistJobs(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistEvals(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistAllocs(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistPeriodicLaunches(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistJobSummaries(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistVaultAccessors(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistSITokenAccessors(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistJobVersions(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistDeployments(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistScalingPolicies(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistScalingEvents(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistCSIPlugins(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistCSIVolumes(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistACLPolicies(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistACLTokens(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistNamespaces(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistEnterpriseTables(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistSchedulerConfig(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistClusterMetadata(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistServiceRegistrations(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistVariables(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistVariablesQuotas(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistRootKeyMeta(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	if err := s.persistACLRoles(sink, encoder); err != nil {
		sink.Cancel()
		return err
	}
	return nil
}

func (s *nomadSnapshot) persistIndexes(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the indexes
	iter, err := s.snap.Indexes()
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := iter.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		idx := raw.(*state.IndexEntry)

		// Write out a node registration
		sink.Write([]byte{byte(IndexSnapshot)})
		if err := encoder.Encode(idx); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistNodes(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the nodes
	ws := memdb.NewWatchSet()
	nodes, err := s.snap.Nodes(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := nodes.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		node := raw.(*structs.Node)

		// Write out a node registration
		sink.Write([]byte{byte(NodeSnapshot)})
		if err := encoder.Encode(node); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistJobs(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the jobs
	ws := memdb.NewWatchSet()
	jobs, err := s.snap.Jobs(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := jobs.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		job := raw.(*structs.Job)

		// Write out a job registration
		sink.Write([]byte{byte(JobSnapshot)})
		if err := encoder.Encode(job); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistEvals(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the evaluations
	ws := memdb.NewWatchSet()
	evals, err := s.snap.Evals(ws, false)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := evals.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		eval := raw.(*structs.Evaluation)

		// Write out the evaluation
		sink.Write([]byte{byte(EvalSnapshot)})
		if err := encoder.Encode(eval); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistAllocs(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the allocations
	ws := memdb.NewWatchSet()
	allocs, err := s.snap.Allocs(ws, state.SortDefault)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := allocs.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		alloc := raw.(*structs.Allocation)

		// Write out the evaluation
		sink.Write([]byte{byte(AllocSnapshot)})
		if err := encoder.Encode(alloc); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistPeriodicLaunches(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the jobs
	ws := memdb.NewWatchSet()
	launches, err := s.snap.PeriodicLaunches(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := launches.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		launch := raw.(*structs.PeriodicLaunch)

		// Write out a job registration
		sink.Write([]byte{byte(PeriodicLaunchSnapshot)})
		if err := encoder.Encode(launch); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistJobSummaries(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	ws := memdb.NewWatchSet()
	summaries, err := s.snap.JobSummaries(ws)
	if err != nil {
		return err
	}

	for {
		raw := summaries.Next()
		if raw == nil {
			break
		}

		jobSummary := raw.(*structs.JobSummary)

		sink.Write([]byte{byte(JobSummarySnapshot)})
		if err := encoder.Encode(jobSummary); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistVaultAccessors(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	ws := memdb.NewWatchSet()
	accessors, err := s.snap.VaultAccessors(ws)
	if err != nil {
		return err
	}

	for {
		raw := accessors.Next()
		if raw == nil {
			break
		}

		accessor := raw.(*structs.VaultAccessor)

		sink.Write([]byte{byte(VaultAccessorSnapshot)})
		if err := encoder.Encode(accessor); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistSITokenAccessors(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	ws := memdb.NewWatchSet()
	accessors, err := s.snap.SITokenAccessors(ws)
	if err != nil {
		return err
	}

	for raw := accessors.Next(); raw != nil; raw = accessors.Next() {
		accessor := raw.(*structs.SITokenAccessor)
		sink.Write([]byte{byte(ServiceIdentityTokenAccessorSnapshot)})
		if err := encoder.Encode(accessor); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistJobVersions(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the jobs
	ws := memdb.NewWatchSet()
	versions, err := s.snap.JobVersions(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := versions.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		job := raw.(*structs.Job)

		// Write out a job registration
		sink.Write([]byte{byte(JobVersionSnapshot)})
		if err := encoder.Encode(job); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistDeployments(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the jobs
	ws := memdb.NewWatchSet()
	deployments, err := s.snap.Deployments(ws, state.SortDefault)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := deployments.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		deployment := raw.(*structs.Deployment)

		// Write out a job registration
		sink.Write([]byte{byte(DeploymentSnapshot)})
		if err := encoder.Encode(deployment); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistACLPolicies(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the policies
	ws := memdb.NewWatchSet()
	policies, err := s.snap.ACLPolicies(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := policies.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		policy := raw.(*structs.ACLPolicy)

		// Write out a policy registration
		sink.Write([]byte{byte(ACLPolicySnapshot)})
		if err := encoder.Encode(policy); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistACLTokens(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get all the policies
	ws := memdb.NewWatchSet()
	tokens, err := s.snap.ACLTokens(ws, state.SortDefault)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := tokens.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		token := raw.(*structs.ACLToken)

		// Write out a token registration
		sink.Write([]byte{byte(ACLTokenSnapshot)})
		if err := encoder.Encode(token); err != nil {
			return err
		}
	}
	return nil
}

// persistNamespaces persists all the namespaces.
func (s *nomadSnapshot) persistNamespaces(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	// Get all the jobs
	ws := memdb.NewWatchSet()
	namespaces, err := s.snap.Namespaces(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := namespaces.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		namespace := raw.(*structs.Namespace)

		// Write out a namespace registration
		sink.Write([]byte{byte(NamespaceSnapshot)})
		if err := encoder.Encode(namespace); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistSchedulerConfig(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {
	// Get scheduler config
	_, schedConfig, err := s.snap.SchedulerConfig()
	if err != nil {
		return err
	}
	if schedConfig == nil {
		return nil
	}
	// Write out scheduler config
	sink.Write([]byte{byte(SchedulerConfigSnapshot)})
	if err := encoder.Encode(schedConfig); err != nil {
		return err
	}
	return nil
}

func (s *nomadSnapshot) persistClusterMetadata(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	// Get the cluster metadata
	ws := memdb.NewWatchSet()
	clusterMetadata, err := s.snap.ClusterMetadata(ws)
	if err != nil {
		return err
	}
	if clusterMetadata == nil {
		return nil
	}

	// Write out the cluster metadata
	sink.Write([]byte{byte(ClusterMetadataSnapshot)})
	if err := encoder.Encode(clusterMetadata); err != nil {
		return err
	}

	return nil
}

func (s *nomadSnapshot) persistScalingPolicies(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	// Get all the scaling policies
	ws := memdb.NewWatchSet()
	scalingPolicies, err := s.snap.ScalingPolicies(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := scalingPolicies.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		scalingPolicy := raw.(*structs.ScalingPolicy)

		// Write out a scaling policy snapshot
		sink.Write([]byte{byte(ScalingPolicySnapshot)})
		if err := encoder.Encode(scalingPolicy); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistScalingEvents(sink raft.SnapshotSink, encoder *codec.Encoder) error {
	// Get all the scaling events
	ws := memdb.NewWatchSet()
	iter, err := s.snap.ScalingEvents(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := iter.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		events := raw.(*structs.JobScalingEvents)

		// Write out a scaling events snapshot
		sink.Write([]byte{byte(ScalingEventsSnapshot)})
		if err := encoder.Encode(events); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistCSIPlugins(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	// Get all the CSI plugins
	ws := memdb.NewWatchSet()
	plugins, err := s.snap.CSIPlugins(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := plugins.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		plugin := raw.(*structs.CSIPlugin)

		// Write out a plugin snapshot
		sink.Write([]byte{byte(CSIPluginSnapshot)})
		if err := encoder.Encode(plugin); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistCSIVolumes(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	// Get all the CSI volumes
	ws := memdb.NewWatchSet()
	volumes, err := s.snap.CSIVolumes(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item
		raw := volumes.Next()
		if raw == nil {
			break
		}

		// Prepare the request struct
		volume := raw.(*structs.CSIVolume)

		// Write out a volume snapshot
		sink.Write([]byte{byte(CSIVolumeSnapshot)})
		if err := encoder.Encode(volume); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistServiceRegistrations(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	// Get all the service registrations.
	ws := memdb.NewWatchSet()
	serviceRegs, err := s.snap.GetServiceRegistrations(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item.
		for raw := serviceRegs.Next(); raw != nil; raw = serviceRegs.Next() {

			// Prepare the request struct.
			reg := raw.(*structs.ServiceRegistration)

			// Write out a service registration snapshot.
			sink.Write([]byte{byte(ServiceRegistrationSnapshot)})
			if err := encoder.Encode(reg); err != nil {
				return err
			}
		}
		return nil
	}
}

func (s *nomadSnapshot) persistVariables(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	ws := memdb.NewWatchSet()
	variables, err := s.snap.Variables(ws)
	if err != nil {
		return err
	}

	for {
		raw := variables.Next()
		if raw == nil {
			break
		}
		variable := raw.(*structs.VariableEncrypted)
		sink.Write([]byte{byte(VariablesSnapshot)})
		if err := encoder.Encode(variable); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistVariablesQuotas(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	ws := memdb.NewWatchSet()
	quotas, err := s.snap.VariablesQuotas(ws)
	if err != nil {
		return err
	}

	for {
		raw := quotas.Next()
		if raw == nil {
			break
		}
		dirEntry := raw.(*structs.VariablesQuota)
		sink.Write([]byte{byte(VariablesQuotaSnapshot)})
		if err := encoder.Encode(dirEntry); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistRootKeyMeta(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	ws := memdb.NewWatchSet()
	keys, err := s.snap.RootKeyMetas(ws)
	if err != nil {
		return err
	}

	for {
		raw := keys.Next()
		if raw == nil {
			break
		}
		key := raw.(*structs.RootKeyMeta)
		sink.Write([]byte{byte(RootKeyMetaSnapshot)})
		if err := encoder.Encode(key); err != nil {
			return err
		}
	}
	return nil
}

func (s *nomadSnapshot) persistACLRoles(sink raft.SnapshotSink,
	encoder *codec.Encoder) error {

	// Get all the ACL roles.
	ws := memdb.NewWatchSet()
	aclRolesIter, err := s.snap.GetACLRoles(ws)
	if err != nil {
		return err
	}

	for {
		// Get the next item.
		for raw := aclRolesIter.Next(); raw != nil; raw = aclRolesIter.Next() {

			// Prepare the request struct.
			role := raw.(*structs.ACLRole)

			// Write out an ACL role snapshot.
			sink.Write([]byte{byte(ACLRoleSnapshot)})
			if err := encoder.Encode(role); err != nil {
				return err
			}
		}
		return nil
	}
}

// Release is a no-op, as we just need to GC the pointer
// to the state store snapshot. There is nothing to explicitly
// cleanup.
func (s *nomadSnapshot) Release() {}
