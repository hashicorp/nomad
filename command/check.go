package command

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	HealthCritical = 1
	HealthWarn     = 2
	HealthPass     = 0
)

type AgentCheckCommand struct {
	Meta
}

func (c *AgentCheckCommand) Help() string {
	helpText := `
Usage: nomad check
  
  Display state of the Nomad agent. The exit code of the command is Nagios
  compatible and could be used with alerting systems.

General Options:

  ` + generalOptionsUsage() + `

Agent Check Options:
  
  -min-peers
     Minimum number of peers that a server is expected to know.
`

	return strings.TrimSpace(helpText)
}

func (c *AgentCheckCommand) Synopsis() string {
	return "Displays health of the local Nomad agent"
}

func (c *AgentCheckCommand) Run(args []string) int {
	var minPeers int

	flags := c.Meta.FlagSet("check", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.IntVar(&minPeers, "min-peers", 0, "")

	if err := flags.Parse(args); err != nil {
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
	if _, ok := info["stats"]["nomad"]; ok {
		return c.checkServerHealth(info["stats"], minPeers)
	}

	if _, ok := info["client"]; ok {
		return c.checkClientHealth(info)
	}
	return HealthWarn
}

// checkServerHealth returns the health of a server
func (c *AgentCheckCommand) checkServerHealth(info map[string]interface{}, minPeers int) int {
	raft := info["raft"].(map[string]interface{})
	knownPeers, err := strconv.Atoi(raft["num_peers"].(string))
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

func (c *AgentCheckCommand) checkClientHealth(info map[string]map[string]interface{}) int {
	return HealthPass
}
