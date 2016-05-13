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
	casHelp = `To submit the job with version verification run:

nomad run -verify %d %s

When running the job with the verify flag, the job will only be run if the server side
version matches the the verify index returned. If the index has changed, another user has
modified the job and the plan's results are potentially invalid.`
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

	c.Ui.Output(c.Colorize().Color(formatCas(resp.Cas, file)))
	return 0
}

func formatCas(cas uint64, jobName string) string {
	help := fmt.Sprintf(casHelp, cas, jobName)
	out := fmt.Sprintf("[reset][bold]Job Verify Index: %d[reset]\n%s", cas, help)
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
	out := fmt.Sprintf("%s[bold]Job: %q\n", getDiffString(job.Type), job.ID)

	if job.Type == "Edited" || verbose {
		for _, field := range job.Fields {
			out += fmt.Sprintf("%s\n", formatFieldDiff(field, "", verbose))
		}

		for _, object := range job.Objects {
			out += fmt.Sprintf("%s\n", formatObjectDiff(object, "", verbose))
		}
	}

	for _, tg := range job.TaskGroups {
		out += fmt.Sprintf("%s\n", formatTaskGroupDiff(tg, verbose))
	}

	return out
}

func formatTaskGroupDiff(tg *api.TaskGroupDiff, verbose bool) string {
	out := fmt.Sprintf("%s[bold]Task Group: %q", getDiffString(tg.Type), tg.Name)

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

	if tg.Type == "Edited" || verbose {
		for _, field := range tg.Fields {
			out += fmt.Sprintf("%s\n", formatFieldDiff(field, "  ", verbose))
		}

		for _, object := range tg.Objects {
			out += fmt.Sprintf("%s\n", formatObjectDiff(object, "  ", verbose))
		}
	}

	for _, task := range tg.Tasks {
		out += fmt.Sprintf("%s\n", formatTaskDiff(task, verbose))
	}

	return out
}

func formatTaskDiff(task *api.TaskDiff, verbose bool) string {
	out := fmt.Sprintf("  %s[bold]Task: %q", getDiffString(task.Type), task.Name)
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

	for _, field := range task.Fields {
		out += fmt.Sprintf("%s\n", formatFieldDiff(field, "    ", verbose))
	}

	for _, object := range task.Objects {
		out += fmt.Sprintf("%s\n", formatObjectDiff(object, "    ", verbose))
	}

	return out
}

func formatFieldDiff(diff *api.FieldDiff, prefix string, verbose bool) string {
	out := prefix
	switch diff.Type {
	case "Added":
		out += fmt.Sprintf("%s%s: %q", getDiffString(diff.Type), diff.Name, diff.New)
	case "Deleted":
		out += fmt.Sprintf("%s%s: %q", getDiffString(diff.Type), diff.Name, diff.Old)
	case "Edited":
		out += fmt.Sprintf("%s%s: %q => %q", getDiffString(diff.Type), diff.Name, diff.Old, diff.New)
	default:
		out += fmt.Sprintf("%s: %q", diff.Name, diff.New)
	}

	// Color the annotations where possible
	if l := len(diff.Annotations); l != 0 {
		out += fmt.Sprintf(" (%s)", colorAnnotations(diff.Annotations))
	}

	return out
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

func formatObjectDiff(diff *api.ObjectDiff, prefix string, verbose bool) string {
	out := fmt.Sprintf("%s%s%s {\n", prefix, getDiffString(diff.Type), diff.Name)

	newPrefix := prefix + "  "
	numFields := len(diff.Fields)
	numObjects := len(diff.Objects)
	haveObjects := numObjects != 0
	for i, field := range diff.Fields {
		out += formatFieldDiff(field, newPrefix, verbose)
		if i+1 != numFields || haveObjects {
			out += "\n"
		}
	}

	for i, object := range diff.Objects {
		out += formatObjectDiff(object, newPrefix, verbose)
		if i+1 != numObjects {
			out += "\n"
		}
	}

	return fmt.Sprintf("%s\n%s}", out, prefix)
}

func getDiffString(diffType string) string {
	switch diffType {
	case "Added":
		return "[green]+[reset] "
	case "Deleted":
		return "[red]-[reset] "
	case "Edited":
		return "[light_yellow]+/-[reset] "
	default:
		return ""
	}
}
