package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestServiceDeleteCommand_Run(t *testing.T) {
	ci.Parallel(t)

	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait for server and client to start
	testutil.WaitForLeader(t, srv.Agent.RPC)
	clientID := srv.Agent.Client().NodeID()
	testutil.WaitForClient(t, srv.Agent.Client().RPC, clientID, srv.Agent.Client().Region())

	// Wait until our test node is ready.
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
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
		require.NoError(t, err)
	})

	ui := cli.NewMockUi()
	cmd := &ServiceDeleteCommand{
		Meta: Meta{
			Ui:          ui,
			flagAddress: url,
		},
	}

	// Run the command without any arguments to ensure we are performing this
	// check.
	require.Equal(t, 1, cmd.Run([]string{"-address=" + url}))
	require.Contains(t, ui.ErrorWriter.String(),
		"This command takes two arguments: <service_name> and <service_id>")
	ui.ErrorWriter.Reset()

	// Create a test job with a Nomad service.
	testJob := testJob("service-discovery-nomad-delete")
	testJob.TaskGroups[0].Tasks[0].Services = []*api.Service{
		{Name: "service-discovery-nomad-delete", Provider: "nomad"}}

	// Register that job.
	regResp, _, err := client.Jobs().Register(testJob, nil)
	require.NoError(t, err)
	registerCode := waitForSuccess(ui, client, fullId, t, regResp.EvalID)
	require.Equal(t, 0, registerCode)

	// Detail the service as we need the ID.
	serviceList, _, err := client.Services().Get("service-discovery-nomad-delete", nil)
	require.NoError(t, err)
	require.Len(t, serviceList, 1)

	// Attempt to manually delete the service registration.
	code := cmd.Run([]string{"-address=" + url, "service-discovery-nomad-delete", serviceList[0].ID})
	require.Equal(t, 0, code)
	require.Contains(t, ui.OutputWriter.String(), "Successfully deleted service registration")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
