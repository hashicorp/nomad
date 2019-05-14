package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestAllocStopCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &AllocStopCommand{}
}

func TestAllocStop_Fails(t *testing.T) {
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	require := require.New(t)
	ui := new(cli.MockUi)
	cmd := &AllocStopCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	require.Equal(cmd.Run([]string{"some", "garbage", "args"}), 1, "Expected failure")
	require.Contains(ui.ErrorWriter.String(), commandErrorText(cmd), "Expected help output")
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	require.Equal(cmd.Run([]string{"-address=nope", "foobar"}), 1, "expected failure")
	require.Contains(ui.ErrorWriter.String(), "Error querying allocation")
	ui.ErrorWriter.Reset()

	// Fails on missing alloc
	require.Equal(cmd.Run([]string{"-address=" + url, "26470238-5CF2-438F-8772-DC67CFB0705C"}), 1)
	require.Contains(ui.ErrorWriter.String(), "No allocation(s) with prefix or id")
	ui.ErrorWriter.Reset()

	// Fail on identifier with too few characters
	require.Equal(cmd.Run([]string{"-address=" + url, "2"}), 1)
	require.Contains(ui.ErrorWriter.String(), "must contain at least two characters")
	ui.ErrorWriter.Reset()

	// Identifiers with uneven length should produce a query result
	require.Equal(cmd.Run([]string{"-address=" + url, "123"}), 1)
	require.Contains(ui.ErrorWriter.String(), "No allocation(s) with prefix or id")
	ui.ErrorWriter.Reset()
}

func TestAllocStop_Run(t *testing.T) {
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	require := require.New(t)

	// Wait for a node to be ready
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		for _, node := range nodes {
			if _, ok := node.Drivers["mock_driver"]; ok &&
				node.Status == structs.NodeStatusReady {
				return true, nil
			}
		}
		return false, fmt.Errorf("no ready nodes")
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	ui := new(cli.MockUi)
	cmd := &AllocStopCommand{Meta: Meta{Ui: ui}}

	jobID := "job1_sfx"
	job1 := testJob(jobID)
	resp, _, err := client.Jobs().Register(job1, nil)
	require.NoError(err)
	if code := waitForSuccess(ui, client, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}
	// get an alloc id
	allocId1 := ""
	if allocs, _, err := client.Jobs().Allocations(jobID, false, nil); err == nil {
		if len(allocs) > 0 {
			allocId1 = allocs[0].ID
		}
	}
	require.NotEmpty(allocId1, "unable to find allocation")

	// Wait for alloc to be running
	testutil.WaitForResult(func() (bool, error) {
		alloc, _, err := client.Allocations().Info(allocId1, nil)
		if err != nil {
			return false, err
		}
		if alloc.ClientStatus == api.AllocClientStatusRunning {
			return true, nil
		}
		return false, fmt.Errorf("alloc is not running, is: %s", alloc.ClientStatus)
	}, func(err error) {
		t.Fatalf("err: %v", err)
	})

	require.Equal(cmd.Run([]string{"-address=" + url, allocId1}), 0, "expected successful exit code")

	ui.OutputWriter.Reset()
}
