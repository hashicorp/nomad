// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/nomad/helper"
)

const (
	// cpusetSyncPeriod is how often we check to see if the cpuset of a task
	// needs to be updated - if there is no work to do, no action is taken
	cpusetSyncPeriod = 3 * time.Second
)

// cpuset is used to manage the cpuset.cpus interface file in the cgroup that
// docker daemon creates for the container being run by the task driver. we
// must do this hack because docker does not allow specifying a pre-existing
// cgroup in which to run the container (i.e. one that we control).
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

	// read the current value of usable cores
	b, err := os.ReadFile(source)
	if err != nil {
		return
	}

	// if the current value is the same as the value we wrote last,
	// there is nothing to do
	current := string(b)
	if current == c.previous {
		return
	}

	// otherwise write the new value
	err = os.WriteFile(destination, b, 0644)
	if err != nil {
		return
	}

	// we wrote a new value; store that value so we do not write it again
	c.previous = current
}
