package command

import (
	"fmt"
	"strings"
)

type LogDaemonCommand struct {
	Meta
}

type LogDaemonConfig struct {
	apiPort       int
	apitInterface string
}

func NewLogDaemonConfig() *LogDaemonConfig {
	return &LogDaemonConfig{
		apiPort:       4470,
		apitInterface: "lo",
	}
}

func (l *LogDaemonCommand) Help() string {
	helpText := `
Usage: nomad log [options]

  INTERNAL ONLY
  
  Spawns a daemon process that provides an HTTP API for users to access logs of 
  Tasks that are running on the Nomad Client. This daemon is forked off by
  the Nomad Client.
  `
	return strings.TrimSpace(helpText)
}

func (l *LogDaemonCommand) Synopsis() string {
	return "Creates the logging daemon"
}

func (l *LogDaemonCommand) Run(args []string) int {
	l.parseConfig(args)
	return 0
}

func (l *LogDaemonCommand) parseConfig(args []string) (*LogDaemonConfig, error) {
	flags := l.Meta.FlagSet("log-daemon", FlagSetClient)
	flags.Usage = func() { l.Ui.Output(l.Help()) }

	if err := flags.Parse(args); err != nil {
		return nil, fmt.Errorf("Unable to parse args: %v", err)
	}

	config := NewLogDaemonConfig()
	return config, nil
}
