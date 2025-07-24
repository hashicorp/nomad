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
	"github.com/posener/complete"
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

Use the 'nomad monitor export' command to export an agent's historic data
from journald or its Nomad log file. If exporting journald logs, you must
pass '-service-name' with the name of the nomad service.
The '-logs-since' and '-follow' options are only valid for journald queries.
You may pass a duration string to the '-logs-since' option to override the
default 72h duration. Nomad will accept the following time units in the
'-logs-since' duration string:"ns", "us" (or "µs"), "ms", "s", "m", "h".
The '-follow=true' option causes the agent to continue to stream logs until
interrupted or until the remote agent quits. Nomad only supports journald
queries on Linux.

If you do not use Linux or you do not run Nomad as a systemd unit, pass the
'-on-disk=true' option to export the entirety of a given agent's nomad log file.

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
    Sets the name of the nomad service, must match systemd conventions and
	include the word 'nomad'. You may provide the full systemd file name
	or omit the suffix. If your service name includes a '.', you must include
	a valid suffix (e.g. nomad.client.service).

  -log-since <duration string>
    Sets the journald log period, invalid if on-disk=true. Defaults to 72h.
	Valid unit strings are "ns", "us" (or "µs"), "ms", "s", "m", "h".

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

func (c *MonitorExportCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-node-id":      NodePredictor(c.Client),
			"-server-id":    ServerPredictor(c.Client),
			"-service-name": complete.PredictSet("nomad"),
			"-log-since":    complete.PredictNothing,
			"-follow":       complete.PredictNothing,
			"-on-disk":      complete.PredictNothing,
		})
}

func (c *MonitorExportCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *MonitorExportCommand) Name() string { return "monitor export" }

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
	flags.DurationVar(&c.logSince, "logs-since", defaultDur,
		`sets the journald	log period.  Defaults to 72h, valid unit strings are
		 "ns", "us" (or "µs"), "ms", "s", "m", or "h".`)
	flags.StringVar(&c.serviceName, "service-name", "",
		"the name of the systemdervice unit to collect logs for, cannot be used with on-disk=true")
	flags.BoolVar(&c.onDisk, "on-disk", false,
		"directs the cli to stream the configured nomad log file, cannot be used with -service-name")
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

	if c.serviceName != "" && c.onDisk {
		c.Ui.Error("Cannot target journalctl and nomad log file simultaneously")
		c.Ui.Error(commandErrorText(c))
	}

	if c.serviceName != "" {
		if isNomad := strings.Contains(c.serviceName, "nomad"); !isNomad {
			c.Ui.Error(fmt.Sprintf("Invalid value: -service-name=%s does not include 'nomad'", c.serviceName))
			c.Ui.Error(commandErrorText(c))
		}
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
		c.Ui.Error(fmt.Sprintf("Error starting monitor: \n%s", err))
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	n, err := io.Copy(os.Stdout, r)
	if err != nil && err != io.EOF {
		c.Ui.Error(fmt.Sprintf("Error monitoring logs: %s", err.Error()))
		return 1
	}

	if n == 0 && err == nil {
		emptyMessage := "Returned no data or errors, check your log_file configuration or service name"
		c.Ui.Error(fmt.Sprintf("Error starting monitor: \n%s", emptyMessage))
		return 1
	}
	return 0
}
