// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package cgroupslib

import (
	"github.com/hashicorp/nomad/client/lib/idset"
)

func NoopPartition() Partition {
	return new(noop)
}

type noop struct{}

func (p *noop) Reserve(*idset.Set[idset.CoreID]) error {
	return nil
}

func (p *noop) Release(*idset.Set[idset.CoreID]) error {
	return nil
}

func (p *noop) Restore(*idset.Set[idset.CoreID]) {}
