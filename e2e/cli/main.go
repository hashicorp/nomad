package main

import (
	"log"
	"os"

	"github.com/hashicorp/nomad/e2e/cli/command"
	"github.com/mitchellh/cli"
)

func main() {
	log.SetPrefix("@@@ ==> ")

	c := cli.NewCLI("nomad-e2e", "0.0.1")
	c.Args = os.Args[1:]
	c.Commands = map[string]cli.CommandFactory{
		"provision": command.ProvisionCommandFactory,
		"run":       command.RunCommandFactory,
	}

	exitStatus, err := c.Run()
	if err != nil {
		log.Println(err)
	}
	os.Exit(exitStatus)
}
