package command

import (
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
)

func TestJobPeriodicForceCommand_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &JobPeriodicForceCommand{}
}

func TestJobPeriodicForceCommand_Fails(t *testing.T) {
	ci.Parallel(t)
	ui := cli.NewMockUi()
	cmd := &JobPeriodicForceCommand{Meta: Meta{Ui: ui}}

	// Fails on misuse
	code := cmd.Run([]string{"some", "bad", "args"})
	require.Equal(t, code, 1, "expected error")
	out := ui.ErrorWriter.String()
	require.Contains(t, out, commandErrorText(cmd), "expected help output")
	ui.ErrorWriter.Reset()

	code = cmd.Run([]string{"-address=nope", "12"})
	require.Equal(t, code, 1, "expected error")
	out = ui.ErrorWriter.String()
	require.Contains(t, out, "Error forcing periodic job", "expected force error")
}

func TestJobPeriodicForceCommand_AutocompleteArgs(t *testing.T) {
	ci.Parallel(t)

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	cmd := &JobPeriodicForceCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake job, not periodic
	state := srv.Agent.Server().State()
	j := mock.Job()
	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, j))

	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(complete.Args{Last: j.ID[:len(j.ID)-5]})
	require.Empty(t, res)

	// Create another fake job, periodic
	state = srv.Agent.Server().State()
	j2 := mock.Job()
	j2.Periodic = &structs.PeriodicConfig{
		Enabled:         true,
		Spec:            "spec",
		SpecType:        "cron",
		ProhibitOverlap: true,
		TimeZone:        "test zone",
	}
	require.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 1000, j2))

	res = predictor.Predict(complete.Args{Last: j2.ID[:len(j.ID)-5]})
	require.Equal(t, []string{j2.ID}, res)

	res = predictor.Predict(complete.Args{})
	require.Equal(t, []string{j2.ID}, res)
}

func TestJobPeriodicForceCommand_NonPeriodicJob(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()
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

	// Register a job
	j := testJob("job_not_periodic")

	ui := cli.NewMockUi()
	cmd := &JobPeriodicForceCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	resp, _, err := client.Jobs().Register(j, nil)
	require.NoError(t, err)
	code := waitForSuccess(ui, client, fullId, t, resp.EvalID)
	require.Equal(t, 0, code)

	code = cmd.Run([]string{"-address=" + url, "job_not_periodic"})
	require.Equal(t, 1, code, "expected exit code")
	out := ui.ErrorWriter.String()
	require.Contains(t, out, "No periodic job(s)", "non-periodic error message")
}

func TestJobPeriodicForceCommand_SuccessfulPeriodicForceDetach(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()
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

	// Register a job
	j := testJob("job1_is_periodic")
	j.Periodic = &api.PeriodicConfig{
		SpecType:        pointer.Of(api.PeriodicSpecCron),
		Spec:            pointer.Of("*/15 * * * * *"),
		ProhibitOverlap: pointer.Of(true),
		TimeZone:        pointer.Of("Europe/Minsk"),
	}

	ui := cli.NewMockUi()
	cmd := &JobPeriodicForceCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	_, _, err := client.Jobs().Register(j, nil)
	require.NoError(t, err)

	code := cmd.Run([]string{"-address=" + url, "-detach", "job1_is_periodic"})
	require.Equal(t, 0, code, "expected no error code")
	out := ui.OutputWriter.String()
	require.Contains(t, out, "Force periodic successful")
	require.Contains(t, out, "Evaluation ID:")
}

func TestJobPeriodicForceCommand_SuccessfulPeriodicForce(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()
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

	// Register a job
	j := testJob("job2_is_periodic")
	j.Periodic = &api.PeriodicConfig{
		SpecType:        pointer.Of(api.PeriodicSpecCron),
		Spec:            pointer.Of("*/15 * * * * *"),
		ProhibitOverlap: pointer.Of(true),
		TimeZone:        pointer.Of("Europe/Minsk"),
	}

	ui := cli.NewMockUi()
	cmd := &JobPeriodicForceCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	_, _, err := client.Jobs().Register(j, nil)
	require.NoError(t, err)

	code := cmd.Run([]string{"-address=" + url, "job2_is_periodic"})
	require.Equal(t, 0, code, "expected no error code")
	out := ui.OutputWriter.String()
	require.Contains(t, out, "Monitoring evaluation")
	require.Contains(t, out, "finished with status \"complete\"")
}

func TestJobPeriodicForceCommand_SuccessfulIfJobIDEqualsPrefix(t *testing.T) {
	ci.Parallel(t)
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()
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

	j1 := testJob("periodic-prefix")
	j1.Periodic = &api.PeriodicConfig{
		SpecType:        pointer.Of(api.PeriodicSpecCron),
		Spec:            pointer.Of("*/15 * * * * *"),
		ProhibitOverlap: pointer.Of(true),
		TimeZone:        pointer.Of("Europe/Minsk"),
	}
	j2 := testJob("periodic-prefix-another-job")
	j2.Periodic = &api.PeriodicConfig{
		SpecType:        pointer.Of(api.PeriodicSpecCron),
		Spec:            pointer.Of("*/15 * * * * *"),
		ProhibitOverlap: pointer.Of(true),
		TimeZone:        pointer.Of("Europe/Minsk"),
	}

	ui := cli.NewMockUi()
	cmd := &JobPeriodicForceCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	_, _, err := client.Jobs().Register(j1, nil)
	require.NoError(t, err)
	_, _, err = client.Jobs().Register(j2, nil)
	require.NoError(t, err)

	code := cmd.Run([]string{"-address=" + url, "periodic-prefix"})
	require.Equal(t, 0, code, "expected no error code")
	out := ui.OutputWriter.String()
	require.Contains(t, out, "Monitoring evaluation")
	require.Contains(t, out, "finished with status \"complete\"")
}
