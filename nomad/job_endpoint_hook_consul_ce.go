// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"errors"
	"fmt"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (h jobConsulHook) Validate(job *structs.Job) ([]error, error) {

	requiresToken := false

	clusterNeedsToken := func(name string, identity *structs.WorkloadIdentity) bool {
		if identity != nil {
			return false
		}
		config := h.srv.config.ConsulConfigs[name]
		if config != nil {
			return !*config.AllowUnauthenticated
		}
		return false
	}

	for _, group := range job.TaskGroups {

		groupPartition := ""

		if group.Consul != nil {
			groupPartition = group.Consul.Partition
			if err := h.validateCluster(group.Consul.Cluster); err != nil {
				return nil, err
			}
		}

		for _, service := range group.Services {
			if service.Provider == structs.ServiceProviderConsul {
				if err := h.validateCluster(service.Cluster); err != nil {
					return nil, err
				}
				requiresToken = clusterNeedsToken(
					service.Cluster, service.Identity) || requiresToken
			}
		}

		for _, task := range group.Tasks {
			for _, service := range task.Services {
				if service.Provider == structs.ServiceProviderConsul {
					if err := h.validateCluster(service.Cluster); err != nil {
						return nil, err
					}
					requiresToken = clusterNeedsToken(
						service.Cluster, service.Identity) || requiresToken
				}
			}

			if task.Consul != nil {
				if task.Consul.Partition != "" {
					if groupPartition != "" && task.Consul.Partition != groupPartition {
						return nil, fmt.Errorf("task.consul.partition %q must match group.consul.partition %q if both are set", task.Consul.Partition, groupPartition)
					}
				}

				if err := h.validateCluster(task.Consul.Cluster); err != nil {
					return nil, err
				}
				var clusterIdentity *structs.WorkloadIdentity
				for _, identity := range task.Identities {
					if identity.Name == "consul_"+task.Consul.Cluster {
						clusterIdentity = identity
						break
					}
				}
				requiresToken = clusterNeedsToken(
					task.Consul.Cluster, clusterIdentity) || requiresToken
			}
		}
	}

	if requiresToken {
		return []error{
			errors.New("Setting a Consul token when submitting a job is deprecated and will be removed in Nomad 1.9. Migrate your Consul configuration to use workload identity")}, nil
	}

	return nil, nil
}

func (h jobConsulHook) validateCluster(name string) error {
	if name != structs.ConsulDefaultCluster {
		return errors.New("non-default Consul cluster requires Nomad Enterprise")
	}
	return nil
}

func consulPartitionConstraint(partition string) *structs.Constraint {
	return &structs.Constraint{
		LTarget: "${attr.consul.partition}",
		RTarget: partition,
		Operand: "=",
	}
}

// Mutate ensures that the job's Consul cluster has been configured to be the
// default Consul cluster if unset
func (j jobConsulHook) Mutate(job *structs.Job) (*structs.Job, []error, error) {
	for _, group := range job.TaskGroups {
		if group.Consul != nil {
			if group.Consul.Cluster == "" {
				group.Consul.Cluster = structs.ConsulDefaultCluster
			}
			if group.Consul.Partition != "" {
				group.Constraints = append(group.Constraints,
					consulPartitionConstraint(group.Consul.Partition))
			}
		}

		for _, service := range group.Services {
			if service.IsConsul() && service.Cluster == "" {
				service.Cluster = structs.ConsulDefaultCluster
			}
		}

		for _, task := range group.Tasks {
			if task.Consul != nil {
				if task.Consul.Cluster == "" {
					task.Consul.Cluster = structs.ConsulDefaultCluster
				}
				if task.Consul.Partition != "" {
					task.Constraints = append(task.Constraints,
						consulPartitionConstraint(task.Consul.Partition))
				}
			}
			for _, service := range task.Services {
				if service.IsConsul() && service.Cluster == "" {
					service.Cluster = structs.ConsulDefaultCluster
				}
			}
		}
	}

	return job, nil, nil
}
