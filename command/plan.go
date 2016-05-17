package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/mitchellh/colorstring"
)

const (
	jobModifyIndexHelp = `To submit the job with version verification run:

nomad run -verify %d %s

When running the job with the verify flag, the job will only be run if the
server side version matches the the job modify index returned. If the index has
changed, another user has modified the job and the plan's results are
potentially invalid.`
)

type PlanCommand struct {
	Meta
	color *colorstring.Colorize
}

func (c *PlanCommand) Help() string {
	helpText := `
Usage: nomad plan [options] <file>


General Options:

  ` + generalOptionsUsage() + `

Run Options:

  -diff
    Defaults to true, but can be toggled off to omit diff output.

  -no-color
    Disable colored output.

  -verbose
    Increased diff verbosity

`
	return strings.TrimSpace(helpText)
}

func (c *PlanCommand) Synopsis() string {
	return "Dry-run a job update to determine its effects"
}

func (c *PlanCommand) Run(args []string) int {
	var diff, verbose bool

	flags := c.Meta.FlagSet("plan", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&diff, "diff", true, "")
	flags.BoolVar(&verbose, "verbose", false, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one job
	args = flags.Args()
	if len(args) != 1 {
		c.Ui.Error(c.Help())
		return 1
	}
	file := args[0]

	// Parse the job file
	job, err := jobspec.ParseFile(file)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing job file %s: %s", file, err))
		return 1
	}

	// Initialize any fields that need to be.
	job.InitFields()

	// Check that the job is valid
	if err := job.Validate(); err != nil {
		c.Ui.Error(fmt.Sprintf("Error validating job: %s", err))
		return 1
	}

	// Convert it to something we can use
	apiJob, err := convertStructJob(job)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error converting job: %s", err))
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Submit the job
	resp, _, err := client.Jobs().Plan(apiJob, diff, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error during plan: %s", err))
		return 1
	}

	if diff {
		c.Ui.Output(fmt.Sprintf("%s\n",
			c.Colorize().Color(strings.TrimSpace(formatJobDiff(resp.Diff, verbose)))))
	}

	c.Ui.Output(c.Colorize().Color("[bold]Scheduler dry-run:[reset]"))
	c.Ui.Output(c.Colorize().Color(formatDryRun(resp.CreatedEvals)))

	c.Ui.Output(c.Colorize().Color(formatJobModifyIndex(resp.JobModifyIndex, file)))
	return 0
}

func formatJobModifyIndex(jobModifyIndex uint64, jobName string) string {
	help := fmt.Sprintf(jobModifyIndexHelp, jobModifyIndex, jobName)
	out := fmt.Sprintf("[reset][bold]Job Modify Index: %d[reset]\n%s", jobModifyIndex, help)
	return out
}

func formatDryRun(evals []*api.Evaluation) string {
	// "- All tasks successfully allocated." bold and green

	var rolling *api.Evaluation
	var blocked *api.Evaluation
	for _, eval := range evals {
		if eval.TriggeredBy == "rolling-update" {
			rolling = eval
		} else if eval.Status == "blocked" {
			blocked = eval
		}
	}

	var out string
	if blocked == nil {
		out = "[bold][green]  - All tasks successfully allocated.[reset]\n"
	} else {
		out = "[bold][yellow]  - WARNING: Failed to place all allocations.[reset]\n"
	}

	if rolling != nil {
		out += fmt.Sprintf("[green]  - Rolling update, next evaluation will be in %s.\n", rolling.Wait)
	}

	return out
}

func formatJobDiff(job *api.JobDiff, verbose bool) string {
	marker, _ := getDiffString(job.Type)
	out := fmt.Sprintf("%s[bold]Job: %q\n", marker, job.ID)

	longestField, longestMarker := getLongestPrefixes(job.Fields, job.Objects)
	for _, tg := range job.TaskGroups {
		if _, l := getDiffString(tg.Type); l > longestMarker {
			longestMarker = l
		}
	}

	subStartPrefix := ""
	if job.Type == "Edited" || verbose {
		for _, field := range job.Fields {
			_, mLength := getDiffString(field.Type)
			kPrefix := longestMarker - mLength
			vPrefix := longestField - len(field.Name)
			out += fmt.Sprintf("%s\n", formatFieldDiff(
				field,
				subStartPrefix,
				strings.Repeat(" ", kPrefix),
				strings.Repeat(" ", vPrefix)))
		}

		for _, object := range job.Objects {
			_, mLength := getDiffString(object.Type)
			kPrefix := longestMarker - mLength
			out += fmt.Sprintf("%s\n", formatObjectDiff(
				object,
				subStartPrefix,
				strings.Repeat(" ", kPrefix)))
		}
	}

	for _, tg := range job.TaskGroups {
		_, mLength := getDiffString(tg.Type)
		kPrefix := longestMarker - mLength
		out += fmt.Sprintf("%s\n", formatTaskGroupDiff(tg, strings.Repeat(" ", kPrefix), verbose))
	}

	return out
}

