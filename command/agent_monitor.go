// Copyright IBM Corp. 2015, 2025
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type MonitorCommand struct {
	Meta

	// Below this point is where CLI flag options are stored.
	logLevel           string
	nodeID             string
	serverID           string
	logJSON            bool
	logIncludeLocation bool
}

func (c *MonitorCommand) Help() string {
	helpText := `
Usage: nomad monitor [options]

  Stream log messages of a nomad agent. The monitor command lets you
  listen for log levels that may be filtered out of the Nomad agent. For
  example your agent may only be logging at INFO level, but with the monitor
  command you can set -log-level DEBUG

  When ACLs are enabled, this command requires a token with the 'agent:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Monitor Specific Options:

  -log-level <level>
    Sets the log level to monitor (default: INFO)

  -log-include-location
    Include file and line information in each log line. The default is false.

  -node-id <node-id>
    Sets the specific node to monitor

  -server-id <server-id>
    Sets the specific server to monitor

  -json
    Sets log output to JSON format
  `
	return strings.TrimSpace(helpText)
}

func (c *MonitorCommand) Synopsis() string {
	return "Stream logs from a Nomad agent"
}

func (c *MonitorCommand) Name() string { return "monitor" }

func (c *MonitorCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-log-level":            complete.PredictSet("TRACE", "DEBUG", "INFO", "WARN", "ERROR"),
			"-log-include-location": complete.PredictNothing,
			"-node-id": complete.PredictFunc(func(a complete.Args) []string {
				client, err := c.Meta.Client()
				if err != nil {
					return nil
				}
				resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Nodes, nil)
				if err != nil {
					return []string{}
				}
				return resp.Matches[contexts.Nodes]
			}),
			"-server-id": complete.PredictAnything,
			"-json":      complete.PredictNothing,
		})
}

func (c *MonitorCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *MonitorCommand) Run(args []string) int {
	c.Ui = &cli.PrefixedUi{
		OutputPrefix: "    ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           c.Ui,
	}

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&c.logLevel, "log-level", "", "")
	flags.BoolVar(&c.logIncludeLocation, "log-include-location", false, "")
	flags.StringVar(&c.nodeID, "node-id", "", "")
	flags.StringVar(&c.serverID, "server-id", "", "")
	flags.BoolVar(&c.logJSON, "json", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error(uiMessageNoArguments)
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Query the node info and lookup prefix
	if c.nodeID != "" {
		c.nodeID, err = lookupNodeID(client.Nodes(), c.nodeID)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	}

	params := map[string]string{
		"log_level":            c.logLevel,
		"node_id":              c.nodeID,
		"server_id":            c.serverID,
		"log_json":             strconv.FormatBool(c.logJSON),
		"log_include_location": strconv.FormatBool(c.logIncludeLocation),
	}

	query := &api.QueryOptions{
		Params: params,
	}

	eventDoneCh := make(chan struct{})
	frames, errCh := client.Agent().Monitor(eventDoneCh, query)
	r, err := streamFrames(frames, errCh, -1, eventDoneCh)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error starting monitor: %s", err))
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	_, err = io.Copy(os.Stdout, r)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error monitoring logs: %s", err))
		return 1
	}

	return 0
}
