// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"github.com/shoenig/netlog"

	"os"
	"time"

	"github.com/hashicorp/nomad/helper"
)

const (
	cpusetSyncPeriod = 3 * time.Second
)

var (
	log = netlog.New("corefix")
)

type cpuset struct {
	doneCh      <-chan bool
	source      string
	destination string
	previous    string
	sync        func(string, string, string)
}

func (c *cpuset) watch() {
	ticks, cancel := helper.NewSafeTimer(cpusetSyncPeriod)
	defer cancel()

	for {
		select {
		case <-c.doneCh:
			return
		case <-ticks.C:
			c.sync(c.previous, c.source, c.destination)
			ticks.Reset(cpusetSyncPeriod)
		}
	}
}

func copyCpuset(previous, source, destination string) {
	log.Info("copyCpuset", "previous", previous, "source", source, "destination", destination)
	b, err := os.ReadFile(source)
	if err != nil {
		log.Error("copyCpuset", "error1", err)
		return
	}
	if string(b) == previous {
		log.Error("copyCpuset", "skip", previous)
		return
	}
	_ = os.WriteFile(destination, b, 0644)
}
