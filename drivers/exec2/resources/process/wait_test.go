// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package process

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

func TestOrphan_Wait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()

	cmd := exec.CommandContext(ctx, "sleep", ".1s")
	must.NoError(t, cmd.Start())

	waiter := WaitOnOrphan(cmd.Process.Pid)
	exit := waiter.Wait()

	must.Greater(t, 100*time.Millisecond, time.Since(start))
	must.NoError(t, exit.Err)
	must.Zero(t, exit.Code)
}

func TestOrphan_WaitFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "sleep", "abc") // exit 1
	must.NoError(t, cmd.Start())

	waiter := WaitOnOrphan(cmd.Process.Pid)
	exit := waiter.Wait()

	must.NoError(t, exit.Err)
	must.One(t, exit.Code)
}
