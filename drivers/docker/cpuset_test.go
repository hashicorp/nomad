// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package docker

import (
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/shoenig/test/must"
)

func Test_cpuset_watch(t *testing.T) {
	ci.Parallel(t)

	doneCh := make(chan bool)

	source := "/source"
	destination := "/destination"
	hits := 0

	callback := func(s, d string) {
		must.Eq(t, source, s)
		must.Eq(t, destination, d)
		hits++
	}

	c := &cpuset{
		doneCh:      doneCh,
		source:      "/source",
		destination: "/destination",
		previous:    "",
		sync:        callback,
	}
	go c.watch()

	time.Sleep(3*time.Second + 10*time.Millisecond)
	doneCh <- true

	must.Eq(t, 1, hits)
}
