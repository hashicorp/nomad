// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper/pointer"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/posener/complete"
)

const (
	jobModifyIndexHelp = `To submit the job with version verification run:

nomad job run -check-index %d %s%s

When running the job with the check-index flag, the job will only be run if the
job modify index given matches the server-side version. If the index has
changed, another user has modified the job and the plan's results are
potentially invalid.`

	// preemptionDisplayThreshold is an upper bound used to limit and summarize
	// the details of preempted jobs in the output
	preemptionDisplayThreshold = 10
)

type JobPlanCommand struct {
	Meta
	JobGetter
}

func (c *JobPlanCommand) Help() string {
	helpText := `
Usage: nomad job plan [options] <path>
Alias: nomad plan

  Plan invokes a dry-run of the scheduler to determine the effects of submitting
  either a new or updated version of a job. The plan will not result in any
  changes to the cluster but gives insight into whether the job could be run
  successfully and how it would affect existing allocations.

  If the supplied path is "-", the jobfile is read from stdin. Otherwise
  it is read from the file at the supplied path or downloaded and
  read from URL specified.

  A job modify index is returned with the plan. This value can be used when
  submitting the job using "nomad run -check-index", which will check that the job
  was not modified between the plan and run command before invoking the
  scheduler. This ensures the job has not been modified since the plan.
  Multiregion jobs do not return a job modify index.

  A structured diff between the local and remote job is displayed to
  give insight into what the scheduler will attempt to do and why.

  If the job has specified the region, the -region flag and NOMAD_REGION
  environment variable are overridden and the job's region is used.

  Plan will return one of the following exit codes:
    * 0: No allocations created or destroyed.
    * 1: Allocations created or destroyed.
    * 255: Error determining plan results.

  The plan command will set the vault_token of the job based on the following
  precedence, going from highest to lowest: the -vault-token flag, the
  $VAULT_TOKEN environment variable and finally the value in the job file.

  When ACLs are enabled, this command requires a token with the 'submit-job'
  capability for the job's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Plan Options:

  -diff
    Determines whether the diff between the remote job and planned job is shown.
    Defaults to true.

  -json
    Parses the job file as JSON. If the outer object has a Job field, such as
    from "nomad job inspect" or "nomad run -output", the value of the field is
    used as the job.

  -hcl1
    Parses the job file as HCLv1. Takes precedence over "-hcl2-strict".

  -hcl2-strict
    Whether an error should be produced from the HCL2 parser where a variable
    has been supplied which is not defined within the root variables. Defaults
    to true, but ignored if "-hcl1" is also defined.

  -policy-override
    Sets the flag to force override any soft mandatory Sentinel policies.

  -vault-token
    Used to validate if the user submitting the job has permission to run the job
    according to its Vault policies. A Vault token must be supplied if the vault
    block allow_unauthenticated is disabled in the Nomad server configuration.
    If the -vault-token flag is set, the passed Vault token is added to the jobspec
    before sending to the Nomad servers. This allows passing the Vault token
    without storing it in the job file. This overrides the token found in the
    $VAULT_TOKEN environment variable and the vault_token field in the job file.
    This token is cleared from the job after validating and cannot be used within
    the job executing environment. Use the vault block when templating in a job
    with a Vault token.

  -vault-namespace
    If set, the passed Vault namespace is stored in the job before sending to the
    Nomad servers.

  -var 'key=value'
    Variable for template, can be used multiple times.

  -var-file=path
    Path to HCL2 file containing user variables.

  -verbose
    Increase diff verbosity.
`
	return strings.TrimSpace(helpText)
}

func (c *JobPlanCommand) Synopsis() string {
	return "Dry-run a job update to determine its effects"
}

func (c *JobPlanCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-diff":            complete.PredictNothing,
			"-policy-override": complete.PredictNothing,
			"-verbose":         complete.PredictNothing,
			"-json":            complete.PredictNothing,
			"-hcl1":            complete.PredictNothing,
			"-hcl2-strict":     complete.PredictNothing,
			"-vault-token":     complete.PredictAnything,
			"-vault-namespace": complete.PredictAnything,
			"-var":             complete.PredictAnything,
			"-var-file":        complete.PredictFiles("*.var"),
		})
}

