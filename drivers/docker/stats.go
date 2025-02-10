// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	containerapi "github.com/docker/docker/api/types/container"
	"github.com/hashicorp/nomad/client/lib/cpustats"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/drivers/docker/util"
	"github.com/hashicorp/nomad/helper"
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

// collectStats starts collecting resource usage stats of a Docker container
// and does this until the context or the tasks handler done channel is closed.
func (h *taskHandle) collectStats(ctx context.Context, destCh *usageSender, interval time.Duration, compute cpustats.Compute) {
	defer destCh.close()

	// retry tracks the number of retries the collection has been through since
	// the last successful Docker API call. This is used to calculate the
	// backoff time for the collection ticker.
	var retry uint64

	ticker, cancel := helper.NewSafeTicker(interval)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		case <-h.doneCh:
			return
		case <-ticker.C:
			stats, err := h.collectDockerStats(ctx)
			switch err {
			case nil:
				resourceUsage := util.DockerStatsToTaskResourceUsage(stats, compute)
				destCh.send(resourceUsage)
				ticker.Reset(interval)
				retry = 0
			default:
				h.logger.Error("error collecting stats from container", "error", err)
				ticker.Reset(helper.Backoff(statsCollectorBackoffBaseline, statsCollectorBackoffLimit, retry))
				retry++
			}
		}
	}
}

// collectDockerStats performs the stats collection from the Docker API. It is
// split into its own function for the purpose of aiding testing.
func (h *taskHandle) collectDockerStats(ctx context.Context) (*containerapi.Stats, error) {

	var stats *containerapi.Stats

	statsReader, err := h.dockerClient.ContainerStats(ctx, h.containerID, false)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to collect stats: %w", err)
	}

	// Ensure the body is not nil to avoid potential panics. The statsReader
	// itself cannot be nil, so there is no need to check this.
	if statsReader.Body == nil {
		return nil, errors.New("error decoding stats data: no reader body")
	}

	err = json.NewDecoder(statsReader.Body).Decode(&stats)
	_ = statsReader.Body.Close()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to decode Docker response: %w", err)
	}

	if stats == nil {
		return nil, errors.New("error decoding stats data: stats were nil")
	}

	return stats, nil
}
