package command

import (
	"context"
	"fmt"

	"github.com/mitchellh/go-glint"

	"github.com/hashicorp/nomad/api"
	cc "github.com/hashicorp/nomad/cli/components"
)

type DevCommand struct {
	UI *glint.Document
	Meta
}

func (c *DevCommand) Help() string { return ":)" }

func (c *DevCommand) Name() string { return "dev" }

func (c *DevCommand) Synopsis() string { return ":D" }

func (c *DevCommand) Run(_ []string) int {
	// Get the HTTP client
	client, err := c.Meta.Client()

	if err != nil {
		c.UI.Append(cc.Error(fmt.Sprintf("Error initializing client: %s", err)))
		c.UI.Render(context.Background())
		return 1
	}

	// The event stream serves as a sorta container thing.
	// I think the ultimate plan should be to have a generic "join"
	// type method on it that receives an event.
	//
	// Then, based on event topic and key, we can determine if that event
	// already exists in the list or not. If it exists, update, otherwise, append
	// It also means the event stream component can keep a map representation and
	// a slice representation of components without us needing to know about it
	// here in the stream reader.
	// But also idk, I'm just spitballing.
	es := &EventStreamComponent{}

	c.UI.Append(es)

	es.Append(glint.Layout(
		cc.IconRunning(),
		cc.Text("Running that sweet sweet dev command"),
	).Row())
	es.Append(cc.LineSpacing())

	go c.UI.Render(context.Background())

	events := client.EventStream()
	q := &api.QueryOptions{}
	topics := map[api.Topic][]string{
		"Job": {"*"},
	}

	streamCh, err := events.Stream(context.Background(), topics, 0, q)

	for frame := range streamCh {
		for _, event := range frame.Events {
			es.Append(JobEvent(event))
		}
	}

	fmt.Scanln()

	return 0
}

type EventStreamComponent struct {
	components []glint.Component
}

func (c *EventStreamComponent) Append(component glint.Component) {
	c.components = append(c.components, component)
}

func (c *EventStreamComponent) Body(context.Context) glint.Component {
	return glint.Layout(c.components...)
}

func JobEvent(event api.Event) glint.Component {
	job := event.Payload["Job"].(map[string]interface{})
	status := job["Status"].(string)
	var statusComponent *glint.LayoutComponent

	switch status {
	case "pending":
		statusComponent = glint.Layout(cc.IconRunning(), cc.Subtle("pending")).Row()
	case "running":
		statusComponent = glint.Layout(cc.IconSuccess(), cc.Success("running")).Row()
	case "dead":
		statusComponent = glint.Layout(cc.IconWarning(), cc.Warning("dead   ")).Row()
	}

	return glint.Layout(
		statusComponent.MarginRight(1),
		cc.Text(event.Type),
		cc.Text(fmt.Sprintf(" %s ", job["Name"])).Bold(),
		cc.Subtle(fmt.Sprintf("(ns: %s, type: %s, priority: %v)", job["Namespace"], job["Type"], job["Priority"])),
	).Row()
}
