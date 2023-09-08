// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import "github.com/hashicorp/nomad/helper/pointer"

// DrainConfig describes a Node's drain behavior on graceful shutdown.
type DrainConfig struct {
	// Deadline is the duration after the drain starts when client will stop
	// waiting for allocations to stop.
	Deadline *string `hcl:"deadline"`

	// IgnoreSystemJobs allows systems jobs to remain on the node even though it
	// has been marked for draining.
	IgnoreSystemJobs *bool `hcl:"ignore_system_jobs"`

	// Force causes the drain to stop all the allocations immediately, ignoring
	// their jobs' migrate blocks.
	Force *bool `hcl:"force"`
}

func (d *DrainConfig) Copy() *DrainConfig {
	if d == nil {
		return nil
	}

	nd := new(DrainConfig)
	*nd = *d
	return nd
}

func (d *DrainConfig) Merge(o *DrainConfig) *DrainConfig {
	switch {
	case d == nil:
		return o.Copy()
	case o == nil:
		return d.Copy()
	default:
		nd := d.Copy()
		if o.Deadline != nil {
			nd.Deadline = pointer.Copy(o.Deadline)
		}
		if o.IgnoreSystemJobs != nil && *o.IgnoreSystemJobs {
			nd.IgnoreSystemJobs = pointer.Of(true)
		}
		if o.Force != nil && *o.Force {
			nd.Force = pointer.Of(true)
		}
		return nd
	}
}
