package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/hashicorp/nomad/api"
)

type EventSinkRegisterCommand struct {
	Meta
	testStdin io.Reader
}

func (c *EventSinkRegisterCommand) Help() string {
	helpText := `
Usage: nomad event sink register <path>

   Register is used to register a new event sink. The event sink is
   sourced from <path> or from stdin if path is "-".

General Options:

  ` + generalOptionsUsage(usageOptsDefault)

	return helpText
}

func (c *EventSinkRegisterCommand) Name() string { return "event sink register" }

func (c *EventSinkRegisterCommand) Run(args []string) int {
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

	path := args[0]
	bs, err := c.readAll(path)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error reading file %q: %s", path, err))
		return 1
	}

	var sink api.EventSink
	err = json.Unmarshal(bs, &sink)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error unmarshaling config: %s", err))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	_, err = client.EventSinks().Register(&sink, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error registering event sink: %s", err))
		return 1
	}

	c.Ui.Output(fmt.Sprintf("Successfully registered %q event sink!",
		sink.ID))
	return 0
}

func (c *EventSinkRegisterCommand) Synopsis() string {
	return "Register an event sink"
}

func (c *EventSinkRegisterCommand) readAll(path string) ([]byte, error) {
	if path == "-" {
		var r io.Reader = os.Stdin
		if c.testStdin != nil {
			r = c.testStdin
		}
		var buf bytes.Buffer
		_, err := io.Copy(&buf, r)
		if err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bs, nil
}