func (c *JobPlanCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictOr(
		complete.PredictFiles("*.nomad"),
		complete.PredictFiles("*.hcl"),
		complete.PredictFiles("*.json"),
	)
}

func (c *JobPlanCommand) Name() string { return "job plan" }
func (c *JobPlanCommand) Run(args []string) int {
	var diff, policyOverride, verbose bool
	var vaultToken, vaultNamespace string

	flagSet := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flagSet.Usage = func() { c.Ui.Output(c.Help()) }
	flagSet.BoolVar(&diff, "diff", true, "")
	flagSet.BoolVar(&policyOverride, "policy-override", false, "")
	flagSet.BoolVar(&verbose, "verbose", false, "")
	flagSet.BoolVar(&c.JobGetter.JSON, "json", false, "")
	flagSet.BoolVar(&c.JobGetter.HCL1, "hcl1", false, "")
	flagSet.BoolVar(&c.JobGetter.Strict, "hcl2-strict", true, "")
	flagSet.StringVar(&vaultToken, "vault-token", "", "")
	flagSet.StringVar(&vaultNamespace, "vault-namespace", "", "")
	flagSet.Var(&c.JobGetter.Vars, "var", "")
	flagSet.Var(&c.JobGetter.VarFiles, "var-file", "")

	if err := flagSet.Parse(args); err != nil {
		return 255
	}

	// Check that we got exactly one job
	args = flagSet.Args()
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <path>")
		c.Ui.Error(commandErrorText(c))
		return 255
	}

	if c.JobGetter.HCL1 {
		c.JobGetter.Strict = false
	}

	if err := c.JobGetter.Validate(); err != nil {
		c.Ui.Error(fmt.Sprintf("Invalid job options: %s", err))
		return 255
	}

	path := args[0]
	// Get Job struct from Jobfile
	_, job, err := c.JobGetter.Get(path)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error getting job struct: %s", err))
		return 255
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 255
	}

	// Force the region to be that of the job.
	if r := job.Region; r != nil {
		client.SetRegion(*r)
	}

	// Force the namespace to be that of the job.
	if n := job.Namespace; n != nil {
		client.SetNamespace(*n)
	}

	// Parse the Vault token.
	if vaultToken == "" {
		// Check the environment variable
		vaultToken = os.Getenv("VAULT_TOKEN")
	}

	// Set the vault token.
	if vaultToken != "" {
		job.VaultToken = pointer.Of(vaultToken)
	}

	//  Set the vault namespace.
	if vaultNamespace != "" {
		job.VaultNamespace = pointer.Of(vaultNamespace)
	}

	// Setup the options
	opts := &api.PlanOptions{
		// Always request the diff so we can tell if there are changes.
		Diff: true,
	}
	if policyOverride {
		opts.PolicyOverride = true
	}

	if job.IsMultiregion() {
		return c.multiregionPlan(client, job, opts, diff, verbose)
	}

	// Submit the job
	resp, _, err := client.Jobs().PlanOpts(job, opts, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error during plan: %s", err))
		return 255
	}

	runArgs := strings.Builder{}
	for _, varArg := range c.JobGetter.Vars {
		runArgs.WriteString(fmt.Sprintf("-var=%q ", varArg))
	}

	for _, varFile := range c.JobGetter.VarFiles {
		runArgs.WriteString(fmt.Sprintf("-var-file=%q ", varFile))
	}

	if c.namespace != "" {
		runArgs.WriteString(fmt.Sprintf("-namespace=%q ", c.namespace))
	}

	exitCode := c.outputPlannedJob(job, resp, diff, verbose)
	c.Ui.Output(c.Colorize().Color(formatJobModifyIndex(resp.JobModifyIndex, runArgs.String(), path)))
	return exitCode
}

func (c *JobPlanCommand) multiregionPlan(client *api.Client, job *api.Job, opts *api.PlanOptions, diff, verbose bool) int {

	var exitCode int
	plans := map[string]*api.JobPlanResponse{}

	// collect all the plans first so that we can report all errors
	for _, region := range job.Multiregion.Regions {
		regionName := region.Name
		client.SetRegion(regionName)

		// Submit the job for this region
		resp, _, err := client.Jobs().PlanOpts(job, opts, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error during plan for region %q: %s", regionName, err))
			exitCode = 255
		}
		plans[regionName] = resp
	}

	if exitCode > 0 {
		return exitCode
	}

	for regionName, resp := range plans {
		c.Ui.Output(c.Colorize().Color(fmt.Sprintf("[bold]Region: %q[reset]", regionName)))
		regionExitCode := c.outputPlannedJob(job, resp, diff, verbose)
		if regionExitCode > exitCode {
			exitCode = regionExitCode
		}
	}
	return exitCode
}

