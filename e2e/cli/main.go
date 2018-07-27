package main

import (
	"os"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/e2e/cli/command"
	"github.com/mitchellh/cli"
)

func main() {

	ui := &cli.BasicUi{
		Reader:      os.Stdin,
		Writer:      os.Stdout,
		ErrorWriter: os.Stderr,
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "nomad-e2e",
		Output: &cli.UiWriter{ui},
	})

	c := cli.NewCLI("nomad-e2e", "0.0.1")
	c.Args = os.Args[1:]
	c.Commands = map[string]cli.CommandFactory{
		"provision": command.ProvisionCommandFactory(ui, logger),
		"run":       command.RunCommandFactory(ui, logger),
	}

	exitStatus, err := c.Run()
	if err != nil {
		logger.Error("command exited with non-zero status", "status", exitStatus, "error", err)
	}
	os.Exit(exitStatus)
}
