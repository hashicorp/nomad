package command

import (
	"fmt"
)

type EventSinkDeregisterCommand struct {
	Meta
}

func (c *EventSinkDeregisterCommand) Help() string {
	helpText := `
Usage: nomad event sink deregister <event sink id>

   Deregister is used to deregister a registered event sink.

General Options:

  ` + generalOptionsUsage(usageOptsDefault)

	return helpText
}

func (c *EventSinkDeregisterCommand) Name() string { return "event sink deregister" }

func (c *EventSinkDeregisterCommand) Run(args []string) int {
	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got one argument
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error("This command takes one argument: <path>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	id := args[0]

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	_, err = client.EventSinks().Deregister(id, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error deregistering event sink: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully deregistered %q event sink!", id))
	return 0
}

func (c *EventSinkDeregisterCommand) Synopsis() string {
	return "Deregister an event sink"
}
