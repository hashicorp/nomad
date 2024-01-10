// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomad

import (
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

// jobConsulHook is a job registration admission controller for Consul
// configuration in Consul, Service, and Template blocks
type jobConsulHook struct {
	srv *Server
}

func (jobConsulHook) Name() string {
	return "consul"
}

// validateTaskPartitionMatchesGroup validates that any partition set for the
// task.Consul matches any partition set for the group
func (jobConsulHook) validateTaskPartitionMatchesGroup(groupPartition string, taskConsul *structs.Consul) error {
	if taskConsul.Partition == "" || groupPartition == "" {
		return nil
	}
	if taskConsul.Partition != groupPartition {
		return fmt.Errorf("task.consul.partition %q must match group.consul.partition %q if both are set", taskConsul.Partition, groupPartition)
	}
	return nil
}

// mutateImpl ensures that the job's Consul blocks have been configured with the
// correct Consul cluster if unset, and sets constraints on the Consul admin
// partition if set. This should be called by the Mutate method.
func (jobConsulHook) mutateImpl(job *structs.Job, defaultCluster string) *structs.Job {
	for _, group := range job.TaskGroups {
		if group.Consul != nil {
			if group.Consul.Cluster == "" {
				group.Consul.Cluster = defaultCluster
			}
			if group.Consul.Partition != "" {
				group.Constraints = append(group.Constraints,
					newConsulPartitionConstraint(group.Consul.Cluster, group.Consul.Partition))
			}
		}

		for _, service := range group.Services {
			if service.IsConsul() && service.Cluster == "" {
				service.Cluster = defaultCluster
			}
		}

		for _, task := range group.Tasks {
			if task.Consul != nil {
				if task.Consul.Cluster == "" {
					task.Consul.Cluster = defaultCluster
				}
				if task.Consul.Partition != "" {
					task.Constraints = append(task.Constraints,
						newConsulPartitionConstraint(task.Consul.Cluster, task.Consul.Partition))
				}
			}
			for _, service := range task.Services {
				if service.IsConsul() && service.Cluster == "" {
					service.Cluster = defaultCluster
				}
			}
		}
	}

	return job
}

// newConsulPartitionConstraint produces a constraint on the Consul admin
// partition, based on the cluster name. In Nomad CE this will always be in the
// default cluster.
func newConsulPartitionConstraint(cluster, partition string) *structs.Constraint {
	if cluster == structs.ConsulDefaultCluster || cluster == "" {
		return &structs.Constraint{
			LTarget: "${attr.consul.partition}",
			RTarget: partition,
			Operand: "=",
		}
	}
	return &structs.Constraint{
		LTarget: "${attr.consul." + cluster + ".partition}",
		RTarget: partition,
		Operand: "=",
	}
}
