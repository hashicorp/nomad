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

func consulPartitionConstraint(cluster, partition string) *structs.Constraint {
	if cluster != structs.ConsulDefaultCluster && cluster != "" {
		return &structs.Constraint{
			LTarget: fmt.Sprintf("${attr.consul.%s.partition}", cluster),
			RTarget: partition,
			Operand: "=",
		}
	}
	return &structs.Constraint{
		LTarget: "${attr.consul.partition}",
		RTarget: partition,
		Operand: "=",
	}
}
