package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/posener/complete"
)

const (
	// DefaultInitName is the default name we use when
	// initializing the example file
	DefaultInitName = "example.nomad"
)

// JobInitCommand generates a new job template that you can customize to your
// liking, like vagrant init
type JobInitCommand struct {
	Meta
}

func (c *JobInitCommand) Help() string {
	helpText := `
Usage: nomad job init <filename>
Alias: nomad init <filename>

  Creates an example job file that can be used as a starting point to customize
  further. If no filename is given, the default of "example.nomad" will be used.

Init Options:

  -short
    If the short flag is set, a minimal jobspec without comments is emitted.

  -connect
    If the connect flag is set, the jobspec includes Consul Connect integration.
`
	return strings.TrimSpace(helpText)
}

func (c *JobInitCommand) Synopsis() string {
	return "Create an example job file"
}

func (c *JobInitCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-short": complete.PredictNothing,
		})
}

func (c *JobInitCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *JobInitCommand) Name() string { return "job init" }

func (c *JobInitCommand) Run(args []string) int {
	var short bool
	var connect bool

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&short, "short", false, "")
	flags.BoolVar(&connect, "connect", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check for misuse
	// Check that we either got no filename or exactly one.
	args = flags.Args()
	if len(args) > 1 {
		c.Ui.Error("This command takes either no arguments or one: <filename>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	filename := DefaultInitName
	if len(args) == 1 {
		filename = args[0]
	}

	// Check if the file already exists
	_, err := os.Stat(filename)
	if err != nil && !os.IsNotExist(err) {
		c.Ui.Error(fmt.Sprintf("Failed to stat '%s': %v", filename, err))
		return 1
	}
	if !os.IsNotExist(err) {
		c.Ui.Error(fmt.Sprintf("Job '%s' already exists", filename))
		return 1
	}

	var jobSpec []byte
	switch {
	case connect && !short:
		jobSpec, err = Asset("command/assets/connect.nomad")
	case connect && short:
		jobSpec, err = Asset("command/assets/connect-short.nomad")
	case !connect && short:
		jobSpec, err = Asset("command/assets/example-short.nomad")
	default:
		jobSpec, err = Asset("command/assets/example.nomad")
	}
	if err != nil {
		// should never see this because we've precompiled the assets
		// as part of `make generate-examples`
		c.Ui.Error(fmt.Sprintf("Accessed non-existent asset: %s", err))
		return 1
	}

	// Write out the example
	err = os.WriteFile(filename, jobSpec, 0660)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to write '%s': %v", filename, err))
		return 1
	}

	// Success
	c.Ui.Output(fmt.Sprintf("Example job file written to %s", filename))
	return 0
}
