// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomadexec

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/v3/cluster3"
	"github.com/hashicorp/nomad/e2e/v3/jobs3"
	dtestutils "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/shoenig/test/must"
)

func TestNomadExec(t *testing.T) {
	cluster3.Establish(t,
		cluster3.Leader(),
		cluster3.LinuxClients(1),
	)

	t.Run("testDocker", testDocker)
}

func getAlloc(t *testing.T, allocID string) *api.Allocation {
	allocsAPI := e2eutil.NomadClient(t).Allocations()
	info, _, err := allocsAPI.Info(allocID, nil)
	must.NoError(t, err)
	return info
}

func testDocker(t *testing.T) {
	job, jobCleanup := jobs3.Submit(t, "./input/busybox.hcl")
	t.Cleanup(jobCleanup)

	alloc := getAlloc(t, job.AllocID("group"))

	for _, tc := range dtestutils.ExecTaskStreamingBasicCases {
		t.Run(tc.Name, func(t *testing.T) {
			stdin := newTestStdin(tc.Tty, tc.Stdin)
			var stdout, stderr bytes.Buffer

			resizeCh := make(chan api.TerminalSize)
			go func() {
				resizeCh <- api.TerminalSize{Height: 100, Width: 100}
			}()

			ctx, cancelFn := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancelFn()

			exitCode, err := e2eutil.NomadClient(t).Allocations().Exec(
				ctx,
				alloc, "task", tc.Tty,
				[]string{"/bin/sh", "-c", tc.Command},
				stdin, &stdout, &stderr,
				resizeCh, nil,
			)
			must.NoError(t, err)
			must.Eq(t, tc.ExitCode, exitCode)

			switch s := tc.Stdout.(type) {
			case string:
				must.Eq(t, s, stdout.String())
			case *regexp.Regexp:
				must.RegexMatch(t, s, stdout.String())
			case nil:
				must.Eq(t, "", stdout.String())
			default:
				must.Unreachable(t, must.Sprint("unexpected match type"))
			}

			switch s := tc.Stderr.(type) {
			case string:
				must.Eq(t, s, stderr.String())
			case *regexp.Regexp:
				must.RegexMatch(t, s, stderr.String())
			case nil:
				must.Eq(t, "", stderr.String())
			default:
				must.Unreachable(t, must.Sprint("unexpected match type"))
			}
		})
	}
}

func newTestStdin(tty bool, d string) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		pw.Write([]byte(d))

		// when testing TTY, leave connection open for the entire duration of command
		// closing stdin may cause TTY session prematurely before command completes
		if !tty {
			pw.Close()
		}

	}()

	return pr
}
