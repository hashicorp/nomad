package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/client/logdaemon"
	"github.com/hashicorp/nomad/nomad/structs"
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
	var config *structs.LogDaemonConfig
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
	logDaemon.Wait()
	return 0
}

func (l *LogDaemonCommand) parseConfig(args []string) (*structs.LogDaemonConfig, error) {
	flags := l.Meta.FlagSet("log-daemon", FlagSetClient)
	flags.Usage = func() { l.Ui.Output(l.Help()) }

	if err := flags.Parse(args); err != nil {
		return nil, fmt.Errorf("unable to parse args: %v", err)
	}

	// Extract the json passed with args
	args = flags.Args()
	if len(args) != 1 {
		return nil, fmt.Errorf("passing the configuration is mandatory")
	}
	configuration := args[0]

	// De-serialize the passed configuration
	var config structs.LogDaemonConfig
	dec := json.NewDecoder(strings.NewReader(configuration))
	if err := dec.Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}
