// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"github.com/shoenig/netlog"

	"os"
	"path/filepath"
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
	sync        func(string, string)
}

func (c *cpuset) watch() {
	if c.sync == nil {
		// use the real thing if we are not doing tests
		c.sync = c.copyCpuset
	}

	ticks, cancel := helper.NewSafeTimer(cpusetSyncPeriod)
	defer cancel()

	for {
		select {
		case <-c.doneCh:
			return
		case <-ticks.C:
			c.sync(c.source, c.destination)
			ticks.Reset(cpusetSyncPeriod)
		}
	}
}

func (c *cpuset) copyCpuset(source, destination string) {
	source = filepath.Join(source, "cpuset.cpus.effective")
	destination = filepath.Join(destination, "cpuset.cpus")

	log.Info("copyCpuset", "previous", c.previous, "source", source, "destination", destination)
	b, err := os.ReadFile(source)
	if err != nil {
		log.Error("copyCpuset", "error1", err)
		return
	}
	current := string(b)
	if current == c.previous {
		log.Error("copyCpuset", "skip", c.previous)
		return
	}
	err = os.WriteFile(destination, b, 0644)
	if err != nil {
		log.Error("copyCpuset", "error2", err)
		return
	}
	c.previous = current
}
