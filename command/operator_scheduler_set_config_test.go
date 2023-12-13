// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strconv"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestOperatorSchedulerSetConfig_Run(t *testing.T) {
	ci.Parallel(t)

	srv, _, addr := testServer(t, false, nil)
	defer srv.Shutdown()

	ui := cli.NewMockUi()
	c := &OperatorSchedulerSetConfig{Meta: Meta{Ui: ui}}

	bootstrappedConfig, _, err := srv.Client().Operator().SchedulerGetConfiguration(nil)
	require.NoError(t, err)
	require.NotEmpty(t, bootstrappedConfig.SchedulerConfig)

	// Run the command with zero value and ensure the configuration does not
	// change.
	require.EqualValues(t, 0, c.Run([]string{"-address=" + addr}))
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()

	// Read the configuration again and test that nothing has changed which
	// ensures our empty flags are working correctly.
	nonModifiedConfig, _, err := srv.Client().Operator().SchedulerGetConfiguration(nil)
	require.NoError(t, err)
	schedulerConfigEquals(t, bootstrappedConfig.SchedulerConfig, nonModifiedConfig.SchedulerConfig)

	// Modify every configuration parameter using the flags. This ensures the
	// merging is working correctly and that operators can control the entire
	// object via the CLI.
	modifyingArgs := []string{
		"-address=" + addr,
		"-scheduler-algorithm=spread",
		"-pause-eval-broker=true",
		"-memory-oversubscription=true",
		"-reject-job-registration=true",
		"-preempt-batch-scheduler=true",
		"-preempt-service-scheduler=true",
		"-preempt-sysbatch-scheduler=true",
		"-preempt-system-scheduler=false",
	}
	require.EqualValues(t, 0, c.Run(modifyingArgs))
	s := ui.OutputWriter.String()
	require.Contains(t, s, "Scheduler configuration updated!")

	modifiedConfig, _, err := srv.Client().Operator().SchedulerGetConfiguration(nil)
	require.NoError(t, err)
	schedulerConfigEquals(t, &api.SchedulerConfiguration{
		SchedulerAlgorithm: "spread",
		PreemptionConfig: api.PreemptionConfig{
			SystemSchedulerEnabled:   false,
			SysBatchSchedulerEnabled: true,
			BatchSchedulerEnabled:    true,
			ServiceSchedulerEnabled:  true,
		},
		MemoryOversubscriptionEnabled: true,
		RejectJobRegistration:         true,
		PauseEvalBroker:               true,
	}, modifiedConfig.SchedulerConfig)

	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()

	// Make a Freudian slip with one of the flags to ensure the usage is
	// returned.
	require.EqualValues(t, 1, c.Run([]string{"-address=" + addr, "-pause-evil-broker=true"}))
	require.Contains(t, ui.OutputWriter.String(), "Usage: nomad operator scheduler set-config")
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()

	// Try updating the config using an incorrect check-index value.
	require.EqualValues(t, 1, c.Run([]string{
		"-address=" + addr,
		"-pause-eval-broker=false",
		"-check-index=1000000",
	}))
	require.Contains(t, ui.ErrorWriter.String(), "check-index 1000000 does not match does not match current state")
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()

	// Try updating the config using a correct check-index value.
	require.EqualValues(t, 0, c.Run([]string{
		"-address=" + addr,
		"-pause-eval-broker=false",
		"-check-index=" + strconv.FormatUint(modifiedConfig.SchedulerConfig.ModifyIndex, 10),
	}))
	require.Contains(t, ui.OutputWriter.String(), "Scheduler configuration updated!")
	ui.ErrorWriter.Reset()
	ui.OutputWriter.Reset()
}

func schedulerConfigEquals(t *testing.T, expected, actual *api.SchedulerConfiguration) {
	require.Equal(t, expected.SchedulerAlgorithm, actual.SchedulerAlgorithm)
	require.Equal(t, expected.RejectJobRegistration, actual.RejectJobRegistration)
	require.Equal(t, expected.MemoryOversubscriptionEnabled, actual.MemoryOversubscriptionEnabled)
	require.Equal(t, expected.PauseEvalBroker, actual.PauseEvalBroker)
	require.Equal(t, expected.PreemptionConfig, actual.PreemptionConfig)
}
