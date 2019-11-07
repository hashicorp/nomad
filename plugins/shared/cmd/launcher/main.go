package main

import (
	"os"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/shared/cmd/launcher/command"
	"github.com/mitchellh/cli"
)

const (
	NomadPluginLauncherCli        = "nomad-plugin-launcher"
	NomadPluginLauncherCliVersion = "0.0.1"
)

func main() {
	ui := &cli.BasicUi{
		Reader:      os.Stdin,
		Writer:      os.Stdout,
		ErrorWriter: os.Stderr,
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:   NomadPluginLauncherCli,
		Output: &cli.UiWriter{Ui: ui},
	})

	c := cli.NewCLI(NomadPluginLauncherCli, NomadPluginLauncherCliVersion)
	c.Args = os.Args[1:]

	meta := command.NewMeta(ui, logger)
	c.Commands = map[string]cli.CommandFactory{
		"device": command.DeviceCommandFactory(meta),
	}

	exitStatus, err := c.Run()
	if err != nil {
		logger.Error("command exited with non-zero status", "status", exitStatus, "error", err)
	}
	os.Exit(exitStatus)
}
