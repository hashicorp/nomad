// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	"github.com/mitchellh/cli"
	"github.com/shoenig/test/must"
)

func TestOperator_Autopilot_SetConfig_Implements(t *testing.T) {
	ci.Parallel(t)
	var _ cli.Command = &OperatorRaftListCommand{}
}

func TestOperatorAutopilotSetConfigCommand(t *testing.T) {
	ci.Parallel(t)

	s, _, addr := testServer(t, false, nil)
	defer s.Shutdown()

	ui := cli.NewMockUi()
	c := &OperatorAutopilotSetCommand{Meta: Meta{Ui: ui}}
	args := []string{
		"-address=" + addr,
		"-cleanup-dead-servers=false",
		"-max-trailing-logs=99",
		"-min-quorum=3",
		"-last-contact-threshold=123ms",
		"-server-stabilization-time=123ms",
		"-enable-redundancy-zones=true",
		"-disable-upgrade-migration=true",
		"-enable-custom-upgrades=true",
	}

	code := c.Run(args)
	must.Zero(t, code)

	output := strings.TrimSpace(ui.OutputWriter.String())
	must.StrContains(t, output, "Configuration updated")

	client, err := c.Client()
	must.NoError(t, err)

	conf, _, err := client.Operator().AutopilotGetConfiguration(nil)
	must.NoError(t, err)

	must.False(t, conf.CleanupDeadServers)
	must.Eq(t, 99, conf.MaxTrailingLogs)
	must.Eq(t, 3, conf.MinQuorum)
	must.Eq(t, 123*time.Millisecond, conf.LastContactThreshold)
	must.Eq(t, 123*time.Millisecond, conf.ServerStabilizationTime)
	must.True(t, conf.EnableRedundancyZones)
	must.True(t, conf.DisableUpgradeMigration)
	must.True(t, conf.EnableCustomUpgrades)
}
