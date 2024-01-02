// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	flaghelper "github.com/hashicorp/nomad/helper/flags"
	"github.com/posener/complete"
)

type OperatorAutopilotSetCommand struct {
	Meta
}

func (c *OperatorAutopilotSetCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-cleanup-dead-servers":      complete.PredictAnything,
			"-max-trailing-logs":         complete.PredictAnything,
			"-last-contact-threshold":    complete.PredictAnything,
			"-server-stabilization-time": complete.PredictAnything,
			"-enable-redundancy-zones":   complete.PredictNothing,
			"-disable-upgrade-migration": complete.PredictNothing,
			"-enable-custom-upgrades":    complete.PredictNothing,
		})
}

func (c *OperatorAutopilotSetCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *OperatorAutopilotSetCommand) Name() string { return "operator autopilot set-config" }

func (c *OperatorAutopilotSetCommand) Run(args []string) int {
	// Autopilot command line flags behave differently from other commands
	// in Nomad. Here, flags assume no default value. The value of the flag
	// is taken into consideration if the flag is set, whether or not it contains
	// the zero value when being applied to inherited configuration.
	//
	// This behavior was inherited from Consul.
	var cleanupDeadServers flaghelper.BoolValue
	var maxTrailingLogs flaghelper.UintValue
	var minQuorum flaghelper.UintValue
	var lastContactThreshold flaghelper.DurationValue
	var serverStabilizationTime flaghelper.DurationValue
	var enableRedundancyZones flaghelper.BoolValue
	var disableUpgradeMigration flaghelper.BoolValue
	var enableCustomUpgrades flaghelper.BoolValue

	flagSet := c.Meta.FlagSet("autopilot", FlagSetClient)
	flagSet.Usage = func() { c.Ui.Output(c.Help()) }

	flagSet.Var(&cleanupDeadServers, "cleanup-dead-servers", "")
	flagSet.Var(&maxTrailingLogs, "max-trailing-logs", "")
	flagSet.Var(&lastContactThreshold, "last-contact-threshold", "")
	flagSet.Var(&serverStabilizationTime, "server-stabilization-time", "")
	flagSet.Var(&enableRedundancyZones, "enable-redundancy-zones", "")
	flagSet.Var(&disableUpgradeMigration, "disable-upgrade-migration", "")
	flagSet.Var(&enableCustomUpgrades, "enable-custom-upgrades", "")
	flagSet.Var(&minQuorum, "min-quorum", "")

	if err := flagSet.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	// Set up a client.
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Fetch the current configuration.
	operator := client.Operator()
	conf, _, err := operator.AutopilotGetConfiguration(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying for Autopilot configuration: %s", err))
		return 1
	}

	// Update the config values based on the set flags.
	cleanupDeadServers.Merge(&conf.CleanupDeadServers)
	enableRedundancyZones.Merge(&conf.EnableRedundancyZones)
	disableUpgradeMigration.Merge(&conf.DisableUpgradeMigration)
	enableCustomUpgrades.Merge(&conf.EnableCustomUpgrades)

	trailing := uint(conf.MaxTrailingLogs)
	maxTrailingLogs.Merge(&trailing)
	conf.MaxTrailingLogs = uint64(trailing)

	minQuorum.Merge(&conf.MinQuorum)

	lastContactThreshold.Merge(&conf.LastContactThreshold)

	serverStabilizationTime.Merge(&conf.ServerStabilizationTime)

	// Check-and-set the new configuration.
	result, _, err := operator.AutopilotCASConfiguration(conf, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error setting Autopilot configuration: %s", err))
		return 1
	}
	if result {
		c.Ui.Output("Configuration updated!")
		return 0
	}
	c.Ui.Output("Configuration could not be atomically updated, please try again")
	return 1
}

func (c *OperatorAutopilotSetCommand) Synopsis() string {
	return "Modify the current Autopilot configuration"
}

func (c *OperatorAutopilotSetCommand) Help() string {
	helpText := `
Usage: nomad operator autopilot set-config [options]

  Modifies the current Autopilot configuration.

  If ACLs are enabled, this command requires a token with the 'operator:write'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Set Config Options:

  -cleanup-dead-servers=[true|false]
     Controls whether Nomad will automatically remove dead servers when
     new ones are successfully added. Must be one of [true|false].

  -disable-upgrade-migration=[true|false]
     (Enterprise-only) Controls whether Nomad will avoid promoting
     new servers until it can perform a migration. Must be one of
     "true|false".

  -last-contact-threshold=200ms
     Controls the maximum amount of time a server can go without contact
     from the leader before being considered unhealthy. Must be a
     duration value such as "200ms".

  -max-trailing-logs=<value>
     Controls the maximum number of log entries that a server can trail
     the leader by before being considered unhealthy.

  -min-quorum=<value>
      Controls the minimum number of servers required in a cluster
      before autopilot can prune dead servers.

  -redundancy-zone-tag=<value>
     (Enterprise-only) Controls the node_meta tag name used for
     separating servers into different redundancy zones.

  -server-stabilization-time=<10s>
     Controls the minimum amount of time a server must be stable in
     the 'healthy' state before being added to the cluster. Only takes
     effect if all servers are running Raft protocol version 3 or
     higher. Must be a duration value such as "10s".

  -upgrade-version-tag=<value>
     (Enterprise-only) The node_meta tag to use for version info when
     performing upgrade migrations. If left blank, the Nomad version
     will be used.
`
	return strings.TrimSpace(helpText)
}
