// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
	"github.com/ryanuber/columnize"
)

type EvalStatusCommand struct {
	Meta
}

func (c *EvalStatusCommand) Help() string {
	helpText := `
Usage: nomad eval status [options] <evaluation>

  Display information about evaluations. This command can be used to inspect the
  current status of an evaluation as well as determine the reason an evaluation
  did not place all allocations.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Eval Status Options:

  -monitor
    Monitor an outstanding evaluation

  -verbose
    Show full information.

  -json
    Output the evaluation in its JSON format.

  -t
    Format and display evaluation using a Go template.

  -ui
    Open the evaluation in the browser.
`

	return strings.TrimSpace(helpText)
}

func (c *EvalStatusCommand) Synopsis() string {
	return "Display evaluation status and placement failure reasons"
}

func (c *EvalStatusCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-json":    complete.PredictNothing,
			"-monitor": complete.PredictNothing,
			"-t":       complete.PredictAnything,
			"-verbose": complete.PredictNothing,
			"-ui":      complete.PredictNothing,
		})
}

func (c *EvalStatusCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Evals, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Evals]
	})
}

func (c *EvalStatusCommand) Name() string { return "eval status" }

func (c *EvalStatusCommand) Run(args []string) int {
	var monitor, verbose, json, openURL bool
	var tmpl string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&monitor, "monitor", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	flags.BoolVar(&openURL, "ui", false, "")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one evaluation ID
	args = flags.Args()

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	if len(args) != 1 {
		c.Ui.Error("This command takes one argument")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	evalID := args[0]

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	// Query the allocation info
	if len(evalID) == 1 {
		c.Ui.Error("Identifier must contain at least two characters.")
		return 1
	}

	if json && len(tmpl) > 0 {
		c.Ui.Error("Both json and template formatting are not allowed")
		return 1
	}

	evalID = sanitizeUUIDPrefix(evalID)
	evals, _, err := client.Evaluations().PrefixList(evalID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying evaluation: %v", err))
		return 1
	}
	if len(evals) == 0 {
		c.Ui.Error(fmt.Sprintf("No evaluation(s) with prefix or id %q found", evalID))
		return 1
	}

	if len(evals) > 1 {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple evaluations\n\n%s", formatEvalList(evals, verbose)))
		return 1
	}

	// If we are in monitor mode, monitor and exit
	if monitor {
		mon := newMonitor(c.Ui, client, length)
		return mon.monitor(evals[0].ID)
	}

	// Prefix lookup matched a single evaluation
	eval, _, err := client.Evaluations().Info(evals[0].ID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying evaluation: %s", err))
		return 1
	}

	// If output format is specified, format and output the data
	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, eval)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	// TOOD
	var placedAllocs []*api.AllocationListStub

	c.formatEvalStatus(eval, placedAllocs, verbose, length)

	hint, _ := c.Meta.showUIPath(UIHintContext{
		Command: "eval status",
		PathParams: map[string]string{
			"evalID": eval.ID,
		},
		OpenURL: openURL,
	})
	if hint != "" {
		c.Ui.Warn(hint)
	}

	return 0
}

