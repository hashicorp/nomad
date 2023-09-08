// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/nomad/client/config"
	"github.com/hashicorp/nomad/helper"
	"github.com/hashicorp/nomad/nomad/structs"
)

func (c *Client) DrainSelf() error {
	drainSpec := c.GetConfig().Drain
	if drainSpec == nil {
		return nil
	}

	logger := c.logger.Named("drain")

	now := time.Now()
	drainReq := &structs.NodeUpdateDrainRequest{
		NodeID: c.NodeID(),
		DrainStrategy: &structs.DrainStrategy{
			DrainSpec: structs.DrainSpec{
				Deadline:         drainSpec.Deadline,
				IgnoreSystemJobs: drainSpec.IgnoreSystemJobs,
			},
			StartedAt: now,
		},
		MarkEligible: false,
		Meta:         map[string]string{"message": "shutting down"},
		WriteRequest: structs.WriteRequest{
			Region: c.Region(), AuthToken: c.secretNodeID()},
	}
	if drainSpec.Deadline > 0 {
		drainReq.DrainStrategy.ForceDeadline = now.Add(drainSpec.Deadline)
	}

	var drainResp structs.NodeDrainUpdateResponse
	err := c.RPC("Node.UpdateDrain", drainReq, &drainResp)
	if err != nil {
		return err
	}

	// note: the default deadline is 1hr but could be set to "". letting this
	// run forever seems wrong but init system (ex systemd) will almost always
	// force kill the client eventually
	ctx := context.Background()
	var cancel context.CancelFunc
	if drainSpec.Deadline > 0 {
		// if we set this context to the deadline, the server will reach the
		// deadline but not get a chance to record it before this context
		// expires, resulting in spurious errors. So extend the deadline here by
		// a few seconds
		ctx, cancel = context.WithTimeout(context.Background(), drainSpec.Deadline+(5*time.Second))
		defer cancel()
	}
	statusCheckInterval := time.Second

	logger.Info("monitoring self-drain")
	err = c.pollServerForDrainStatus(ctx, statusCheckInterval)
	switch err {
	case nil:
		logger.Debug("self-drain complete")
		return nil
	case context.DeadlineExceeded, context.Canceled:
		logger.Error("self-drain exceeded deadline")
		return fmt.Errorf("self-drain exceeded deadline")
	default:
		logger.Error("could not check node status, falling back to local status checks", "error", err)
	}

	err = c.pollLocalStatusForDrainStatus(ctx, statusCheckInterval, drainSpec)
	if err != nil {
		return fmt.Errorf("self-drain exceeded deadline")
	}

	logger.Debug("self-drain complete")
	return nil
}

// pollServerForDrainStatus will poll the server periodically for the client's
// drain status, returning an error if the context expires or get any error from
// the RPC call. If this function returns nil, the drain was successful.
func (c *Client) pollServerForDrainStatus(ctx context.Context, interval time.Duration) error {
	timer, stop := helper.NewSafeTimer(0)
	defer stop()

	statusReq := &structs.NodeSpecificRequest{
		NodeID:   c.NodeID(),
		SecretID: c.secretNodeID(),
		QueryOptions: structs.QueryOptions{
			Region: c.Region(), AuthToken: c.secretNodeID()},
	}
	var statusResp structs.SingleNodeResponse

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			err := c.RPC("Node.GetNode", statusReq, &statusResp)
			if err != nil {
				return err
			}
			if &statusResp != nil && statusResp.Node.DrainStrategy == nil {
				return nil
			}
			timer.Reset(interval)
		}
	}
}

// pollLocalStatusForDrainStatus polls the local allocrunner state periodicially
// for the client status of all allocation runners, returning an error if the
// context expires or get any error from the RPC call. If this function returns
// nil, the drain was successful. This is a fallback function in case polling
// the server fails.
func (c *Client) pollLocalStatusForDrainStatus(ctx context.Context,
	interval time.Duration, drainSpec *config.DrainConfig) error {

	// drainIsDone is its own function scope so we can release the allocLock
	// between poll attempts
	drainIsDone := func() bool {
		c.allocLock.RLock()
		defer c.allocLock.RUnlock()
		for _, runner := range c.allocs {

			// note: allocs in runners should never be nil or have a nil Job but
			// if they do we can safely assume the runner is done with it
			alloc := runner.Alloc()
			if alloc != nil && !alloc.ClientTerminalStatus() {
				if !drainSpec.IgnoreSystemJobs {
					return false
				}
				if alloc.Job == nil {
					continue
				}
				if alloc.Job.Type != structs.JobTypeSystem {
					return false
				}
			}
		}
		return true
	}

	timer, stop := helper.NewSafeTimer(0)
	defer stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			if drainIsDone() {
				return nil
			}
			timer.Reset(interval)
		}

	}
}
