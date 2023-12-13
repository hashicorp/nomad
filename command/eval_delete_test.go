// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestEvalDeleteCommand_Run(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		testFn func()
		name   string
	}{
		{
			testFn: func() {

				testServer, client, url := testServer(t, false, nil)
				defer testServer.Shutdown()

				// Create the UI and command.
				ui := cli.NewMockUi()
				cmd := &EvalDeleteCommand{
					Meta: Meta{
						Ui:          ui,
						flagAddress: url,
					},
				}

				// Test basic command input validation.
				require.Equal(t, 1, cmd.Run([]string{"-address=" + url}))
				require.Contains(t, ui.ErrorWriter.String(), "Error validating command args and flags")
				ui.ErrorWriter.Reset()
				ui.OutputWriter.Reset()

				// Try running the command when the eval broker is not paused.
				require.Equal(t, 1, cmd.Run([]string{"-address=" + url, "fa3a8c37-eac3-00c7-3410-5ba3f7318fd8"}))
				require.Contains(t, ui.ErrorWriter.String(), "Eval broker is not paused")
				ui.ErrorWriter.Reset()
				ui.OutputWriter.Reset()

				// Paused the eval broker, then try deleting with an eval that
				// does not exist.
				schedulerConfig, _, err := client.Operator().SchedulerGetConfiguration(nil)
				require.NoError(t, err)
				require.False(t, schedulerConfig.SchedulerConfig.PauseEvalBroker)

				schedulerConfig.SchedulerConfig.PauseEvalBroker = true
				_, _, err = client.Operator().SchedulerSetConfiguration(schedulerConfig.SchedulerConfig, nil)
				require.NoError(t, err)
				require.True(t, schedulerConfig.SchedulerConfig.PauseEvalBroker)

				require.Equal(t, 1, cmd.Run([]string{"-address=" + url, "fa3a8c37-eac3-00c7-3410-5ba3f7318fd8"}))
				require.Contains(t, ui.ErrorWriter.String(), "eval not found")
				ui.ErrorWriter.Reset()
				ui.OutputWriter.Reset()
			},
			name: "failure",
		},
		{
			testFn: func() {

				testServer, client, url := testServer(t, false, nil)
				defer testServer.Shutdown()

				// Create the UI and command.
				ui := cli.NewMockUi()
				cmd := &EvalDeleteCommand{
					Meta: Meta{
						Ui:          ui,
						flagAddress: url,
					},
				}

				// Paused the eval broker.
				schedulerConfig, _, err := client.Operator().SchedulerGetConfiguration(nil)
				require.NoError(t, err)
				require.False(t, schedulerConfig.SchedulerConfig.PauseEvalBroker)

				schedulerConfig.SchedulerConfig.PauseEvalBroker = true
				_, _, err = client.Operator().SchedulerSetConfiguration(schedulerConfig.SchedulerConfig, nil)
				require.NoError(t, err)
				require.True(t, schedulerConfig.SchedulerConfig.PauseEvalBroker)

				// With the eval broker paused, run a job register several times
				// to generate evals that will not be acted on.
				testJob := testJob("eval-delete")

				evalIDs := make([]string, 3)
				for i := 0; i < 3; i++ {
					regResp, _, err := client.Jobs().Register(testJob, nil)
					require.NoError(t, err)
					require.NotNil(t, regResp)
					require.NotEmpty(t, regResp.EvalID)
					evalIDs[i] = regResp.EvalID
				}

				// Ensure we have three evaluations in state.
				evalList, _, err := client.Evaluations().List(nil)
				require.NoError(t, err)
				require.Len(t, evalList, 3)

				// Attempted to delete one eval using the ID.
				require.Equal(t, 0, cmd.Run([]string{"-address=" + url, evalIDs[0]}))
				require.Contains(t, ui.OutputWriter.String(), "Successfully deleted 1 evaluation")
				ui.ErrorWriter.Reset()
				ui.OutputWriter.Reset()

				// We modify the number deleted on each command run, so we
				// need to reset this in order to check the next command
				// output.
				cmd.numDeleted = 0

				// Attempted to delete the remaining two evals using a filter
				// expression.
				expr := fmt.Sprintf("JobID == %q and Status == \"pending\" ", *testJob.Name)
				require.Equal(t, 0, cmd.Run([]string{"-address=" + url, "-filter=" + expr}))
				require.Contains(t, ui.OutputWriter.String(), "Successfully deleted 2 evaluations")
				ui.ErrorWriter.Reset()
				ui.OutputWriter.Reset()

				// Ensure we have zero evaluations in state.
				evalList, _, err = client.Evaluations().List(nil)
				require.NoError(t, err)
				require.Len(t, evalList, 0)
			},
			name: "successful",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.testFn()
		})
	}
}

func TestEvalDeleteCommand_verifyArgsAndFlags(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		inputEvalDeleteCommand *EvalDeleteCommand
		inputArgs              []string
		expectedError          error
		name                   string
	}{
		{
			inputEvalDeleteCommand: &EvalDeleteCommand{
				filter: `Status == "Pending"`,
			},
			inputArgs:     []string{"fa3a8c37-eac3-00c7-3410-5ba3f7318fd8"},
			expectedError: errors.New("evaluation ID or filter flag required"),
			name:          "arg and flags",
		},
		{
			inputEvalDeleteCommand: &EvalDeleteCommand{
				filter: "",
			},
			inputArgs:     []string{},
			expectedError: errors.New("evaluation ID or filter flag required"),
			name:          "no arg or flags",
		},
		{
			inputEvalDeleteCommand: &EvalDeleteCommand{
				filter: "",
			},
			inputArgs:     []string{"fa3a8c37-eac3-00c7-3410-5ba3f7318fd8", "fa3a8c37-eac3-00c7-3410-5ba3f7318fd9"},
			expectedError: errors.New("expected 1 argument, got 2"),
			name:          "multiple args",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualError := tc.inputEvalDeleteCommand.verifyArgsAndFlags(tc.inputArgs)
			require.Equal(t, tc.expectedError, actualError)
		})
	}
}