func (c *EvalStatusCommand) formatEvalStatus(eval *api.Evaluation, placedAllocs []*api.AllocationListStub, verbose bool, length int) {

	failureString, failures := evalFailureStatus(eval)
	triggerNoun, triggerSubj := getTriggerDetails(eval)
	statusDesc := eval.StatusDescription
	if statusDesc == "" {
		statusDesc = eval.Status
	}

	// Format eval timestamps
	var formattedCreateTime, formattedModifyTime string
	if verbose {
		formattedCreateTime = formatUnixNanoTime(eval.CreateTime)
		formattedModifyTime = formatUnixNanoTime(eval.ModifyTime)
	} else {
		formattedCreateTime = prettyTimeDiff(time.Unix(0, eval.CreateTime), time.Now())
		formattedModifyTime = prettyTimeDiff(time.Unix(0, eval.ModifyTime), time.Now())
	}

	// Format the evaluation data
	basic := []string{
		fmt.Sprintf("ID|%s", limit(eval.ID, length)),
		fmt.Sprintf("Create Time|%s", formattedCreateTime),
		fmt.Sprintf("Modify Time|%s", formattedModifyTime),
		fmt.Sprintf("Status|%s", eval.Status),
		fmt.Sprintf("Status Description|%s", statusDesc),
		fmt.Sprintf("Type|%s", eval.Type),
		fmt.Sprintf("TriggeredBy|%s", eval.TriggeredBy),
		fmt.Sprintf("Job ID|%s", eval.JobID),
		fmt.Sprintf("Namespace|%s", eval.Namespace),
	}

	if triggerNoun != "" && triggerSubj != "" {
		basic = append(basic, fmt.Sprintf("%s|%s", triggerNoun, triggerSubj))
	}

	basic = append(basic,
		fmt.Sprintf("Priority|%d", eval.Priority),
		fmt.Sprintf("Placement Failures|%s", failureString))

	if !eval.WaitUntil.IsZero() {
		basic = append(basic,
			fmt.Sprintf("Wait Until|%s", formatTime(eval.WaitUntil)))
	}
	if eval.QuotaLimitReached != "" {
		basic = append(basic,
			fmt.Sprintf("Quota Limit Reached|%s", eval.QuotaLimitReached))
	}

	if verbose {
		// NextEval, PreviousEval, BlockedEval
		basic = append(basic,
			fmt.Sprintf("Previous Eval|%s", eval.PreviousEval),
			fmt.Sprintf("Next Eval|%s", eval.NextEval),
			fmt.Sprintf("Blocked Eval|%s", eval.BlockedEval),
			//fmt.Sprintf("Escaped Computed Class|%v", eval.EscapedComputedClass),
		)

	}
	c.Ui.Output(formatKV(basic))

	if len(eval.RelatedEvals) > 0 {
		c.Ui.Output(c.Colorize().Color("\n[bold]Related Evaluations[reset]"))
		c.Ui.Output(formatRelatedEvalStubs(eval.RelatedEvals, length))
	}

	// if len(eval.ClassEligibility) > 0 {
	// 	c.Ui.Output(c.Colorize().Color("\n[bold]Computed Node Class Eligibility[reset]"))
	// 	classes := make([]string, len(eval.ClassEligibility)+1)
	// 	classes[0] = "Node Class|Eligible"
	// 	i := 1
	// 	for class, ok := range eval.ClassEligibility {
	// 		classes[i] = fmt.Sprintf("%s|%v", class, ok)
	// 	}
	// 	c.Ui.Output(columnize.SimpleFormat(classes))
	// }

	if eval.PlanAnnotations != nil {
		c.Ui.Output(c.Colorize().Color("\n[bold]Reconciler Annotations[reset]"))

		if len(eval.PlanAnnotations.DesiredTGUpdates) > 0 {

			annotations := make([]string, len(eval.PlanAnnotations.DesiredTGUpdates)+1)
			annotations[0] = "Task Group|Ignore|Place|Stop|Migrate|InPlace|Destructive|Canary|Preemptions"
			i := 1
			for tg, updates := range eval.PlanAnnotations.DesiredTGUpdates {
				annotations[i] = fmt.Sprintf("%s|%d|%d|%d|%d|%d|%d|%d|%d",
					tg,
					updates.Ignore,
					updates.Place,
					updates.Stop,
					updates.Migrate,
					updates.InPlaceUpdate,
					updates.DestructiveUpdate,
					updates.Canary,
					updates.Preemptions,
				)
				i++
			}
			c.Ui.Output(columnize.SimpleFormat(annotations))
		}
		if len(eval.PlanAnnotations.PreemptedAllocs) > 0 {
			c.Ui.Output(c.Colorize().Color("\n[bold]Preempted Allocations[reset]"))
			allocsOut := c.formatPreemptedAllocListStubs(eval.PlanAnnotations.PreemptedAllocs, length)
			c.Ui.Output(allocsOut)
		}
	}

	if len(placedAllocs) > 0 {
		c.Ui.Output(c.Colorize().Color("\n[bold]Placed Allocations[reset]"))
		allocsOut := formatAllocListStubs(placedAllocs, false, length)
		c.Ui.Output(allocsOut)
	}

	if failures {
		c.Ui.Output(c.Colorize().Color("\n[bold]Failed Placements[reset]"))
		sorted := sortedTaskGroupFromMetrics(eval.FailedTGAllocs)
		for _, tg := range sorted {
			metrics := eval.FailedTGAllocs[tg]

			noun := "allocation"
			if metrics.CoalescedFailures > 0 {
				noun += "s"
			}
			c.Ui.Output(fmt.Sprintf("Task Group %q (failed to place %d %s):", tg, metrics.CoalescedFailures+1, noun))
			c.Ui.Output(formatAllocMetrics(metrics, false, "  "))
			c.Ui.Output("")
		}

		if eval.BlockedEval != "" {
			c.Ui.Output(fmt.Sprintf("Evaluation %q waiting for additional capacity to place remainder",
				limit(eval.BlockedEval, length)))
		}
	}
}

func (c *EvalStatusCommand) formatPreemptedAllocListStubs(stubs []*api.AllocationListStub, uuidLength int) string {
	allocs := make([]string, len(stubs)+1)
	allocs[0] = "ID|Job ID|Node ID|Task Group|Version|Desired|Status|Created|Modified"
	for i, alloc := range stubs {
		now := time.Now()
		createTimePretty := prettyTimeDiff(time.Unix(0, alloc.CreateTime), now)
		modTimePretty := prettyTimeDiff(time.Unix(0, alloc.ModifyTime), now)
		allocs[i+1] = fmt.Sprintf("%s|%s|%s|%s|%d|%s|%s|%s|%s",
			limit(alloc.ID, uuidLength),
			alloc.JobID,
			limit(alloc.NodeID, uuidLength),
			alloc.TaskGroup,
			alloc.JobVersion,
			alloc.DesiredStatus,
			alloc.ClientStatus,
			createTimePretty,
			modTimePretty)
	}

	return formatList(allocs)
}

func sortedTaskGroupFromMetrics(groups map[string]*api.AllocationMetric) []string {
	tgs := make([]string, 0, len(groups))
	for tg := range groups {
		tgs = append(tgs, tg)
	}
	sort.Strings(tgs)
	return tgs
}

func getTriggerDetails(eval *api.Evaluation) (noun, subject string) {
	switch eval.TriggeredBy {
	case "node-update":
		return "Node ID", eval.NodeID
	case "max-plan-attempts":
		return "Previous Eval", eval.PreviousEval
	default:
		return "", ""
	}
}

func formatRelatedEvalStubs(evals []*api.EvaluationStub, length int) string {
	out := make([]string, len(evals)+1)
	out[0] = "ID|Priority|Triggered By|Node ID|Status|Description"
	for i, eval := range evals {
		out[i+1] = fmt.Sprintf("%s|%d|%s|%s|%s|%s",
			limit(eval.ID, length),
			eval.Priority,
			eval.TriggeredBy,
			limit(eval.NodeID, length),
			eval.Status,
			eval.StatusDescription,
		)
	}

	return formatList(out)
}
