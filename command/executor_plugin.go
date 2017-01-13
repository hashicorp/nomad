package command

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/hashicorp/go-plugin"

	"github.com/hashicorp/nomad/client/driver"
	dstructs "github.com/hashicorp/nomad/client/driver/structs"
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
	if len(args) != 1 {
		e.Ui.Error("json configuration not provided")
		return 1
	}
	config := args[0]
	var executorConfig dstructs.ExecutorConfig
	if err := json.Unmarshal([]byte(config), &executorConfig); err != nil {
		return 1
	}
	stdo, err := os.OpenFile(executorConfig.LogFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		e.Ui.Error(err.Error())
		return 1
	}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: driver.HandshakeConfig,
		Plugins:         driver.GetPluginMap(stdo, executorConfig.LogLevel),
	})
	return 0
}
