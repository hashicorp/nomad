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
