package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	// These packages have init() funcs which check os.Args and drop directly
	// into their command logic. This is because they are run as separate
	// processes along side of a task. By early importing them we can avoid
	// additional code being imported and thus reserving memory.
	_ "github.com/hashicorp/nomad/client/allocrunner/taskrunner/getter"
	_ "github.com/hashicorp/nomad/client/logmon"
	_ "github.com/hashicorp/nomad/drivers/docker/docklog"
	_ "github.com/hashicorp/nomad/drivers/shared/executor"

	// Don't move any other code imports above the import block above!
	"github.com/hashicorp/nomad/command"
	"github.com/hashicorp/nomad/version"
	"github.com/mitchellh/cli"
	"github.com/sean-/seed"
)

var (
	// Hidden hides the commands from both help and autocomplete. Commands that
	// users should not be running should be placed here, versus hiding
	// subcommands from the main help, which should be filtered out of the
	// commands above.
	hidden = []string{
		"alloc-status",
		"check",
		"client-config",
		"debug",
		"eval-status",
		"executor",
		"keygen",
		"keyring",
		"logmon",
		"node-drain",
		"node-status",
		"server-force-leave",
		"server-join",
		"server-members",
		"syslog",
		"docker_logger",
		"operator raft _info",
		"operator raft _logs",
		"operator raft _state",
		"operator snapshot _state",
	}

	// aliases is the list of aliases we want users to be aware of. We hide
	// these form the help output but autocomplete them.
	aliases = []string{
		"fs",
		"init",
		"inspect",
		"logs",
		"plan",
		"validate",
	}

	// Common commands are grouped separately to call them out to operators.
	commonCommands = []string{
		"run",
		"stop",
		"status",
		"alloc",
		"job",
		"node",
		"agent",
	}
)

func init() {
	seed.Init()
}

func main() {
	os.Exit(Run(os.Args[1:]))
}

func Run(args []string) int {
	return RunCustom(args)
}

func RunCustom(args []string) int {
	// Create the meta object
	metaPtr := new(command.Meta)
	metaPtr.SetupUi(args)

	// The Nomad agent never outputs color
	agentUi := &cli.BasicUi{
		Reader:      os.Stdin,
		Writer:      os.Stdout,
		ErrorWriter: os.Stderr,
	}

	commands := command.Commands(metaPtr, agentUi)
	cli := &cli.CLI{
		Name:                       "nomad",
		Version:                    version.GetVersion().FullVersionNumber(true),
		Args:                       args,
		Commands:                   commands,
		HiddenCommands:             hidden,
		Autocomplete:               true,
		AutocompleteNoDefaultFlags: true,
		HelpFunc: groupedHelpFunc(
			cli.BasicHelpFunc("nomad"),
		),
		HelpWriter: os.Stdout,
	}

	exitCode, err := cli.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %s\n", err.Error())
		return 1
	}

	return exitCode
}

func groupedHelpFunc(f cli.HelpFunc) cli.HelpFunc {
	return func(commands map[string]cli.CommandFactory) string {
		var b bytes.Buffer
		tw := tabwriter.NewWriter(&b, 0, 2, 6, ' ', 0)

		fmt.Fprintf(tw, "Usage: nomad [-version] [-help] [-autocomplete-(un)install] <command> [args]\n\n")
		fmt.Fprintf(tw, "Common commands:\n")
		for _, v := range commonCommands {
			printCommand(tw, v, commands[v])
		}

		// Filter out common commands and aliased commands from the other
		// commands output
		otherCommands := make([]string, 0, len(commands))
		for k := range commands {
			found := false
			for _, v := range commonCommands {
				if k == v {
					found = true
					break
				}
			}

			for _, v := range aliases {
				if k == v {
					found = true
					break
				}
			}

			if !found {
				otherCommands = append(otherCommands, k)
			}
		}
		sort.Strings(otherCommands)

		fmt.Fprintf(tw, "\n")
		fmt.Fprintf(tw, "Other commands:\n")
		for _, v := range otherCommands {
			printCommand(tw, v, commands[v])
		}

		tw.Flush()

		return strings.TrimSpace(b.String())
	}
}

func printCommand(w io.Writer, name string, cmdFn cli.CommandFactory) {
	cmd, err := cmdFn()
	if err != nil {
		panic(fmt.Sprintf("failed to load %q command: %s", name, err))
	}
	fmt.Fprintf(w, "    %s\t%s\n", name, cmd.Synopsis())
}