func formatTaskGroupDiff(tg *api.TaskGroupDiff, tgPrefix string, verbose bool) string {
	marker, _ := getDiffString(tg.Type)
	out := fmt.Sprintf("%s%s[bold]Task Group: %q[reset]", marker, tgPrefix, tg.Name)

	// Append the updates
	if l := len(tg.Updates); l > 0 {
		updates := make([]string, 0, l)
		for updateType, count := range tg.Updates {
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
			}
			updates = append(updates, fmt.Sprintf("[reset]%s%d %s", color, count, updateType))
		}
		out += fmt.Sprintf(" (%s[reset])\n", strings.Join(updates, ", "))
	} else {
		out += "[reset]\n"
	}

	longestField, longestMarker := getLongestPrefixes(tg.Fields, tg.Objects)
	for _, task := range tg.Tasks {
		if _, l := getDiffString(task.Type); l > longestMarker {
			longestMarker = l
		}
	}

	subStartPrefix := strings.Repeat(" ", len(tgPrefix)+2)
	if tg.Type == "Edited" || verbose {
		for _, field := range tg.Fields {
			_, mLength := getDiffString(field.Type)
			kPrefix := longestMarker - mLength
			vPrefix := longestField - len(field.Name)
			out += fmt.Sprintf("%s\n", formatFieldDiff(
				field,
				subStartPrefix,
				strings.Repeat(" ", kPrefix),
				strings.Repeat(" ", vPrefix)))
		}

		for _, object := range tg.Objects {
			_, mLength := getDiffString(object.Type)
			kPrefix := longestMarker - mLength
			out += fmt.Sprintf("%s\n", formatObjectDiff(
				object,
				subStartPrefix,
				strings.Repeat(" ", kPrefix)))
		}
	}

	for _, task := range tg.Tasks {
		_, mLength := getDiffString(task.Type)
		prefix := strings.Repeat(" ", (longestMarker - mLength))
		out += fmt.Sprintf("%s\n", formatTaskDiff(task, subStartPrefix, prefix, verbose))
	}

	return out
}

func formatTaskDiff(task *api.TaskDiff, startPrefix, taskPrefix string, verbose bool) string {
	marker, _ := getDiffString(task.Type)
	out := fmt.Sprintf("%s%s%s[bold]Task: %q", startPrefix, marker, taskPrefix, task.Name)
	if len(task.Annotations) != 0 {
		out += fmt.Sprintf(" [reset](%s)", colorAnnotations(task.Annotations))
	}

	if task.Type == "None" {
		return out
	} else if (task.Type == "Deleted" || task.Type == "Added") && !verbose {
		return out
	} else {
		out += "\n"
	}

	subStartPrefix := strings.Repeat(" ", len(startPrefix)+2)
	longestField, longestMarker := getLongestPrefixes(task.Fields, task.Objects)
	for _, field := range task.Fields {
		_, mLength := getDiffString(field.Type)
		kPrefix := longestMarker - mLength
		vPrefix := longestField - len(field.Name)
		out += fmt.Sprintf("%s\n", formatFieldDiff(
			field,
			subStartPrefix,
			strings.Repeat(" ", kPrefix),
			strings.Repeat(" ", vPrefix)))
	}

	for _, object := range task.Objects {
		_, mLength := getDiffString(object.Type)
		kPrefix := longestMarker - mLength
		out += fmt.Sprintf("%s\n", formatObjectDiff(
			object,
			subStartPrefix,
			strings.Repeat(" ", kPrefix)))
	}

	return out
}

func formatFieldDiff(diff *api.FieldDiff, startPrefix, keyPrefix, valuePrefix string) string {
	marker, _ := getDiffString(diff.Type)
	out := fmt.Sprintf("%s%s%s%s: %s", startPrefix, marker, keyPrefix, diff.Name, valuePrefix)
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

func formatObjectDiff(diff *api.ObjectDiff, startPrefix, keyPrefix string) string {
	marker, _ := getDiffString(diff.Type)
	out := fmt.Sprintf("%s%s%s%s {\n", startPrefix, marker, keyPrefix, diff.Name)

	// Determine the length of the longest name and longest diff marker to
	// properly align names and values
	longestField, longestMarker := getLongestPrefixes(diff.Fields, diff.Objects)
	subStartPrefix := strings.Repeat(" ", len(startPrefix)+2)
	numFields := len(diff.Fields)
	numObjects := len(diff.Objects)
	haveObjects := numObjects != 0
	for i, field := range diff.Fields {
		_, mLength := getDiffString(field.Type)
		kPrefix := longestMarker - mLength
		vPrefix := longestField - len(field.Name)
		out += formatFieldDiff(
			field,
			subStartPrefix,
			strings.Repeat(" ", kPrefix),
			strings.Repeat(" ", vPrefix))

		// Avoid a dangling new line
		if i+1 != numFields || haveObjects {
			out += "\n"
		}
	}

	for i, object := range diff.Objects {
		_, mLength := getDiffString(object.Type)
		kPrefix := longestMarker - mLength
		out += formatObjectDiff(object, subStartPrefix, strings.Repeat(" ", kPrefix))

		// Avoid a dangling new line
		if i+1 != numObjects {
			out += "\n"
		}
	}

	return fmt.Sprintf("%s\n%s}", out, startPrefix)
}

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
