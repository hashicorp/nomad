// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package cgroupslib

import (
	"github.com/shoenig/netlog"

	"github.com/hashicorp/nomad/client/lib/idset"
)

func NewPartition(cores *idset.Set[idset.CoreID]) Partition {
	return &partition{
		// share:   cores,
		// reserve: idset.Empty[idset.CoreID](),
	}
}

type partition struct {
	// share   *idset.Set[idset.CoreID]
	// reserve *idset.Set[idset.CoreID]
}

func (p *partition) Reserve(cores *idset.Set[idset.CoreID]) {
	netlog.Green("Partition.Reserve", "cores", cores)
}

func (p *partition) Release(cores *idset.Set[idset.CoreID]) {
	netlog.Blue("Partition.Release", "cores", cores)
}
