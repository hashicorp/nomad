package main

import (
	"fmt"
	"os"

	"github.com/mitchellh/cli"
	"github.com/sean-/seed"
)

func init() {
	seed.Init()
}

func main() {
	os.Exit(Run(os.Args[1:]))
}

func Run(args []string) int {
	return RunCustom(args, Commands(nil))
}

func RunCustom(args []string, commands map[string]cli.CommandFactory) int {
	// Build the commands to include in the help now.
	commandsInclude := make([]string, 0, len(commands))
	for k, _ := range commands {
		switch k {
		case "check":
		case "deployment list", "deployment status", "deployment pause",
			"deployment resume", "deployment fail", "deployment promote":
		case "executor":
		case "fs ls", "fs cat", "fs stat":
		case "job deployments", "job dispatch", "job history", "job revert":
		case "operator raft", "operator raft list-peers", "operator raft remove-peer":
		case "syslog":
		default:
			commandsInclude = append(commandsInclude, k)
		}
	}

	cli := &cli.CLI{
		Name:         "nomad",
		Version:      PrettyVersion(GetVersionParts()),
		Args:         args,
		Commands:     commands,
		Autocomplete: true,
		HelpFunc:     cli.FilteredHelpFunc(commandsInclude, cli.BasicHelpFunc("nomad")),
	}

	exitCode, err := cli.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %s\n", err.Error())
		return 1
	}

	return exitCode
}
