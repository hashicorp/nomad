package command

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
)

type MonitorCommand struct {
	Meta
}

func (c *MonitorCommand) Help() string {
	helpText := `
Usage: nomad monitor [options]

	Shows recent log messages of a nomad agent, and attaches to the agent,
	outputting log messagse as they occur in real time. The monitor lets you
	listen for log levels that may be filtered out of the Nomad agent. For
	example your agent may only be logging at INFO level, but with the monitor
	command you can set -log-level DEBUG

General Options:

  ` + generalOptionsUsage()
	return strings.TrimSpace(helpText)
}

func (c *MonitorCommand) Synopsis() string {
	return "stream logs from a nomad agent"
}

func (c *MonitorCommand) Name() string { return "monitor" }

func (c *MonitorCommand) Run(args []string) int {
	c.Ui = &cli.PrefixedUi{
		OutputPrefix: "    ",
		InfoPrefix:   "    ",
		ErrorPrefix:  "==> ",
		Ui:           c.Ui,
	}

	var logLevel string
	var nodeID string
	var logJSON bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&logLevel, "log-level", "", "")
	flags.StringVar(&nodeID, "node-id", "", "")
	flags.BoolVar(&logJSON, "log-json", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	params := map[string]string{
		"log-level": logLevel,
		"node-id":   nodeID,
		"log-json":  strconv.FormatBool(logJSON),
	}

	query := &api.QueryOptions{
		Params: params,
	}
	eventDoneCh := make(chan struct{})
	logCh, err := client.Agent().Monitor(eventDoneCh, query)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error starting monitor: %s", err))
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	go func() {
		defer close(eventDoneCh)
	OUTER:
		for {
			select {
			case log := <-logCh:
				if log == "" {
					break OUTER
				}
				c.Ui.Output(log)
			}
		}

	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	select {
	case <-eventDoneCh:
		c.Ui.Error("Remote side ended the monitor! This usually means that the\n" +
			"remote side has exited or crashed.")
		return 1
	case <-signalCh:
		return 0
	}
}
