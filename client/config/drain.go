// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"fmt"
	"time"

	"github.com/hashicorp/nomad/nomad/structs/config"
)

// DrainConfig describes a Node's drain behavior on graceful shutdown.
type DrainConfig struct {
	// Deadline is the duration after the drain starts when client will stop
	// waiting for allocations to stop.
	Deadline time.Duration

	// IgnoreSystemJobs allows systems jobs to remain on the node even though it
	// has been marked for draining.
	IgnoreSystemJobs bool

	// Force causes the drain to stop all the allocations immediately, ignoring
	// their jobs' migrate blocks.
	Force bool
}

// DrainConfigFromAgent creates the internal read-only copy of the client
// agent's DrainConfig.
func DrainConfigFromAgent(c *config.DrainConfig) (*DrainConfig, error) {
	if c == nil {
		return nil, nil
	}

	deadline := time.Hour
	ignoreSystemJobs := false
	force := false

	if c.Deadline != nil {
		var err error
		deadline, err = time.ParseDuration(*c.Deadline)
		if err != nil {
			return nil, fmt.Errorf("error parsing Deadline: %w", err)
		}
	}
	if c.IgnoreSystemJobs != nil {
		ignoreSystemJobs = *c.IgnoreSystemJobs
	}
	if c.Force != nil {
		force = *c.Force
	}

	return &DrainConfig{
		Deadline:         deadline,
		IgnoreSystemJobs: ignoreSystemJobs,
		Force:            force,
	}, nil
}
