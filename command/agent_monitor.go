package command

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
	var logLevel string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&logLevel, "log-level", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	eventDoneCh := make(chan struct{})
	logCh, err := client.Agent().Monitor(logLevel, eventDoneCh, nil)
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
