// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package jobs3

import (
	"bytes"
	"context"
	"io"

	nomadapi "github.com/hashicorp/nomad/api"
	"github.com/shoenig/test/must"
)

// Exec executes cmd in the given task of a random allocation of the given
// group.
func (sub *Submission) Exec(group, task string, cmd []string) Logs {
	queryOpts := sub.queryOptions()
	jobsAPI := sub.nomadClient.Jobs()
	stubs, _, err := jobsAPI.Allocations(sub.jobID, false, queryOpts)
	must.NoError(sub.t, err, must.Sprintf("failed to query allocations for %s", group))

	var allocID string
	for _, stub := range stubs {
		if stub.TaskGroup == group {
			allocID = stub.ID
			break
		}
	}
	must.NotEq(sub.t, "", allocID, must.Sprintf("no allocation found for %s", group))

	// do stuff
	allocsAPI := sub.nomadClient.Allocations()
	alloc, _, err := allocsAPI.Info(allocID, queryOpts)
	must.NoError(sub.t, err, must.Sprintf("failed to query allocation %s", allocID))

	var (
		stdout   bytes.Buffer
		stderr   bytes.Buffer
		input    io.Reader = bytes.NewReader(nil)
		tty      bool
		resizeCh chan (nomadapi.TerminalSize) = make(chan (nomadapi.TerminalSize))
	)

	ctx, cancel := context.WithTimeout(context.Background(), sub.timeout)
	defer cancel()

	sub.logf("alloc exec %s in %s/%s, id: %s", cmd, group, task, allocID)
	exitCode, err := allocsAPI.Exec(
		ctx,
		alloc,
		task,
		tty,
		cmd,
		input,
		&stdout,
		&stderr,
		resizeCh,
		queryOpts,
	)
	sub.logf("alloc exec exit code: %d in: %s", exitCode, allocID)

	must.NoError(sub.t, err, must.Sprintf("failed to exec cmd %q in %s/%s (%s)", cmd, group, task, allocID))
	must.Zero(sub.t, exitCode, must.Sprintf("expected success exit code executing %s in %s/%s %s", cmd, group, task, allocID))

	return Logs{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
}