func (c *JobPlanCommand) outputPlannedJob(job *api.Job, resp *api.JobPlanResponse, diff, verbose bool) int {

	// Print the diff if not disabled
	if diff {
		c.Ui.Output(fmt.Sprintf("%s\n",
			c.Colorize().Color(strings.TrimSpace(formatJobDiff(resp.Diff, verbose)))))
	}

	// Print the scheduler dry-run output
	c.Ui.Output(c.Colorize().Color("[bold]Scheduler dry-run:[reset]"))
	c.Ui.Output(c.Colorize().Color(formatDryRun(resp, job)))
	c.Ui.Output("")

	// Print any warnings if there are any
	if resp.Warnings != "" {
		c.Ui.Output(
			c.Colorize().Color(fmt.Sprintf("[bold][yellow]Job Warnings:\n%s[reset]\n", resp.Warnings)))
	}

	// Print preemptions if there are any
	if resp.Annotations != nil && len(resp.Annotations.PreemptedAllocs) > 0 {
		c.addPreemptions(resp)
	}

	return getExitCode(resp)
}

// addPreemptions shows details about preempted allocations
func (c *JobPlanCommand) addPreemptions(resp *api.JobPlanResponse) {
	c.Ui.Output(c.Colorize().Color("[bold][yellow]Preemptions:\n[reset]"))
	if len(resp.Annotations.PreemptedAllocs) < preemptionDisplayThreshold {
		var allocs []string
		allocs = append(allocs, "Alloc ID|Job ID|Task Group")
		for _, alloc := range resp.Annotations.PreemptedAllocs {
			allocs = append(allocs, fmt.Sprintf("%s|%s|%s", alloc.ID, alloc.JobID, alloc.TaskGroup))
		}
		c.Ui.Output(formatList(allocs))
		return
	}
	// Display in a summary format if the list is too large
	// Group by job type and job ids
	allocDetails := make(map[string]map[namespaceIdPair]int)
	numJobs := 0
	for _, alloc := range resp.Annotations.PreemptedAllocs {
		id := namespaceIdPair{alloc.JobID, alloc.Namespace}
		countMap := allocDetails[alloc.JobType]
		if countMap == nil {
			countMap = make(map[namespaceIdPair]int)
		}
		cnt, ok := countMap[id]
		if !ok {
			// First time we are seeing this job, increment counter
			numJobs++
		}
		countMap[id] = cnt + 1
		allocDetails[alloc.JobType] = countMap
	}

	// Show counts grouped by job ID if its less than a threshold
	var output []string
	if numJobs < preemptionDisplayThreshold {
		output = append(output, "Job ID|Namespace|Job Type|Preemptions")
		for jobType, jobCounts := range allocDetails {
			for jobId, count := range jobCounts {
				output = append(output, fmt.Sprintf("%s|%s|%s|%d", jobId.id, jobId.namespace, jobType, count))
			}
		}
	} else {
		// Show counts grouped by job type
		output = append(output, "Job Type|Preemptions")
		for jobType, jobCounts := range allocDetails {
			total := 0
			for _, count := range jobCounts {
				total += count
			}
			output = append(output, fmt.Sprintf("%s|%d", jobType, total))
		}
	}
	c.Ui.Output(formatList(output))

}

type namespaceIdPair struct {
	id        string
	namespace string
}

// getExitCode returns 0:
// * 0: No allocations created or destroyed.
// * 1: Allocations created or destroyed.
func getExitCode(resp *api.JobPlanResponse) int {
	if resp.Diff.Type == "None" {
		return 0
	}

	// Check for changes
	for _, d := range resp.Annotations.DesiredTGUpdates {
		if d.Stop+d.Place+d.Migrate+d.DestructiveUpdate+d.Canary > 0 {
			return 1
		}
	}

	return 0
}

