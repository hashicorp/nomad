// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/cli"
	"github.com/hashicorp/nomad/api"
)

type MonitorExternalCommand struct {
	Meta

	// Below this point is where CLI flag options are stored.
	nodeID      string
	serverID    string
	onDisk      bool
	logSince    int
	serviceName string
	follow      bool
}

func (c *MonitorExternalCommand) Help() string {
	helpText := `
Usage: nomad monitor external [options]

  Return logs written to disk by a nomad agent. The monitor export command
  lets you read nomad logs from either the agent's configured log path or
  journalctl. To export journalctl logs provide the service-name and how
  far back (in hours) you would like to view logs along with the node or server
  ID. To export an agent's nomad log file pass 'log-path=true' and the node or
  server ID with no other options.

  When ACLs are enabled, this command requires a token with the 'agent:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Monitor Specific Options:

  -node-id <node-id>
    Sets the specific node to monitor

  -server-id <server-id>
    Sets the specific server to monitor
  `
	return strings.TrimSpace(helpText)
}

func (c *MonitorExternalCommand) Synopsis() string {
	return "Stream logs from a Nomad agent"
}

func (c *MonitorExternalCommand) Name() string { return "monitor" }

func (c *MonitorExternalCommand) Run(args []string) int {
	c.Ui = &cli.PrefixedUi{
		OutputPrefix: "    ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           c.Ui,
	}

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&c.nodeID, "node-id", "", "")
	flags.StringVar(&c.serverID, "server-id", "", "")
	flags.IntVar(&c.logSince, "logs-since", 72, "")
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
		"node_id":      c.nodeID,
		"server_id":    c.serverID,
		"log_since":    strconv.Itoa(c.logSince),
		"service_name": c.serviceName,
		"on_disk":      strconv.FormatBool(c.onDisk),
		"follow":       strconv.FormatBool(c.follow),
	}

	query := &api.QueryOptions{
		Params: params,
	}

	eventDoneCh := make(chan struct{})
	frames, errCh := client.Agent().MonitorExternal(eventDoneCh, query)
	select {
	case err := <-errCh:
		c.Ui.Error(fmt.Sprintf("Error starting monitor: %s", err))
		c.Ui.Error(commandErrorText(c))
		return 1
	default:
	}

	// Create a reader
	var r io.ReadCloser

	frameReader := api.NewFrameReader(frames, errCh, eventDoneCh)
	frameReader.SetUnblockTime(300 * time.Millisecond)
	r = frameReader

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	//close stream on sigterm
	go func() {
		<-signalCh
		r.Close()
	}()

	_, err = io.Copy(os.Stdout, r)
	if err != nil && err != io.EOF {
		c.Ui.Error(fmt.Sprintf("error monitoring logs: %s", err.Error()))
		return 1
	}

	return 0
}
