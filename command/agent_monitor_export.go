// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
)

type MonitorExportCommand struct {
	Meta

	// Below this point is where CLI flag options are stored.
	nodeID      string
	serverID    string
	onDisk      bool
	logSince    time.Duration
	serviceName string
	follow      bool
}

func (c *MonitorExportCommand) Help() string {
	helpText := `
Usage: nomad monitor export [options]

  Return logs written to disk by a Nomad agent. The monitor export command
  lets you read Nomad logs from either the agent's configured log path or
  journalctl. To export journald logs provide the service-name and how
  far back (in hours) you would like to view logs along with the node or server
  ID. To export an agent's Nomad log file pass 'log-path=true' and the node or
  server ID with no other options.

  When ACLs are enabled, this command requires a token with the 'agent:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Monitor Specific Options:

  -node-id <node-id>
    Sets the specific node to monitor. Accepts only a single node-id and cannot
	be used with server-id.

  -server-id <server-id>
    Sets the specific server to monitor. Accepts only a single server-id and
	cannot be used with node-id.

  -service-name <service-name>
    Sets the systemd unit name to query journalctl. Only available on Linux.

  -log-since <int>
    Sets the log period for journald logs. Defaults to 72 and ignored if
	on-disk=true.

  -follow <bool>
	If set, the export command will continue streaming until interrupted. Ignored
	if on-disk=true.

  -on-disk <bool>
    If set, the export command will retrieve the Nomad log file defined in the
	target agent's log_file configuration.
	`
	return strings.TrimSpace(helpText)
}

func (c *MonitorExportCommand) Synopsis() string {
	return "Stream logs from a Nomad agent"
}

func (c *MonitorExportCommand) Name() string { return "monitor" }

func (c *MonitorExportCommand) Run(args []string) int {
	c.Ui = &cli.PrefixedUi{
		OutputPrefix: "    ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           c.Ui,
	}
	defaultDur := time.Hour * 72

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&c.nodeID, "node-id", "", "")
	flags.StringVar(&c.serverID, "server-id", "", "")
	flags.DurationVar(&c.logSince, "logs-since", defaultDur, "")
	flags.StringVar(&c.serviceName, "service-name", "", "the name of the systemd service unit to collect logs for, defaults to nomad if unset")
	flags.BoolVar(&c.onDisk, "on-disk", false, "use configured nomad log file")
	flags.BoolVar(&c.follow, "follow", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error("This command takes no arguments")
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
		"follow":       strconv.FormatBool(c.follow),
		"log_since":    c.logSince.String(),
		"node_id":      c.nodeID,
		"on_disk":      strconv.FormatBool(c.onDisk),
		"server_id":    c.serverID,
		"service_name": c.serviceName,
	}

	query := &api.QueryOptions{
		Params: params,
	}

	eventDoneCh := make(chan struct{})
	frames, errCh := client.Agent().MonitorExport(eventDoneCh, query)

	r, err := streamFrames(frames, errCh, -1, eventDoneCh)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error starting monitor: %s", err))
		c.Ui.Error(commandErrorText(c))
		return 1
	}
	_, err = io.Copy(os.Stdout, r)
	if err != nil && err != io.EOF {
		c.Ui.Error(fmt.Sprintf("error monitoring logs: %s", err.Error()))
		return 1
	}

	return 0
}