// formatJobModifyIndex produces a help string that displays the job modify
// index and how to submit a job with it.
func formatJobModifyIndex(jobModifyIndex uint64, args string, jobName string) string {
	help := fmt.Sprintf(jobModifyIndexHelp, jobModifyIndex, args, jobName)
	out := fmt.Sprintf("[reset][bold]Job Modify Index: %d[reset]\n%s", jobModifyIndex, help)
	return out
}

// formatDryRun produces a string explaining the results of the dry run.
func formatDryRun(resp *api.JobPlanResponse, job *api.Job) string {
	var rolling *api.Evaluation
	for _, eval := range resp.CreatedEvals {
		if eval.TriggeredBy == "rolling-update" {
			rolling = eval
		}
	}

	var out string
	if len(resp.FailedTGAllocs) == 0 {
		out = "[bold][green]- All tasks successfully allocated.[reset]\n"
	} else {
		// Change the output depending on if we are a system job or not
		if job.Type != nil && *job.Type == "system" {
			out = "[bold][yellow]- WARNING: Failed to place allocations on all nodes.[reset]\n"
		} else {
			out = "[bold][yellow]- WARNING: Failed to place all allocations.[reset]\n"
		}
		sorted := sortedTaskGroupFromMetrics(resp.FailedTGAllocs)
		for _, tg := range sorted {
			metrics := resp.FailedTGAllocs[tg]

			noun := "allocation"
			if metrics.CoalescedFailures > 0 {
				noun += "s"
			}
			out += fmt.Sprintf("%s[yellow]Task Group %q (failed to place %d %s):\n[reset]", strings.Repeat(" ", 2), tg, metrics.CoalescedFailures+1, noun)
			out += fmt.Sprintf("[yellow]%s[reset]\n\n", formatAllocMetrics(metrics, false, strings.Repeat(" ", 4)))
		}
		if rolling == nil {
			out = strings.TrimSuffix(out, "\n")
		}
	}

	if rolling != nil {
		out += fmt.Sprintf("[green]- Rolling update, next evaluation will be in %s.\n", rolling.Wait)
	}

	if next := resp.NextPeriodicLaunch; !next.IsZero() && !job.IsParameterized() {
		loc, err := job.Periodic.GetLocation()
		if err != nil {
			out += fmt.Sprintf("[yellow]- Invalid time zone: %v", err)
		} else {
			now := time.Now().In(loc)
			out += fmt.Sprintf("[green]- If submitted now, next periodic launch would be at %s (%s from now).\n",
				formatTime(next), formatTimeDifference(now, next, time.Second))
		}
	}

	out = strings.TrimSuffix(out, "\n")
	return out
}

// formatJobDiff produces an annotated diff of the job. If verbose mode is
// set, added or deleted task groups and tasks are expanded.
func formatJobDiff(job *api.JobDiff, verbose bool) string {
	marker, _ := getDiffString(job.Type)
	out := fmt.Sprintf("%s[bold]Job: %q\n", marker, job.ID)

	// Determine the longest markers and fields so that the output can be
	// properly aligned.
	longestField, longestMarker := getLongestPrefixes(job.Fields, job.Objects)
	for _, tg := range job.TaskGroups {
		if _, l := getDiffString(tg.Type); l > longestMarker {
			longestMarker = l
		}
	}

	// Only show the job's field and object diffs if the job is edited or
	// verbose mode is set.
	if job.Type == "Edited" || verbose {
		fo := alignedFieldAndObjects(job.Fields, job.Objects, 0, longestField, longestMarker)
		out += fo
		if len(fo) > 0 {
			out += "\n"
		}
	}

	// Print the task groups
	for _, tg := range job.TaskGroups {
		_, mLength := getDiffString(tg.Type)
		kPrefix := longestMarker - mLength
		out += fmt.Sprintf("%s\n", formatTaskGroupDiff(tg, kPrefix, verbose))
	}

	return out
}

