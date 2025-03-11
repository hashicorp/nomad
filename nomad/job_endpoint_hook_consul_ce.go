// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !ent
// +build !ent

package nomad

import (
	"errors"

	"github.com/hashicorp/nomad/nomad/structs"
)

func (h jobConsulHook) Validate(job *structs.Job) ([]error, error) {

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
			}
		}

		for _, task := range group.Tasks {
			for _, service := range task.Services {
				if service.Provider == structs.ServiceProviderConsul {
					if err := h.validateCluster(service.Cluster); err != nil {
						return nil, err
					}
				}
			}

			if task.Consul != nil {
				err := h.validateTaskPartitionMatchesGroup(groupPartition, task.Consul)
				if err != nil {
					return nil, err
				}

				if err := h.validateCluster(task.Consul.Cluster); err != nil {
					return nil, err
				}
			}
		}
	}

	return nil, nil
}

func (h jobConsulHook) validateCluster(name string) error {
	if name != structs.ConsulDefaultCluster {
		return errors.New("non-default Consul cluster requires Nomad Enterprise")
	}
	return nil
}

// Mutate ensures that the job's Consul cluster has been configured to be the
// default Consul cluster if unset
func (j jobConsulHook) Mutate(job *structs.Job) (*structs.Job, []error, error) {
	return j.mutateImpl(job, structs.ConsulDefaultCluster), nil, nil
}
