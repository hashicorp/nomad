package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/command/agent"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentStatusCommand_Implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &DeploymentStatusCommand{}
}

func TestDeploymentStatusCommand_Fails(t *testing.T) {
	t.Parallel()
	ui := new(cli.MockUi)
	cmd := &DeploymentStatusCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, 1, code)
	out := ui.ErrorWriter.String()
	require.Contains(t, out, commandErrorText(cmd))
	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope", "12"})
	require.Equal(t, 1, code)
	out = ui.ErrorWriter.String()
	require.Contains(t, out, "Error retrieving deployment")
	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope"})
	require.Equal(t, 1, code)
	out = ui.ErrorWriter.String()
	// "deployments" indicates that we attempted to list all deployments
	require.Contains(t, out, "Error retrieving deployments")
	ui.ErrorWriter.Reset()
}

func TestDeploymentStatusCommand_AutocompleteArgs(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &DeploymentStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake deployment
	state := srv.Agent.Server().State()
	d := mock.Deployment()
	assert.Nil(state.UpsertDeployment(1000, d))

	prefix := d.ID[:5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(d.ID, res[0])
}

func TestDeploymentStatusCommand_Multiregion(t *testing.T) {
	t.Parallel()

	cbe := func(config *agent.Config) {
		config.Region = "east"
		config.Datacenter = "east-1"
	}
	cbw := func(config *agent.Config) {
		config.Region = "west"
		config.Datacenter = "west-1"
	}

	srv, clientEast, url := testServer(t, true, cbe)
	defer srv.Shutdown()

	srv2, clientWest, _ := testServer(t, true, cbw)
	defer srv2.Shutdown()

	// Join with srv1
	addr1 := fmt.Sprintf("127.0.0.1:%d",
		srv.Agent.Server().GetConfig().SerfConfig.MemberlistConfig.BindPort)

	if _, err := srv2.Agent.Server().Join([]string{addr1}); err != nil {
		t.Fatalf("Join err: %v", err)
	}

	// wait for client node
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := clientEast.Nodes().List(nil)
		if err != nil {
			return false, err
		}
		if len(nodes) == 0 {
			return false, fmt.Errorf("missing node")
		}
		if _, ok := nodes[0].Drivers["mock_driver"]; !ok {
			return false, fmt.Errorf("mock_driver not ready")
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	ui := new(cli.MockUi)
	cmd := &DeploymentStatusCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Register multiregion job in east
	jobEast := testMultiRegionJob("job1_sfxx", "east", "east-1")
	resp, _, err := clientEast.Jobs().Register(jobEast, nil)
	require.NoError(t, err)
	if code := waitForSuccess(ui, clientEast, fullId, t, resp.EvalID); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}

	// Register multiregion job in west
	jobWest := testMultiRegionJob("job1_sfxx", "west", "west-1")
	resp2, _, err := clientWest.Jobs().Register(jobWest, &api.WriteOptions{Region: "west"})
	require.NoError(t, err)
	if code := waitForSuccess(ui, clientWest, fullId, t, resp2.EvalID); code != 0 {
		t.Fatalf("status code non zero saw %d", code)
	}

	jobs, _, err := clientEast.Jobs().List(&api.QueryOptions{})
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	deploys, _, err := clientEast.Jobs().Deployments(jobs[0].ID, true, &api.QueryOptions{})
	require.NoError(t, err)
	require.Len(t, deploys, 1)

	// Grab both deployments to verify output
	eastDeploys, _, err := clientEast.Jobs().Deployments(jobs[0].ID, true, &api.QueryOptions{Region: "east"})
	require.NoError(t, err)
	require.Len(t, eastDeploys, 1)

	westDeploys, _, err := clientWest.Jobs().Deployments(jobs[0].ID, true, &api.QueryOptions{Region: "west"})
	require.NoError(t, err)
	require.Len(t, westDeploys, 1)

	// Run command for specific deploy
	if code := cmd.Run([]string{"-region=east", "-address=" + url, deploys[0].ID}); code != 0 {
		t.Fatalf("expected exit 0, got: %d", code)
	}

	// Verify Multi-region Deployment info populated
	out := ui.OutputWriter.String()
	require.Contains(t, out, "Multiregion Deployment")
	require.Contains(t, out, "Region")
	require.Contains(t, out, "ID")
	require.Contains(t, out, "Status")
	require.Contains(t, out, "east")
	require.Contains(t, out, eastDeploys[0].ID[0:7])
	require.Contains(t, out, "west")
	require.Contains(t, out, westDeploys[0].ID[0:7])
	require.Contains(t, out, "running")

	require.NotContains(t, out, "<none>")

}