// formatTaskGroupDiff produces an annotated diff of a task group. If the
// verbose field is set, the task groups fields and objects are expanded even if
// the full object is an addition or removal. tgPrefix is the number of spaces to prefix
// the output of the task group.
func formatTaskGroupDiff(tg *api.TaskGroupDiff, tgPrefix int, verbose bool) string {
	marker, _ := getDiffString(tg.Type)
	out := fmt.Sprintf("%s%s[bold]Task Group: %q[reset]", marker, strings.Repeat(" ", tgPrefix), tg.Name)

	// Append the updates and colorize them
	if l := len(tg.Updates); l > 0 {
		order := make([]string, 0, l)
		for updateType := range tg.Updates {
			order = append(order, updateType)
		}

		sort.Strings(order)
		updates := make([]string, 0, l)
		for _, updateType := range order {
			count := tg.Updates[updateType]
			var color string
			switch updateType {
			case scheduler.UpdateTypeIgnore:
			case scheduler.UpdateTypeCreate:
				color = "[green]"
			case scheduler.UpdateTypeDestroy:
				color = "[red]"
			case scheduler.UpdateTypeMigrate:
				color = "[blue]"
			case scheduler.UpdateTypeInplaceUpdate:
				color = "[cyan]"
			case scheduler.UpdateTypeDestructiveUpdate:
				color = "[yellow]"
			case scheduler.UpdateTypeCanary:
				color = "[light_yellow]"
			}
			updates = append(updates, fmt.Sprintf("[reset]%s%d %s", color, count, updateType))
		}
		out += fmt.Sprintf(" (%s[reset])\n", strings.Join(updates, ", "))
	} else {
		out += "[reset]\n"
	}

	// Determine the longest field and markers so the output is properly
	// aligned
	longestField, longestMarker := getLongestPrefixes(tg.Fields, tg.Objects)
	for _, task := range tg.Tasks {
		if _, l := getDiffString(task.Type); l > longestMarker {
			longestMarker = l
		}
	}

	// Only show the task groups's field and object diffs if the group is edited or
	// verbose mode is set.
	subStartPrefix := tgPrefix + 2
	if tg.Type == "Edited" || verbose {
		fo := alignedFieldAndObjects(tg.Fields, tg.Objects, subStartPrefix, longestField, longestMarker)
		out += fo
		if len(fo) > 0 {
			out += "\n"
		}
	}

	// Output the tasks
	for _, task := range tg.Tasks {
		_, mLength := getDiffString(task.Type)
		prefix := longestMarker - mLength
		out += fmt.Sprintf("%s\n", formatTaskDiff(task, subStartPrefix, prefix, verbose))
	}

	return out
}

// formatTaskDiff produces an annotated diff of a task. If the verbose field is
// set, the tasks fields and objects are expanded even if the full object is an
// addition or removal. startPrefix is the number of spaces to prefix the output of
// the task and taskPrefix is the number of spaces to put between the marker and
// task name output.
func formatTaskDiff(task *api.TaskDiff, startPrefix, taskPrefix int, verbose bool) string {
	marker, _ := getDiffString(task.Type)
	out := fmt.Sprintf("%s%s%s[bold]Task: %q",
		strings.Repeat(" ", startPrefix), marker, strings.Repeat(" ", taskPrefix), task.Name)
	if len(task.Annotations) != 0 {
		out += fmt.Sprintf(" [reset](%s)", colorAnnotations(task.Annotations))
	}

	if task.Type == "None" {
		return out
	} else if (task.Type == "Deleted" || task.Type == "Added") && !verbose {
		// Exit early if the job was not edited and it isn't verbose output
		return out
	} else {
		out += "\n"
	}

	subStartPrefix := startPrefix + 2
	longestField, longestMarker := getLongestPrefixes(task.Fields, task.Objects)
	out += alignedFieldAndObjects(task.Fields, task.Objects, subStartPrefix, longestField, longestMarker)
	return out
}

// formatObjectDiff produces an annotated diff of an object. startPrefix is the
// number of spaces to prefix the output of the object and keyPrefix is the number
// of spaces to put between the marker and object name output.
func formatObjectDiff(diff *api.ObjectDiff, startPrefix, keyPrefix int) string {
	start := strings.Repeat(" ", startPrefix)
	marker, markerLen := getDiffString(diff.Type)
	out := fmt.Sprintf("%s%s%s%s {\n", start, marker, strings.Repeat(" ", keyPrefix), diff.Name)

	// Determine the length of the longest name and longest diff marker to
	// properly align names and values
	longestField, longestMarker := getLongestPrefixes(diff.Fields, diff.Objects)
	subStartPrefix := startPrefix + keyPrefix + 2
	out += alignedFieldAndObjects(diff.Fields, diff.Objects, subStartPrefix, longestField, longestMarker)

	endprefix := strings.Repeat(" ", startPrefix+markerLen+keyPrefix)
	return fmt.Sprintf("%s\n%s}", out, endprefix)
}

