package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/version"
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
		case "deployment list", "deployment status", "deployment pause",
			"deployment resume", "deployment fail", "deployment promote":
		case "fs ls", "fs cat", "fs stat":
		case "job deployments", "job dispatch", "job history", "job promote", "job revert":
		case "namespace list", "namespace delete", "namespace apply":
		case "operator raft", "operator raft list-peers", "operator raft remove-peer":
		default:
			commandsInclude = append(commandsInclude, k)
		}
	}

	// Hidden hides the commands from both help and autocomplete. Commands that
	// users should not be running should be placed here, versus hiding
	// subcommands from the main help, which should be filtered out of the
	// commands above.
	hidden := []string{"check", "executor", "syslog"}

	cli := &cli.CLI{
		Name:           "nomad",
		Version:        version.GetVersion().FullVersionNumber(true),
		Args:           args,
		Commands:       commands,
		Autocomplete:   true,
		HiddenCommands: hidden,
		HelpFunc:       cli.FilteredHelpFunc(commandsInclude, helpFunc),
	}

	exitCode, err := cli.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %s\n", err.Error())
		return 1
	}

	return exitCode
}

// helpFunc is a custom help function. At the moment it is essentially a copy of
// the cli.BasicHelpFunc that includes flags demonstrating how to use the
// autocomplete flags.
func helpFunc(commands map[string]cli.CommandFactory) string {
	var buf bytes.Buffer
	buf.WriteString("Usage: nomad [-version] [-help] [-autocomplete-(un)install] <command> [<args>]\n\n")
	buf.WriteString("Available commands are:\n")

	// Get the list of keys so we can sort them, and also get the maximum
	// key length so they can be aligned properly.
	keys := make([]string, 0, len(commands))
	maxKeyLen := 0
	for key := range commands {
		if len(key) > maxKeyLen {
			maxKeyLen = len(key)
		}

		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		commandFunc, ok := commands[key]
		if !ok {
			// This should never happen since we JUST built the list of
			// keys.
			panic("command not found: " + key)
		}

		command, err := commandFunc()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERR] cli: Command '%s' failed to load: %s", key, err)
			continue
		}

		key = fmt.Sprintf("%s%s", key, strings.Repeat(" ", maxKeyLen-len(key)))
		buf.WriteString(fmt.Sprintf("    %s    %s\n", key, command.Synopsis()))
	}

	return buf.String()
}
