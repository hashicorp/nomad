package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/posener/complete"
)

const (
	// DefaultHclVarInitName is the default name we use when initializing the
	// example var file in HCL format
	DefaultHclVarInitName = "spec.nsv.hcl"

	// DefaultHclVarInitName is the default name we use when initializing the
	// example var file in JSON format
	DefaultJsonVarInitName = "spec.nsv.json"
)

// VarInitCommand generates a new var spec that you can customize to your
// liking, like vagrant init
type VarInitCommand struct {
	Meta
}

func (c *VarInitCommand) Help() string {
	helpText := `
Usage: nomad var init <filename>

  Creates an example secure variable specification file that can be used as a
  starting point to customize further. If no filename is given, the default of
  "spec.nsv.hcl" or "spec.nsv.json" will be used.

Init Options:

  -json
    Create an example JSON secure variable specification.
`
	return strings.TrimSpace(helpText)
}

func (c *VarInitCommand) Synopsis() string {
	return "Create an example secure variable specification file"
}

func (c *VarInitCommand) AutocompleteFlags() complete.Flags {
	return complete.Flags{
		"-json": complete.PredictNothing,
	}
}

func (c *VarInitCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *VarInitCommand) Name() string { return "var init" }

func (c *VarInitCommand) Run(args []string) int {
	var jsonOutput bool
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&jsonOutput, "json", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we get no arguments
	args = flags.Args()
	if l := len(args); l > 1 {
		c.Ui.Error("This command takes no arguments or one: <filename>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	fileName := DefaultHclVarInitName
	fileContent := defaultHclVarSpec
	if jsonOutput {
		fileName = DefaultJsonVarInitName
		fileContent = defaultJsonVarSpec
	}
	if len(args) == 1 {
		fileName = args[0]
	}

	// Check if the file already exists
	_, err := os.Stat(fileName)
	if err != nil && !os.IsNotExist(err) {
		c.Ui.Error(fmt.Sprintf("Failed to stat %q: %v", fileName, err))
		return 1
	}
	if !os.IsNotExist(err) {
		c.Ui.Error(fmt.Sprintf("Secure variable specification %q already exists", fileName))
		return 1
	}

	// Write out the example
	err = ioutil.WriteFile(fileName, []byte(fileContent), 0660)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to write %q: %v", fileName, err))
		return 1
	}

	// Success
	c.Ui.Output(fmt.Sprintf("Example secure variable specification written to %s", fileName))
	return 0
}

var defaultHclVarSpec = strings.TrimSpace(`
# A secure variable path contains a collection of secure data expressed
# using HCL map syntax. Keys in the 'Items' collection should not contain
# dots as a best practice when possible to facilitates later use in Nomad
# templates.
Items {
  key1 = "value 1"
  key2 = "value 2"
}
`) + "\n"

var defaultJsonVarSpec = strings.TrimSpace(`
{
  "Items": {
    "key1": "value 1",
	"key2": "value 2"
  }
}
`) + "\n"
