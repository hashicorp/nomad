package isolation

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

type PIDsNamespacing struct {
	framework.TC

	jobIDs []string
}

func init() {
	framework.AddSuites(&framework.TestSuite{
		Component:   "PIDS",
		CanRunLocal: true,
		Cases: []framework.TestCase{
			new(PIDsNamespacing),
		},
	})
}

func (tc *PIDsNamespacing) BeforeAll(f *framework.F) {
	t := f.T()
	e2eutil.WaitForLeader(t, tc.Nomad())
	e2eutil.WaitForNodesReady(t, tc.Nomad(), 1)
}

func (tc *PIDsNamespacing) TestIsolation_ExecDriver_PIDNamespacing(f *framework.F) {
	t := f.T()

	clientNodes, err := e2eutil.ListLinuxClientNodes(tc.Nomad())
	require.Nil(t, err)

	if len(clientNodes) == 0 {
		t.Skip("no Linux clients")
	}

	jobID := "isolation-pid-namespace-" + uuid.Short()
	file := "isolation/input/exec.nomad"
	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), file, jobID, "")
	require.Equal(t, len(allocs), 1, fmt.Sprintf("failed to register %s", jobID))

	tc.jobIDs = append(tc.jobIDs, jobID)
	defer func() {
		_, _, err = tc.Nomad().Jobs().Deregister(jobID, true, nil)
		require.NoError(t, err)
	}()

	allocID := allocs[0].ID
	e2eutil.WaitForAllocStopped(t, tc.Nomad(), allocID)

	out, err := e2eutil.AllocLogs(allocID, e2eutil.LogsStdOut)
	require.NoError(t, err, fmt.Sprintf("could not get logs for alloc %s", allocID))

	require.Contains(t, out, "my pid is 1\n")
}

func (tc *PIDsNamespacing) TestIsolation_ExecDriver_PIDNamespacing_host(f *framework.F) {
	t := f.T()

	clientNodes, err := e2eutil.ListLinuxClientNodes(tc.Nomad())
	require.Nil(t, err)

	if len(clientNodes) == 0 {
		t.Skip("no Linux clients")
	}

	jobID := "isolation-pid-namespace-" + uuid.Short()
	file := "isolation/input/exec_host.nomad"
	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), file, jobID, "")
	require.Equal(t, len(allocs), 1, fmt.Sprintf("failed to register %s", jobID))

	tc.jobIDs = append(tc.jobIDs, jobID)
	defer func() {
		_, _, err = tc.Nomad().Jobs().Deregister(jobID, true, nil)
		require.NoError(t, err)
	}()

	allocID := allocs[0].ID
	e2eutil.WaitForAllocStopped(t, tc.Nomad(), allocID)

	out, err := e2eutil.AllocLogs(allocID, e2eutil.LogsStdOut)
	require.NoError(t, err, fmt.Sprintf("could not get logs for alloc %s", allocID))

	require.NotContains(t, out, "my pid is 1\n")
}

func (tc *PIDsNamespacing) TestIsolation_ExecDriver_PIDNamespacing_AllocExec(f *framework.F) {
	t := f.T()

	clientNodes, err := e2eutil.ListLinuxClientNodes(tc.Nomad())
	require.Nil(t, err)

	if len(clientNodes) == 0 {
		t.Skip("no Linux clients")
	}

	jobID := "isolation-pid-namespace-" + uuid.Short()
	file := "isolation/input/alloc_exec.nomad"
	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), file, jobID, "")
	require.Equal(t, len(allocs), 1, fmt.Sprintf("failed to register %s", jobID))

	defer func() {
		_, _, err = tc.Nomad().Jobs().Deregister(jobID, true, nil)
		require.NoError(t, err)
	}()

	allocID := allocs[0].ID
	e2eutil.WaitForAllocRunning(t, tc.Nomad(), allocID)

	alloc, _, err := tc.Nomad().Allocations().Info(allocID, nil)
	require.NoError(t, err)
	require.NotNil(t, alloc)

	resizeCh := make(chan api.TerminalSize)
	var tty bool

	ctx, cancelFn := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelFn()

	var stdout, stderr bytes.Buffer

	exitCode, err := tc.Nomad().Allocations().Exec(
		ctx,
		alloc,
		"main",
		tty,
		[]string{"ps", "ax"},
		bytes.NewReader([]byte("")),
		&stdout,
		&stderr,
		resizeCh,
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	// header, sleep process, ps ax process are the only output lines expected
	require.Len(t, lines, 3)
}

