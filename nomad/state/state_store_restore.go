package state

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

// StateRestore is used to optimize the performance when restoring state by
// only using a single large transaction instead of thousands of sub
// transactions.
type StateRestore struct {
	txn *txn
}

// Abort is used to abort the restore operation
func (r *StateRestore) Abort() {
	r.txn.Abort()
}

// Commit is used to commit the restore operation
func (r *StateRestore) Commit() error {
	return r.txn.Commit()
}

// NodeRestore is used to restore a node
func (r *StateRestore) NodeRestore(node *structs.Node) error {
	if err := r.txn.Insert("nodes", node); err != nil {
		return fmt.Errorf("node insert failed: %v", err)
	}
	return nil
}

// JobRestore is used to restore a job
func (r *StateRestore) JobRestore(job *structs.Job) error {
	if err := r.txn.Insert("jobs", job); err != nil {
		return fmt.Errorf("job insert failed: %v", err)
	}
	return nil
}

// EvalRestore is used to restore an evaluation
func (r *StateRestore) EvalRestore(eval *structs.Evaluation) error {
	if err := r.txn.Insert("evals", eval); err != nil {
		return fmt.Errorf("eval insert failed: %v", err)
	}
	return nil
}

// AllocRestore is used to restore an allocation
func (r *StateRestore) AllocRestore(alloc *structs.Allocation) error {
	if err := r.txn.Insert("allocs", alloc); err != nil {
		return fmt.Errorf("alloc insert failed: %v", err)
	}
	return nil
}

// IndexRestore is used to restore an index
func (r *StateRestore) IndexRestore(idx *IndexEntry) error {
	if err := r.txn.Insert("index", idx); err != nil {
		return fmt.Errorf("index insert failed: %v", err)
	}
	return nil
}

// PeriodicLaunchRestore is used to restore a periodic launch.
func (r *StateRestore) PeriodicLaunchRestore(launch *structs.PeriodicLaunch) error {
	if err := r.txn.Insert("periodic_launch", launch); err != nil {
		return fmt.Errorf("periodic launch insert failed: %v", err)
	}
	return nil
}

// JobSummaryRestore is used to restore a job summary
func (r *StateRestore) JobSummaryRestore(jobSummary *structs.JobSummary) error {
	if err := r.txn.Insert("job_summary", jobSummary); err != nil {
		return fmt.Errorf("job summary insert failed: %v", err)
	}
	return nil
}

// JobVersionRestore is used to restore a job version
func (r *StateRestore) JobVersionRestore(version *structs.Job) error {
	if err := r.txn.Insert("job_version", version); err != nil {
		return fmt.Errorf("job version insert failed: %v", err)
	}
	return nil
}

// DeploymentRestore is used to restore a deployment
func (r *StateRestore) DeploymentRestore(deployment *structs.Deployment) error {
	if err := r.txn.Insert("deployment", deployment); err != nil {
		return fmt.Errorf("deployment insert failed: %v", err)
	}
	return nil
}

// VaultAccessorRestore is used to restore a vault accessor
func (r *StateRestore) VaultAccessorRestore(accessor *structs.VaultAccessor) error {
	if err := r.txn.Insert("vault_accessors", accessor); err != nil {
		return fmt.Errorf("vault accessor insert failed: %v", err)
	}
	return nil
}

// SITokenAccessorRestore is used to restore an SI token accessor
func (r *StateRestore) SITokenAccessorRestore(accessor *structs.SITokenAccessor) error {
	if err := r.txn.Insert(siTokenAccessorTable, accessor); err != nil {
		return fmt.Errorf("si token accessor insert failed: %w", err)
	}
	return nil
}

// ACLPolicyRestore is used to restore an ACL policy
func (r *StateRestore) ACLPolicyRestore(policy *structs.ACLPolicy) error {
	if err := r.txn.Insert("acl_policy", policy); err != nil {
		return fmt.Errorf("inserting acl policy failed: %v", err)
	}
	return nil
}

// ACLTokenRestore is used to restore an ACL token
func (r *StateRestore) ACLTokenRestore(token *structs.ACLToken) error {
	if err := r.txn.Insert("acl_token", token); err != nil {
		return fmt.Errorf("inserting acl token failed: %v", err)
	}
	return nil
}

// OneTimeTokenRestore is used to restore a one-time token
func (r *StateRestore) OneTimeTokenRestore(token *structs.OneTimeToken) error {
	if err := r.txn.Insert("one_time_token", token); err != nil {
		return fmt.Errorf("inserting one-time token failed: %v", err)
	}
	return nil
}

func (r *StateRestore) SchedulerConfigRestore(schedConfig *structs.SchedulerConfiguration) error {
	if err := r.txn.Insert("scheduler_config", schedConfig); err != nil {
		return fmt.Errorf("inserting scheduler config failed: %s", err)
	}
	return nil
}

func (r *StateRestore) ClusterMetadataRestore(meta *structs.ClusterMetadata) error {
	if err := r.txn.Insert("cluster_meta", meta); err != nil {
		return fmt.Errorf("inserting cluster meta failed: %v", err)
	}
	return nil
}

// ScalingPolicyRestore is used to restore a scaling policy
func (r *StateRestore) ScalingPolicyRestore(scalingPolicy *structs.ScalingPolicy) error {
	if err := r.txn.Insert("scaling_policy", scalingPolicy); err != nil {
		return fmt.Errorf("scaling policy insert failed: %v", err)
	}
	return nil
}

// CSIPluginRestore is used to restore a CSI plugin
func (r *StateRestore) CSIPluginRestore(plugin *structs.CSIPlugin) error {
	if err := r.txn.Insert("csi_plugins", plugin); err != nil {
		return fmt.Errorf("csi plugin insert failed: %v", err)
	}
	return nil
}

// CSIVolumeRestore is used to restore a CSI volume
func (r *StateRestore) CSIVolumeRestore(volume *structs.CSIVolume) error {
	if err := r.txn.Insert("csi_volumes", volume); err != nil {
		return fmt.Errorf("csi volume insert failed: %v", err)
	}
	return nil
}

// ScalingEventsRestore is used to restore scaling events for a job
func (r *StateRestore) ScalingEventsRestore(jobEvents *structs.JobScalingEvents) error {
	if err := r.txn.Insert("scaling_event", jobEvents); err != nil {
		return fmt.Errorf("scaling event insert failed: %v", err)
	}
	return nil
}

// NamespaceRestore is used to restore a namespace
func (r *StateRestore) NamespaceRestore(ns *structs.Namespace) error {
	if err := r.txn.Insert(TableNamespaces, ns); err != nil {
		return fmt.Errorf("namespace insert failed: %v", err)
	}
	return nil
}

// ServiceRegistrationRestore is used to restore a single service registration
// into the service_registrations table.
func (r *StateRestore) ServiceRegistrationRestore(service *structs.ServiceRegistration) error {
	if err := r.txn.Insert(TableServiceRegistrations, service); err != nil {
		return fmt.Errorf("service registration insert failed: %v", err)
	}
	return nil
}
