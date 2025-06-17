// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"runtime"
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
	logPath     string
	logSince    int
	serviceName string
	follow      bool
}

func (c *MonitorExternalCommand) Help() string {
	helpText := `
Usage: nomad monitor external [options]

  Stream log messages of a nomad agent. The monitor command lets you
  listen for log levels that may be filtered out of the Nomad agent. For
  example your agent may only be logging at INFO level, but with the monitor
  command you can set -log-level DEBUG

  When ACLs are enabled, this command requires a token with the 'agent:read'
  capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault|usageOptsNoNamespace) + `

Monitor Specific Options:

  -node-id <node-id>
    Sets the specific node to monitor

  -server-id <server-id>
    Sets the specific server to monitor

  -json
    Sets log output to JSON format
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
	flags.StringVar(&c.serviceName, "service", "", "the name of the systemd service unit to collect logs for, defaults to nomad if unset")
	flags.StringVar(&c.logPath, "log-path", "", "full path to the desired log file")
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
	scanServiceName := func(input string) error {
		// Trim leading and trailing spaces.
		input = strings.TrimSpace(input)

		// Create a regex pattern to exclude unwanted characters.
		re := regexp.MustCompile(`[!@#\$%^&~*()\x60+=\[\]{};':"\\|.,<>\/?]`)

		unsafe := re.MatchString(input)
		if unsafe {
			return errors.New("disallowed character detected. Option 'service' may only contain '-', '_', and alphanumeric characters.")
		}
		return nil
	}

	// Check serviceName for unsafe characters
	err = scanServiceName(c.serviceName)
	if err != nil {
		c.Ui.Error(err.Error())
	}

	scanLogPath := func(input string) error {
		// Trim leading and trailing spaces.
		input = strings.TrimSpace(input)
		path := strings.Split(input, "/")

		// return error if file does not end with expected file type
		if !strings.Contains(path[len(path)-1], ".txt") ||
			!strings.Contains(path[len(path)-1], ".text") ||
			!strings.Contains(path[len(path)-1], ".log") ||
			!strings.Contains(path[len(path)-1], ".syslog") {
			return errors.New("unrecognized log file type")
		}
		//update path value if "/" was not found and we are on windows
		if runtime.GOOS == "windows" {
			path = strings.Split(input, "\\")
		}

		if len(path) == 1 {
			// return as safe if "/" was not found and we're not on windows
			if runtime.GOOS != "windows" {
				return nil
			}

		}
		// Create a regex pattern to exclude unwanted characters.
		re := regexp.MustCompile(`[!@#\$%^&~*()\x60+=\[\]{};':"\\|,<>\/?]`) // identical to above but . is allowed

		for _, p := range path {
			unsafe := re.MatchString(p)
			if unsafe {
				return errors.New("Disallowed character detected in log path segment, directory and file names may only contain '-', '_','.', and alphanumeric characters.")
			}
		}
		return nil
	}

	// Check serviceName for unsafe characters
	err = scanLogPath(c.logPath)
	if err != nil {
		c.Ui.Error(err.Error())
	}
	params := map[string]string{
		"node_id":      c.nodeID,
		"server_id":    c.serverID,
		"log_since":    strconv.Itoa(c.logSince),
		"service_name": c.serviceName,
		"log_path":     c.logPath,
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

	//journalReader, journalWriter := io.Pipe()
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
