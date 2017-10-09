package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/posener/complete"
)

type QuotaApplyCommand struct {
	Meta
}

func (c *QuotaApplyCommand) Help() string {
	helpText := `
Usage: nomad quota apply [options] <input>

Apply is used to create or update a quota specification. The specification file
will be read from stdin by specifying "-", otherwise a path to the file is
expected. The file should be a JSON formatted quota specification.

General Options:

  ` + generalOptionsUsage()

	return strings.TrimSpace(helpText)
}

func (c *QuotaApplyCommand) AutocompleteFlags() complete.Flags {
	return c.Meta.AutocompleteFlags(FlagSetClient)
}

func (c *QuotaApplyCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFiles("*.json")
}

func (c *QuotaApplyCommand) Synopsis() string {
	return "Create or update a quota specification"
}

func (c *QuotaApplyCommand) Run(args []string) int {
	flags := c.Meta.FlagSet("quota apply", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we get exactly one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Read the file contents
	file := args[0]
	var rawQuota []byte
	var err error
	if file == "-" {
		rawQuota, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read stdin: %v", err))
			return 1
		}
	} else {
		rawQuota, err = ioutil.ReadFile(file)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to read file: %v", err))
			return 1
		}
	}

	var spec api.QuotaSpec
	dec := json.NewDecoder(bytes.NewBuffer(rawQuota))
	if err := dec.Decode(&spec); err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to parse quota: %v", err))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	_, err = client.Quotas().Register(&spec, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error applying quota specification: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully applied quota specification %q!", spec.Name))
	return 0
}
