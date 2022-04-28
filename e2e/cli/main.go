package main

import (
	"os"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/e2e/cli/command"
	"github.com/mitchellh/cli"
)

const (
	NomadE2ECli        = "nomad-e2e"
	NomadE2ECliVersion = "0.0.1"
)

func main() {

	ui := &cli.BasicUi{
		Reader:      os.Stdin,
		Writer:      os.Stdout,
		ErrorWriter: os.Stderr,
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:   NomadE2ECli,
		Output: &cli.UiWriter{Ui: ui},
	})

	c := cli.NewCLI(NomadE2ECli, NomadE2ECliVersion)
	c.Args = os.Args[1:]

	meta := command.NewMeta(ui, logger)
	c.Commands = map[string]cli.CommandFactory{
		"provision": command.ProvisionCommandFactory(meta),
		"run":       command.RunCommandFactory(meta),
	}

	exitStatus, err := c.Run()
	if err != nil {
		logger.Error("command exited with non-zero status", "status", exitStatus, "error", err)
	}
	os.Exit(exitStatus)
}
