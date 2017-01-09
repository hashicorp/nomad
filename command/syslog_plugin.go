package command

import (
	"os"
	"strings"

	"github.com/hashicorp/go-plugin"

	"github.com/hashicorp/nomad/client/driver"
)

type SyslogPluginCommand struct {
	Meta
}

func (e *SyslogPluginCommand) Help() string {
	helpText := `
	This is a command used by Nomad internally to launch a syslog collector"
	`
	return strings.TrimSpace(helpText)
}

func (s *SyslogPluginCommand) Synopsis() string {
	return "internal - launch a syslog collector plugin"
}

func (s *SyslogPluginCommand) Run(args []string) int {
	if len(args) == 2 {
		s.Ui.Error("log output file and log level are not provided")
		return 1
	}
	logFileName := args[0]
	logLevel := args[1]
	stdo, err := os.OpenFile(logFileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		s.Ui.Error(err.Error())
		return 1
	}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: driver.HandshakeConfig,
		Plugins:         driver.GetPluginMap(stdo, logLevel),
	})

	return 0
}
