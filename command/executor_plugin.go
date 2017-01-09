package command

import (
	"os"
	"strings"

	"github.com/hashicorp/go-plugin"

	"github.com/hashicorp/nomad/client/driver"
)

type ExecutorPluginCommand struct {
	Meta
}

func (e *ExecutorPluginCommand) Help() string {
	helpText := `
	This is a command used by Nomad internally to launch an executor plugin"
	`
	return strings.TrimSpace(helpText)
}

func (e *ExecutorPluginCommand) Synopsis() string {
	return "internal - launch an executor plugin"
}

func (e *ExecutorPluginCommand) Run(args []string) int {
	if len(args) != 2 {
		e.Ui.Error("log output file and log level are not provided")
		return 1
	}
	logFileName := args[0]
	logLevel := args[1]
	stdo, err := os.OpenFile(logFileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		e.Ui.Error(err.Error())
		return 1
	}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: driver.HandshakeConfig,
		Plugins:         driver.GetPluginMap(stdo, logLevel),
	})
	return 0
}
