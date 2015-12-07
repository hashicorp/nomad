package command

import (
	"fmt"
	"github.com/hashicorp/nomad/command/logdaemon"
	"strings"
)

type LogDaemonCommand struct {
	Meta
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
	var config *logdaemon.LogDaemonConfig
	var logDaemon *logdaemon.LogDaemon
	var err error

	if config, err = l.parseConfig(args); err != nil {
		l.Ui.Error(err.Error())
		return 1
	}
	if logDaemon, err = logdaemon.NewLogDaemon(config); err != nil {
		l.Ui.Error(err.Error())
		return 1
	}
	logDaemon.Start()
	return 0
}

func (l *LogDaemonCommand) parseConfig(args []string) (*logdaemon.LogDaemonConfig, error) {
	flags := l.Meta.FlagSet("log-daemon", FlagSetClient)
	flags.Usage = func() { l.Ui.Output(l.Help()) }

	if err := flags.Parse(args); err != nil {
		return nil, fmt.Errorf("Unable to parse args: %v", err)
	}

	config := logdaemon.NewLogDaemonConfig()
	return config, nil
}
