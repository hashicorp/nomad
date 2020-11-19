package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"
)

type EventSinkListCommand struct {
	Meta
}

func (c *EventSinkListCommand) Help() string {
	helpText := `
Usage: nomad event sink list

  List is used to list event sinks that have been registered.

  When ACLs are enabled, this command requires a token with the
  'operator:read' capability.

General Options:

  ` + generalOptionsUsage(usageOptsDefault)

	return helpText
}

func (c *EventSinkListCommand) Name() string { return "event sink list" }

func (c *EventSinkListCommand) Run(args []string) int {

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got no arguments
	args = flags.Args()
	if l := len(args); l != 0 {
		c.Ui.Error("This command takes no arguments")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	sinks, _, err := client.EventSinks().List(nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving event sinks: %s", err))
		return 1
	}

	c.Ui.Output(formatEventSinks(sinks))
	return 0
}

func (e *EventSinkListCommand) Synopsis() string {
	return "List event sinks"
}

func formatEventSinks(sinks []*api.EventSink) string {
	if len(sinks) == 0 {
		return "No event sinks found"
	}

	rows := make([]string, len(sinks)+1)
	rows[0] = "ID|Type|Address|Topics|LatestIndex"
	for i, s := range sinks {
		rows[i+1] = fmt.Sprintf("%s|%s|%s|%s|%d",
			s.ID,
			s.Type,
			s.Address,
			formatTopics(s.Topics),
			s.LatestIndex)
	}
	return formatList(rows)
}

func formatTopics(topicMap map[api.Topic][]string) string {
	var formatted []string
	var topics []string

	for topic := range topicMap {
		topics = append(topics, string(topic))
	}

	sort.Strings(topics)

	for _, t := range topics {
		out := fmt.Sprintf("%s[%s]", t, strings.Join(topicMap[api.Topic(t)], " "))
		formatted = append(formatted, out)
	}
	return strings.Join(formatted, ",")
}
