// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	containerapi "github.com/docker/docker/api/types/container"
	"github.com/hashicorp/nomad/client/lib/cpustats"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/docker/util"
	"github.com/hashicorp/nomad/helper"

	//"github.com/hashicorp/nomad/helper"
	nstructs "github.com/hashicorp/nomad/nomad/structs"
)

const (
	// statsCollectorBackoffBaseline is the baseline time for exponential
	// backoff while calling the docker stats api.
	statsCollectorBackoffBaseline = 5 * time.Second

	// statsCollectorBackoffLimit is the limit of the exponential backoff for
	// calling the docker stats api.
	statsCollectorBackoffLimit = 2 * time.Minute
)

// usageSender wraps a TaskResourceUsage chan such that it supports concurrent
// sending and closing, and backpressures by dropping events if necessary.
type usageSender struct {
	closed bool
	destCh chan<- *cstructs.TaskResourceUsage
	mu     sync.Mutex
}

// newStatsChanPipe returns a chan wrapped in a struct that supports concurrent
// sending and closing, and the receiver end of the chan.
func newStatsChanPipe() (*usageSender, <-chan *cstructs.TaskResourceUsage) {
	destCh := make(chan *cstructs.TaskResourceUsage, 1)
	return &usageSender{destCh: destCh}, destCh

}

// send resource usage to the receiver unless the chan is already full or
// closed.
func (u *usageSender) send(tru *cstructs.TaskResourceUsage) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.closed {
		return
	}

	select {
	case u.destCh <- tru:
	default:
		// Backpressure caused missed interval
	}
}

// close resource usage. Any further sends will be dropped.
func (u *usageSender) close() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		// already closed
		return
	}

	u.closed = true
	close(u.destCh)
}

// Stats starts collecting stats from the docker daemon and sends them on the
// returned channel.
func (h *taskHandle) Stats(ctx context.Context, interval time.Duration, compute cpustats.Compute) (<-chan *cstructs.TaskResourceUsage, error) {
	select {
	case <-h.doneCh:
		return nil, nstructs.NewRecoverableError(fmt.Errorf("container stopped"), false)
	default:
	}

	destCh, recvCh := newStatsChanPipe()
	go h.collectStats(ctx, destCh, interval, compute)
	return recvCh, nil
}

// collectStats starts collecting resource usage stats of a docker container
func (h *taskHandle) collectStats(ctx context.Context, destCh *usageSender, interval time.Duration, compute cpustats.Compute) {
	defer destCh.close()

	ticker, cancel := helper.NewSafeTicker(interval)
	defer cancel()
	var stats *containerapi.Stats

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.doneCh:
			return
		case <-ticker.C:
			// we need to use the streaming stats API here because our calculation for
			// CPU usage depends on having the values from the previous read, which are
			// not available in one-shot. This streaming stats can be reused over time,
			// but require synchronization, which restricts the interval for the metrics.
			statsReader, err := h.dockerClient.ContainerStats(ctx, h.containerID, true)
			if err != nil && err != io.EOF {
				h.logger.Debug("error collecting stats from container", "error", err)
				return
			}

			err = json.NewDecoder(statsReader.Body).Decode(&stats)
			statsReader.Body.Close()
			if err != nil && err != io.EOF {
				h.logger.Error("error decoding stats data from container", "error", err)
				return
			}

			if stats == nil {
				h.logger.Error("error decoding stats data: stats were nil")
				return
			}

			resourceUsage := util.DockerStatsToTaskResourceUsage(stats, compute)
			destCh.send(resourceUsage)
		}
	}
}
