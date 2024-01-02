// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/posener/complete"
)

const (
	HealthCritical = 2
	HealthWarn     = 1
	HealthPass     = 0
	HealthUnknown  = 3
)

type AgentCheckCommand struct {
	Meta
}

func (c *AgentCheckCommand) Help() string {
	helpText := `
Usage: nomad check [options]

  Display state of the Nomad agent. The exit code of the command is Nagios
  compatible and could be used with alerting systems.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Agent Check Options:

  -min-peers
     Minimum number of peers that a server is expected to know.

  -min-servers
     Minimum number of servers that a client is expected to know.
`

	return strings.TrimSpace(helpText)
}

func (c *AgentCheckCommand) Synopsis() string {
	return "Displays health of the local Nomad agent"
}

func (c *AgentCheckCommand) Name() string { return "check" }

func (c *AgentCheckCommand) Run(args []string) int {
	var minPeers, minServers int

	flags := c.Meta.FlagSet("check", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.IntVar(&minPeers, "min-peers", 0, "")
	flags.IntVar(&minServers, "min-servers", 1, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	args = flags.Args()
	if len(args) > 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error initializing client: %s", err))
		return HealthCritical
	}

	info, err := client.Agent().Self()
	if err != nil {
		c.Ui.Output(fmt.Sprintf("unable to query agent info: %v", err))
		return HealthCritical
	}
	if _, ok := info.Stats["nomad"]; ok {
		return c.checkServerHealth(info.Stats, minPeers)
	}

	if clientStats, ok := info.Stats["client"]; ok {
		return c.checkClientHealth(clientStats, minServers)
	}
	return HealthWarn
}

// checkServerHealth returns the health of a server.
// TODO Add more rules for determining server health
func (c *AgentCheckCommand) checkServerHealth(info map[string]map[string]string, minPeers int) int {
	raft := info["raft"]
	knownPeers, err := strconv.Atoi(raft["num_peers"])
	if err != nil {
		c.Ui.Output(fmt.Sprintf("unable to get known peers: %v", err))
		return HealthCritical
	}

	if knownPeers < minPeers {
		c.Ui.Output(fmt.Sprintf("known peers: %v, is less than expected number of peers: %v", knownPeers, minPeers))
		return HealthCritical
	}
	return HealthPass
}

// checkClientHealth returns the health of a client
func (c *AgentCheckCommand) checkClientHealth(clientStats map[string]string, minServers int) int {
	knownServers, err := strconv.Atoi(clientStats["known_servers"])
	if err != nil {
		c.Ui.Output(fmt.Sprintf("unable to get known servers: %v", err))
		return HealthCritical
	}

	heartbeatTTL, err := time.ParseDuration(clientStats["heartbeat_ttl"])
	if err != nil {
		c.Ui.Output(fmt.Sprintf("unable to parse heartbeat TTL: %v", err))
		return HealthCritical
	}

	lastHeartbeat, err := time.ParseDuration(clientStats["last_heartbeat"])
	if err != nil {
		c.Ui.Output(fmt.Sprintf("unable to parse last heartbeat: %v", err))
		return HealthCritical
	}

	if lastHeartbeat > heartbeatTTL {
		c.Ui.Output(fmt.Sprintf("last heartbeat was %q time ago, expected heartbeat ttl: %q", lastHeartbeat, heartbeatTTL))
		return HealthCritical
	}

	if knownServers < minServers {
		c.Ui.Output(fmt.Sprintf("known servers: %v, is less than expected number of servers: %v", knownServers, minServers))
		return HealthCritical
	}

	return HealthPass
}

func (c *AgentCheckCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-min-peers":   complete.PredictAnything,
			"-min-servers": complete.PredictAnything,
		})
}

func (c *AgentCheckCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}
