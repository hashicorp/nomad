package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllocSignalCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &AllocSignalCommand{}
}

func TestAllocSignalCommand_Fails(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	require := require.New(t)

	ui := new(cli.MockUi)
	cmd := &AllocSignalCommand{Meta: Meta{Ui: ui}}

	// Fails on lack of alloc ID
	require.Equal(1, cmd.Run([]string{}))
	require.Contains(ui.ErrorWriter.String(), "This command takes up to two arguments")
	ui.ErrorWriter.Reset()

	// Fails on misuse
	require.Equal(1, cmd.Run([]string{"some", "bad", "args"}))
	require.Contains(ui.ErrorWriter.String(), "This command takes up to two arguments")
	ui.ErrorWriter.Reset()

	// Fails on connection failure
	require.Equal(1, cmd.Run([]string{"-address=nope", "foobar"}))
	require.Contains(ui.ErrorWriter.String(), "Error querying allocation")
	ui.ErrorWriter.Reset()

	// Fails on missing alloc
	code := cmd.Run([]string{"-address=" + url, "26470238-5CF2-438F-8772-DC67CFB0705C"})
	require.Equal(1, code)
	require.Contains(ui.ErrorWriter.String(), "No allocation(s) with prefix or id")
	ui.ErrorWriter.Reset()

	// Fail on identifier with too few characters
	require.Equal(1, cmd.Run([]string{"-address=" + url, "2"}))
	require.Contains(ui.ErrorWriter.String(), "must contain at least two characters.")
	ui.ErrorWriter.Reset()
}

func TestAllocSignalCommand_AutocompleteArgs(t *testing.T) {
	assert := assert.New(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &AllocSignalCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake alloc
	state := srv.Agent.Server().State()
	a := mock.Alloc()
	assert.Nil(state.UpsertAllocs(1000, []*structs.Allocation{a}))

	prefix := a.ID[:5]
	args := complete.Args{All: []string{"signal", prefix}, Last: prefix}
	predictor := cmd.AutocompleteArgs()

	// Match Allocs
	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(a.ID, res[0])
}

func TestAllocSignalCommand_Run(t *testing.T) {
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
	cmd := &AllocSignalCommand{Meta: Meta{Ui: ui}}

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