func (tc *PIDsNamespacing) TestIsolation_JavaDriver_PIDNamespacing(f *framework.F) {
	t := f.T()

	clientNodes, err := e2eutil.ListLinuxClientNodes(tc.Nomad())
	require.Nil(t, err)

	if len(clientNodes) == 0 {
		t.Skip("no Linux clients")
	}

	jobID := "isolation-pid-namespace-" + uuid.Short()
	file := "isolation/input/java.nomad"
	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), file, jobID, "")
	require.Equal(t, len(allocs), 1, fmt.Sprintf("failed to register %s", jobID))

	tc.jobIDs = append(tc.jobIDs, jobID)
	defer func() {
		_, _, err = tc.Nomad().Jobs().Deregister(jobID, true, nil)
		require.NoError(t, err)
	}()

	allocID := allocs[0].ID
	e2eutil.WaitForAllocStopped(t, tc.Nomad(), allocID)

	out, err := e2eutil.AllocTaskLogs(allocID, "pid", e2eutil.LogsStdOut)
	require.NoError(t, err, fmt.Sprintf("could not get logs for alloc %s", allocID))

	require.Contains(t, out, "my pid is 1\n")
}

func (tc *PIDsNamespacing) TestIsolation_JavaDriver_PIDNamespacing_host(f *framework.F) {
	t := f.T()

	clientNodes, err := e2eutil.ListLinuxClientNodes(tc.Nomad())
	require.Nil(t, err)

	if len(clientNodes) == 0 {
		t.Skip("no Linux clients")
	}

	jobID := "isolation-pid-namespace-" + uuid.Short()
	file := "isolation/input/java_host.nomad"
	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), file, jobID, "")
	require.Equal(t, len(allocs), 1, fmt.Sprintf("failed to register %s", jobID))

	tc.jobIDs = append(tc.jobIDs, jobID)
	defer func() {
		_, _, err = tc.Nomad().Jobs().Deregister(jobID, true, nil)
		require.NoError(t, err)
	}()

	allocID := allocs[0].ID
	e2eutil.WaitForAllocStopped(t, tc.Nomad(), allocID)

	out, err := e2eutil.AllocTaskLogs(allocID, "pid", e2eutil.LogsStdOut)
	require.NoError(t, err, fmt.Sprintf("could not get logs for alloc %s", allocID))

	require.NotContains(t, out, "my pid is 1\n")
}

func (tc *PIDsNamespacing) TestIsolation_JavaDriver_PIDNamespacing_AllocExec(f *framework.F) {
	t := f.T()

	clientNodes, err := e2eutil.ListLinuxClientNodes(tc.Nomad())
	require.Nil(t, err)

	if len(clientNodes) == 0 {
		t.Skip("no Linux clients")
	}

	jobID := "isolation-pid-namespace-" + uuid.Short()
	file := "isolation/input/alloc_exec_java.nomad"
	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), file, jobID, "")
	require.Equal(t, len(allocs), 1, fmt.Sprintf("failed to register %s", jobID))

	defer func() {
		_, _, err = tc.Nomad().Jobs().Deregister(jobID, true, nil)
		require.NoError(t, err)
	}()

	allocID := allocs[0].ID
	e2eutil.WaitForAllocTaskRunning(t, tc.Nomad(), allocID, "sleep")

	alloc, _, err := tc.Nomad().Allocations().Info(allocID, nil)
	require.NoError(t, err)
	require.NotNil(t, alloc)

	resizeCh := make(chan api.TerminalSize)
	var tty bool

	ctx, cancelFn := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelFn()

	var stdout, stderr bytes.Buffer

	exitCode, err := tc.Nomad().Allocations().Exec(
		ctx,
		alloc,
		"sleep",
		tty,
		[]string{"ps", "ax"},
		bytes.NewReader([]byte("")),
		&stdout,
		&stderr,
		resizeCh,
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, 0, exitCode)

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	// header, sleep process, ps ax process are the only output lines expected
	require.Len(t, lines, 3)
}

func (tc *PIDsNamespacing) TestIsolation_RawExecDriver_NoPIDNamespacing(f *framework.F) {
	t := f.T()

	clientNodes, err := e2eutil.ListLinuxClientNodes(tc.Nomad())
	require.Nil(t, err)

	if len(clientNodes) == 0 {
		t.Skip("no Linux clients")
	}

	jobID := "isolation-pid-namespace-" + uuid.Short()
	file := "isolation/input/raw_exec.nomad"

	allocs := e2eutil.RegisterAndWaitForAllocs(t, tc.Nomad(), file, jobID, "")
	require.Equal(t, len(allocs), 1, fmt.Sprintf("failed to register %s", jobID))

	defer func() {
		_, _, err = tc.Nomad().Jobs().Deregister(jobID, true, nil)
		require.NoError(t, err)
	}()

	allocID := allocs[0].ID
	e2eutil.WaitForAllocStopped(t, tc.Nomad(), allocID)

	out, err := e2eutil.AllocLogs(allocID, e2eutil.LogsStdOut)
	require.NoError(t, err, fmt.Sprintf("could not get logs for alloc %s", allocID))

	var pid uint64
	_, err = fmt.Sscanf(out, "my pid is %d", &pid)
	require.NoError(t, err)

	require.Greater(t, pid, uint64(1))
}
