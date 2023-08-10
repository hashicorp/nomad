// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nomadexec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	dtestutils "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/stretchr/testify/assert"
)

type NomadExecE2ETest struct {
	framework.TC

	name        string
	jobFilePath string

	jobID string
	alloc api.Allocation
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "Nomad exec",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			newNomadExecE2eTest("docker", "./nomadexec/testdata/docker.nomad"),
		},
	})
}

func newNomadExecE2eTest(name, jobFilePath string) *NomadExecE2ETest {
	return &NomadExecE2ETest{
		name:        name,
		jobFilePath: jobFilePath,
	}
}

func (tc *NomadExecE2ETest) Name() string {
	return fmt.Sprintf("%v (%v)", tc.TC.Name(), tc.name)
}

func (tc *NomadExecE2ETest) BeforeAll(f *framework.F) {
	// Ensure cluster has leader before running tests
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)

	// register a job for execing into
	tc.jobID = "nomad-exec" + uuid.Generate()[:8]
	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), tc.Nomad(), tc.jobFilePath, tc.jobID, "")
	f.Len(allocs, 1)

	e2eutil.WaitForAllocRunning(f.T(), tc.Nomad(), allocs[0].ID)

	tc.alloc = api.Allocation{
		ID:        allocs[0].ID,
		Namespace: allocs[0].Namespace,
		NodeID:    allocs[0].NodeID,
	}
}

func (tc *NomadExecE2ETest) TestExecBasicResponses(f *framework.F) {
	for _, c := range dtestutils.ExecTaskStreamingBasicCases {
		f.T().Run(c.Name, func(t *testing.T) {

			stdin := newTestStdin(c.Tty, c.Stdin)
			var stdout, stderr bytes.Buffer

			resizeCh := make(chan api.TerminalSize)
			go func() {
				resizeCh <- api.TerminalSize{Height: 100, Width: 100}
			}()

			ctx, cancelFn := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancelFn()

			exitCode, err := tc.Nomad().Allocations().Exec(ctx,
				&tc.alloc, "task", c.Tty,
				[]string{"/bin/sh", "-c", c.Command},
				stdin, &stdout, &stderr,
				resizeCh, nil)

			assert.NoError(t, err)

			assert.Equal(t, c.ExitCode, exitCode)

			switch s := c.Stdout.(type) {
			case string:
				assert.Equal(t, s, stdout.String())
			case *regexp.Regexp:
				assert.Regexp(t, s, stdout.String())
			case nil:
				assert.Empty(t, stdout.String())
			default:
				assert.Fail(t, "unexpected stdout type", "found %v (%v), but expected string or regexp", s, reflect.TypeOf(s))
			}

			switch s := c.Stderr.(type) {
			case string:
				assert.Equal(t, s, stderr.String())
			case *regexp.Regexp:
				assert.Regexp(t, s, stderr.String())
			case nil:
				assert.Empty(t, stderr.String())
			default:
				assert.Fail(t, "unexpected stderr type", "found %v (%v), but expected string or regexp", s, reflect.TypeOf(s))
			}
		})
	}
}

func (tc *NomadExecE2ETest) AfterAll(f *framework.F) {
	jobs := tc.Nomad().Jobs()
	if tc.jobID != "" {
		jobs.Deregister(tc.jobID, true, nil)
	}
	tc.Nomad().System().GarbageCollect()
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
