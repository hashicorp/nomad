package nomadexec

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	dtestutils "github.com/hashicorp/nomad/plugins/drivers/testutils"
	"github.com/stretchr/testify/require"
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
	// Ensure that we have four client nodes in ready state
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)

	// register a job for execing into
	tc.jobID = "nomad-exec" + uuid.Generate()[:8]
	allocs := e2eutil.RegisterAndWaitForAllocs(f.T(), tc.Nomad(), tc.jobFilePath, tc.jobID)
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
			stdin, stdout, stderr, readOutput, cleanupFn := dtestutils.NewIO(t, c.Tty, c.Stdin)
			defer cleanupFn()

			resizeCh := make(chan api.TerminalSize)
			go func() {
				resizeCh <- api.TerminalSize{Height: 100, Width: 100}
			}()

			exitCode, err := tc.Nomad().Allocations().Exec(context.Background(),
				&tc.alloc, "task", c.Tty,
				[]string{"/bin/sh", "-c", c.Command},
				stdin, stdout, stderr,
				resizeCh, nil)

			require.NoError(t, err)

			require.Equal(t, c.ExitCode, exitCode)

			// flush any pending writes
			stdin.Close()
			stdout.Close()
			stderr.Close()

			stdoutFound, stderrFound := readOutput()
			require.Equal(t, c.Stdout, stdoutFound)
			require.Equal(t, c.Stderr, stderrFound)
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
