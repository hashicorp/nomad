// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
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
	return &usageSender{
		destCh: destCh,
	}, destCh

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
func (h *taskHandle) Stats(ctx context.Context, interval time.Duration, top cpustats.Topology) (<-chan *cstructs.TaskResourceUsage, error) {
	select {
	case <-h.doneCh:
		return nil, nstructs.NewRecoverableError(fmt.Errorf("container stopped"), false)
	default:
	}

	destCh, recvCh := newStatsChanPipe()
	go h.collectStats(ctx, destCh, interval, top)
	return recvCh, nil
}

// collectStats starts collecting resource usage stats of a docker container
func (h *taskHandle) collectStats(ctx context.Context, destCh *usageSender, interval time.Duration, top cpustats.Topology) {
	defer destCh.close()

	// backoff and retry used if the docker stats API returns an error
	var backoff time.Duration
	var retry uint64

	// create an interval timer
	timer, stop := helper.NewSafeTimer(backoff)
	defer stop()

	// loops until doneCh is closed
	for {
		timer.Reset(backoff)

		if backoff > 0 {
			select {
			case <-timer.C:
			case <-ctx.Done():
				return
			case <-h.doneCh:
				return
			}
		}

		// make a channel for docker stats structs and start a collector to
		// receive stats from docker and emit nomad stats
		// statsCh will always be closed by docker client.
		statsCh := make(chan *docker.Stats)
		go dockerStatsCollector(destCh, statsCh, interval, top)

		statsOpts := docker.StatsOptions{
			ID:      h.containerID,
			Context: ctx,
			Done:    h.doneCh,
			Stats:   statsCh,
			Stream:  true,
		}

		// Stats blocks until an error has occurred, or doneCh has been closed
		if err := h.dockerClient.Stats(statsOpts); err != nil && err != io.ErrClosedPipe {
			// An error occurred during stats collection, retry with backoff
			h.logger.Debug("error collecting stats from container", "error", err)

			// Calculate the new backoff
			backoff = helper.Backoff(statsCollectorBackoffBaseline, statsCollectorBackoffLimit, retry)
			retry++
			continue
		}
		// Stats finished either because context was canceled, doneCh was closed
		// or the container stopped. Stop stats collections.
		return
	}
}

func dockerStatsCollector(destCh *usageSender, statsCh <-chan *docker.Stats, interval time.Duration, top cpustats.Topology) {
	var resourceUsage *cstructs.TaskResourceUsage

	// hasSentInitialStats is used so as to emit the first stats received from
	// the docker daemon
	var hasSentInitialStats bool

	// timer is used to send nomad status at the specified interval
	timer := time.NewTimer(interval)
	for {
		select {
		case <-timer.C:
			// it is possible for the timer to go off before the first stats
			// has been emitted from docker
			if resourceUsage == nil {
				continue
			}

			// sending to destCh could block, drop this interval if it does
			destCh.send(resourceUsage)

			timer.Reset(interval)

		case s, ok := <-statsCh:
			// if statsCh is closed stop collection
			if !ok {
				return
			}
			// s should always be set, but check and skip just in case
			if s != nil {
				resourceUsage = util.DockerStatsToTaskResourceUsage(s, top)
				// send stats next interation if this is the first time received
				// from docker
				if !hasSentInitialStats {
					timer.Reset(0)
					hasSentInitialStats = true
				}
			}
		}
	}
}
