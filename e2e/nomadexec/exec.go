package nomadexec

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/plugins/drivers"
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
	cases := []struct {
		name        string
		command     string
		tty         bool
		stdin       string
		stdout      string
		stderr      string
		exitCode    int
		customizeFn func(*drivers.ExecOptions, chan drivers.TerminalSize)
	}{
		{
			name:     "notty: basic",
			command:  "echo hello stdout; echo hello stderr >&2; exit 43",
			tty:      false,
			stdout:   "hello stdout\n",
			stderr:   "hello stderr\n",
			exitCode: 43,
		},
		{
			name:     "notty: streaming",
			command:  "for n in 1 2 3; do echo $n; sleep 1; done",
			tty:      false,
			stdout:   "1\n2\n3\n",
			exitCode: 0,
		},
		{
			name:     "ntty: stty check",
			command:  "stty size",
			tty:      false,
			stderr:   "stty: standard input: Inappropriate ioctl for device\n",
			exitCode: 1,
		},
		{
			name:     "notty: stdin passing",
			command:  "echo hello from command; cat",
			tty:      false,
			stdin:    "hello from stdin\n",
			stdout:   "hello from command\nhello from stdin\n",
			exitCode: 0,
		},
		{
			name:     "notty: stdin passing",
			command:  "echo hello from command; cat",
			tty:      false,
			stdin:    "hello from stdin\n",
			stdout:   "hello from command\nhello from stdin\n",
			exitCode: 0,
		},
		{
			name:    "notty: children processes",
			command: "(( sleep 3; echo from background ) & ); echo from main; exec sleep 1",
			tty:     false,
			// when not using tty; wait for all processes to exit matching behavior of `docker exec`
			stdout:   "from main\nfrom background\n",
			exitCode: 0,
		},
	}

	for _, c := range cases {
		f.T().Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			resize := make(chan api.TerminalSize)

			stdin := ioutil.NopCloser(strings.NewReader(c.stdin))

			exitCode, err := tc.Nomad().Allocations().Exec(context.Background(),
				&tc.alloc, "task", c.tty,
				[]string{"/bin/sh", "-c", c.command},
				stdin, &stdout, &stderr,
				resize, nil)

			require.NoError(t, err)
			require.Equal(t, c.exitCode, exitCode)
			require.Equal(t, c.stdout, stdout.String())
			require.Equal(t, c.stderr, stderr.String())
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
