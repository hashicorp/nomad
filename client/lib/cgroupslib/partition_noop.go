// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cgroupslib

import (
	"github.com/hashicorp/nomad/client/lib/idset"
	"github.com/hashicorp/nomad/client/lib/numalib/hw"
)

func NoopPartition() Partition {
	return new(noop)
}

type noop struct{}

func (p *noop) Reserve(*idset.Set[hw.CoreID]) error {
	return nil
}

func (p *noop) Release(*idset.Set[hw.CoreID]) error {
	return nil
}

func (p *noop) Restore(*idset.Set[hw.CoreID]) {}
