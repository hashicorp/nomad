// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
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

	// Create an upsert some service registrations.
	serviceRegs := mock.ServiceRegistrations()
	assert.NoError(t,
		srv.Agent.Server().State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 10, serviceRegs))

	// Detail the service within the default namespace as we need the ID.
	defaultNSService, _, err := client.Services().Get(serviceRegs[0].ServiceName, nil)
	require.NoError(t, err)
	require.Len(t, defaultNSService, 1)

	// Attempt to manually delete the service registration within the default
	// namespace.
	code := cmd.Run([]string{"-address=" + url, "service-discovery-nomad-delete", defaultNSService[0].ID})
	require.Equal(t, 0, code)
	require.Contains(t, ui.OutputWriter.String(), "Successfully deleted service registration")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()

	// Detail the service within the platform namespace as we need the ID.
	platformNSService, _, err := client.Services().Get(serviceRegs[1].ServiceName, &api.QueryOptions{
		Namespace: serviceRegs[1].Namespace},
	)
	require.NoError(t, err)
	require.Len(t, platformNSService, 1)

	// Attempt to manually delete the service registration within the platform
	// namespace.
	code = cmd.Run([]string{"-address=" + url, "-namespace=" + platformNSService[0].Namespace,
		"service-discovery-nomad-delete", platformNSService[0].ID})
	require.Equal(t, 0, code)
	require.Contains(t, ui.OutputWriter.String(), "Successfully deleted service registration")

	ui.OutputWriter.Reset()
	ui.ErrorWriter.Reset()
}
