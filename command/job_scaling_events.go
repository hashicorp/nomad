// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// Ensure JobScalingEventsCommand satisfies the cli.Command interface.
var _ cli.Command = &JobScalingEventsCommand{}

// JobScalingEventsCommand implements cli.Command.
type JobScalingEventsCommand struct {
	Meta
}

// Help satisfies the cli.Command Help function.
func (j *JobScalingEventsCommand) Help() string {
	helpText := `
Usage: nomad job scaling-events [options] <args>

  List the scaling events for the specified job.

  When ACLs are enabled, this command requires a token with either the
  'read-job' or 'read-job-scaling' capability for the job's namespace. The
  'list-jobs' capability is required to run the command with a job prefix
  instead of the exact job ID.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Scaling-Events Options:

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

// Synopsis satisfies the cli.Command Synopsis function.
func (j *JobScalingEventsCommand) Synopsis() string {
	return "Display the most recent scaling events for a job"
}

func (j *JobScalingEventsCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(j.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-verbose": complete.PredictNothing,
		})
}

// Name returns the name of this command.
func (j *JobScalingEventsCommand) Name() string { return "job scaling-events" }

// Run satisfies the cli.Command Run function.
func (j *JobScalingEventsCommand) Run(args []string) int {

	var verbose bool

	flags := j.Meta.FlagSet(j.Name(), FlagSetClient)
	flags.Usage = func() { j.Ui.Output(j.Help()) }
	flags.BoolVar(&verbose, "verbose", false, "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	args = flags.Args()
	if len(args) != 1 {
		j.Ui.Error("This command takes one argument: <job_id>")
		j.Ui.Error(commandErrorText(j))
		return 1
	}

	// Get the HTTP client.
	client, err := j.Meta.Client()
	if err != nil {
		j.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Check if the job exists
	jobIDPrefix := strings.TrimSpace(args[0])
	jobID, namespace, err := j.JobIDByPrefix(client, jobIDPrefix, nil)
	if err != nil {
		j.Ui.Error(err.Error())
		return 1
	}

	q := &api.QueryOptions{Namespace: namespace}
	events, _, err := client.Jobs().ScaleStatus(jobID, q)
	if err != nil {
		j.Ui.Error(fmt.Sprintf("Error listing scaling events: %s", err))
		return 1
	}

	// Check if any of the task groups have scaling events, otherwise exit
	// indicating there are not any.
	var haveEvents bool
	for _, tg := range events.TaskGroups {
		if tg.Events != nil {
			haveEvents = true
			break
		}
	}

	if !haveEvents {
		j.Ui.Output("No events found")
		return 0
	}

	// Create our sorted list of events and output.
	sortedList := sortedScalingEventList(events)
	j.Ui.Output(formatList(formatScalingEventListOutput(sortedList, verbose, 0)))
	return 0
}

func formatScalingEventListOutput(e scalingEventList, verbose bool, limit int) []string {

	// If the limit is zero, aka no limit or the limit is greater than the
	// number of events we have then set it this to the length of the event
	// list.
	if limit == 0 || limit > len(e) {
		limit = len(e)
	}

	// Create the initial output heading.
	output := make([]string, limit+1)
	output[0] = "Task Group|Count|PrevCount"

	// If we are outputting verbose information, add these fields to the header
	// and then add our end date field.
	if verbose {
		output[0] += "|Error|Message|Eval ID"
	}
	output[0] += "|Date"

	var i int

	for i < limit {
		output[i+1] = fmt.Sprintf("%s|%s|%v", e[i].name, valueOrNil(e[i].event.Count), e[i].event.PreviousCount)
		if verbose {
			output[i+1] += fmt.Sprintf("|%v|%s|%s",
				e[i].event.Error, e[i].event.Message, valueOrNil(e[i].event.EvalID))
		}
		output[i+1] += fmt.Sprintf("|%v", formatTime(time.Unix(0, int64(e[i].event.Time))))
		i++
	}
	return output
}

// sortedScalingEventList generates a time sorted list of scaling events as
// provided by the api.JobScaleStatusResponse.
func sortedScalingEventList(e *api.JobScaleStatusResponse) []groupEvent {

	// sortedList is our output list.
	var sortedList scalingEventList

	// Iterate over the response object to create a sorted list.
	for group, status := range e.TaskGroups {
		for _, event := range status.Events {
			sortedList = append(sortedList, groupEvent{name: group, event: event})
		}
	}
	sort.Sort(sortedList)

	return sortedList
}

// valueOrNil helps format the event output in cases where the object has a
// potential to be nil.
func valueOrNil(i interface{}) string {
	switch t := i.(type) {
	case *int64:
		if t != nil {
			return strconv.FormatInt(*t, 10)
		}
	case *string:
		if t != nil {
			return *t
		}
	}
	return ""
}

// scalingEventList is a helper list of all events for the job which allows us
// to sort based on time.
type scalingEventList []groupEvent

// groupEvent contains all the required information of an individual group
// scaling event.
type groupEvent struct {
	name  string
	event api.ScalingEvent
}

// Len satisfies the Len function on the sort.Interface.
func (s scalingEventList) Len() int {
	return len(s)
}

// Less satisfies the Less function on the sort.Interface and sorts by the
// event time.
func (s scalingEventList) Less(i, j int) bool {
	return s[i].event.Time > s[j].event.Time
}

// Swap satisfies the Swap function on the sort.Interface.
func (s scalingEventList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
