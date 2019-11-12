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

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
)

type MonitorCommand struct {
	Meta
}

func (c *MonitorCommand) Help() string {
	helpText := `
Usage: nomad monitor [options]

	Stream log messages of a nomad agent. The monitor command lets you
	listen for log levels that may be filtered out of the Nomad agent. For
	example your agent may only be logging at INFO level, but with the monitor
	command you can set -log-level DEBUG

General Options:

	` + generalOptionsUsage() + `
	
Monitor Specific Options:

  -log-level <level>
    Sets the log level to monitor (default: INFO)

  -node-id <node-id>
    Sets the specific node to monitor

  -json
    Sets log output to JSON format
  `
	return strings.TrimSpace(helpText)
}

func (c *MonitorCommand) Synopsis() string {
	return "stream logs from a Nomad agent"
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
	var serverID string
	var logJSON bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&logLevel, "log-level", "", "")
	flags.StringVar(&nodeID, "node-id", "", "")
	flags.StringVar(&serverID, "server-id", "", "")
	flags.BoolVar(&logJSON, "json", false, "")

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

	params := map[string]string{
		"log_level": logLevel,
		"node_id":   nodeID,
		"server_id": serverID,
		"log_json":  strconv.FormatBool(logJSON),
	}

	query := &api.QueryOptions{
		Params: params,
	}

	eventDoneCh := make(chan struct{})
	frames, errCh := client.Agent().Monitor(eventDoneCh, query)
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
	frameReader.SetUnblockTime(500 * time.Millisecond)
	r = frameReader

	defer r.Close()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalCh
		// End the streaming
		r.Close()
	}()

	_, err = io.Copy(os.Stdout, r)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("error monitoring logs: %s", err))
		return 1
	}

	return 0
}