// formatFieldDiff produces an annotated diff of a field. startPrefix is the
// number of spaces to prefix the output of the field, keyPrefix is the number
// of spaces to put between the marker and field name output and valuePrefix is
// the number of spaces to put infront of the value for aligning values.
func formatFieldDiff(diff *api.FieldDiff, startPrefix, keyPrefix, valuePrefix int) string {
	marker, _ := getDiffString(diff.Type)
	out := fmt.Sprintf("%s%s%s%s: %s",
		strings.Repeat(" ", startPrefix),
		marker, strings.Repeat(" ", keyPrefix),
		diff.Name,
		strings.Repeat(" ", valuePrefix))

	switch diff.Type {
	case "Added":
		out += fmt.Sprintf("%q", diff.New)
	case "Deleted":
		out += fmt.Sprintf("%q", diff.Old)
	case "Edited":
		out += fmt.Sprintf("%q => %q", diff.Old, diff.New)
	default:
		out += fmt.Sprintf("%q", diff.New)
	}

	// Color the annotations where possible
	if l := len(diff.Annotations); l != 0 {
		out += fmt.Sprintf(" (%s)", colorAnnotations(diff.Annotations))
	}

	return out
}

// alignedFieldAndObjects is a helper method that prints fields and objects
// properly aligned.
func alignedFieldAndObjects(fields []*api.FieldDiff, objects []*api.ObjectDiff,
	startPrefix, longestField, longestMarker int) string {

	var out string
	numFields := len(fields)
	numObjects := len(objects)
	haveObjects := numObjects != 0
	for i, field := range fields {
		_, mLength := getDiffString(field.Type)
		kPrefix := longestMarker - mLength
		vPrefix := longestField - len(field.Name)
		out += formatFieldDiff(field, startPrefix, kPrefix, vPrefix)

		// Avoid a dangling new line
		if i+1 != numFields || haveObjects {
			out += "\n"
		}
	}

	for i, object := range objects {
		_, mLength := getDiffString(object.Type)
		kPrefix := longestMarker - mLength
		out += formatObjectDiff(object, startPrefix, kPrefix)

		// Avoid a dangling new line
		if i+1 != numObjects {
			out += "\n"
		}
	}

	return out
}

// getLongestPrefixes takes a list  of fields and objects and determines the
// longest field name and the longest marker.
func getLongestPrefixes(fields []*api.FieldDiff, objects []*api.ObjectDiff) (longestField, longestMarker int) {
	for _, field := range fields {
		if l := len(field.Name); l > longestField {
			longestField = l
		}
		if _, l := getDiffString(field.Type); l > longestMarker {
			longestMarker = l
		}
	}
	for _, obj := range objects {
		if _, l := getDiffString(obj.Type); l > longestMarker {
			longestMarker = l
		}
	}
	return longestField, longestMarker
}

// getDiffString returns a colored diff marker and the length of the string
// without color annotations.
func getDiffString(diffType string) (string, int) {
	switch diffType {
	case "Added":
		return "[green]+[reset] ", 2
	case "Deleted":
		return "[red]-[reset] ", 2
	case "Edited":
		return "[light_yellow]+/-[reset] ", 4
	default:
		return "", 0
	}
}

// colorAnnotations returns a comma concatenated list of the annotations where
// the annotations are colored where possible.
func colorAnnotations(annotations []string) string {
	l := len(annotations)
	if l == 0 {
		return ""
	}

	colored := make([]string, l)
	for i, annotation := range annotations {
		switch annotation {
		case "forces create":
			colored[i] = fmt.Sprintf("[green]%s[reset]", annotation)
		case "forces destroy":
			colored[i] = fmt.Sprintf("[red]%s[reset]", annotation)
		case "forces in-place update":
			colored[i] = fmt.Sprintf("[cyan]%s[reset]", annotation)
		case "forces create/destroy update":
			colored[i] = fmt.Sprintf("[yellow]%s[reset]", annotation)
		default:
			colored[i] = annotation
		}
	}

	return strings.Join(colored, ", ")
}
