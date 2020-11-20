package command

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/mitchellh/go-glint"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/cli/components"
	cc "github.com/hashicorp/nomad/cli/components"
	"github.com/hashicorp/nomad/helper/uuid"
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
	es := &EventStreamComponent{
		components: make(map[string]*component),
	}

	c.UI.Append(es)

	introIcon := cc.IconRunning()
	// Imaginary wait time to set icon from spinner to success
	go func() {
		time.Sleep(5 * time.Second)
		introIcon.SetFinal(cc.IconHealthy())
	}()

	introStartText := cc.Text("Running that sweet sweet dev command")

	intro := glint.Layout(
		introIcon,
		introStartText,
	).Row()
	es.Append("0", intro, nil)

	es.Append("1", cc.LineSpacing(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	go c.UI.Render(ctx)

	// Trap signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		<-sigCh
		cancel()
	}()

	events := client.EventStream()
	q := &api.QueryOptions{}
	topics := map[api.Topic][]string{
		"*": {"*"},
	}

	streamCh, err := events.Stream(context.Background(), topics, 0, q)

	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return 0
		case event := <-streamCh:
			if event.Err != nil {
				es.Append(uuid.Generate(), components.Error(event.Err.Error()), nil)
				return 1
			}

			for _, event := range event.Events {
				es.handleEvent(event)
			}
		}

	}
}

type EventStreamComponent struct {
	components map[string]*component
}

type component struct {
	state     *componentState
	component glint.Component
}

type componentState struct {
	subtleText string
	mainText   string
	icon       string
	eventType  string
}

func (e *EventStreamComponent) handleEvent(event api.Event) {
	switch event.Topic {
	case api.TopicJob:
		job, err := event.Job()
		if err != nil {
			e.Append(event.Key, components.Error(err.Error()), nil)
			return
		}
		e.JobEvent(job, event.Type, event.Key)
	case api.TopicDeployment:
	case api.TopicNode:
		node, err := event.Node()
		if err != nil {
			e.Append(event.Key, components.Error(err.Error()), nil)
			return
		}
		e.Append(event.Key, e.NodeEvent(node, event.Type, event.Key), nil)

	default:
	}
}

func (c *EventStreamComponent) Append(key string, gcomponent glint.Component, state *componentState) {
	c.components[key] = &component{state: state, component: gcomponent}
}

func (c *EventStreamComponent) Body(context.Context) glint.Component {
	var keys []string
	for k := range c.components {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var comps []glint.Component
	for _, k := range keys {
		comps = append(comps, c.components[k].component)
	}

	return glint.Layout(comps...)
}

func (e *EventStreamComponent) JobEvent(job *api.Job, eventType, key string) {
	status := *job.Status

	var existing bool
	var state *componentState
	if component, existing := e.components[key]; existing {
		state = component.state
	} else {
		state = &componentState{}
	}

	state.eventType = eventType
	switch status {
	case "pending":
		state.subtleText = "pending"
		state.icon = "Running"
	case "running":
		state.subtleText = "running"
		state.icon = "Success"
	case "dead":
		state.subtleText = "dead"
		state.icon = "Warning"
	}

	if !existing {
		layout := glint.Layout(
			glint.Layout(cc.IconFor(state.icon), cc.Status(state.subtleText, state.icon)).Row().MarginRight(1),
			cc.Text(eventType),
			cc.Text(fmt.Sprintf(" %s ", *job.Name)).Bold(),
			cc.Subtle(fmt.Sprintf("(ns: %s, type: %s, priority: %v)", *job.Namespace, *job.Type, *job.Priority)),
		).Row()

		e.Append(key, layout, state)
	}

}

func (e *EventStreamComponent) NodeEvent(node *api.Node, eventType, key string) glint.Component {
	var statusComponent *glint.LayoutComponent
	switch node.Status {
	case api.NodeStatusInit:
		statusComponent = glint.Layout(cc.IconRunning(), cc.Subtle("pending")).Row()
	case api.NodeStatusReady:
		statusComponent = glint.Layout(cc.IconSuccess(), cc.Subtle("ready")).Row()
	case api.NodeStatusDown:
		statusComponent = glint.Layout(cc.IconWarning(), cc.Subtle("down")).Row()
	}

	return glint.Layout(
		statusComponent.MarginRight(1),
		cc.Text(eventType),
		cc.Text(fmt.Sprintf(" %s ", node.ID[0:8])).Bold(),
	).Row()
}
