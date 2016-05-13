package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/hashicorp/nomad/scheduler"
	"github.com/mitchellh/colorstring"
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
`
	return strings.TrimSpace(helpText)
}

func (c *PlanCommand) Synopsis() string {
	return "Dry-run a job update to determine its effects"
}

func (c *PlanCommand) Run(args []string) int {
	var diff bool

	flags := c.Meta.FlagSet("plan", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&diff, "diff", true, "")

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
		c.Ui.Output(c.Colorize().Color(formatJobDiff(resp.Diff)))
	}

	return 0
}

func formatJobDiff(job *api.JobDiff) string {
	out := fmt.Sprintf("%s[bold]Job: %q\n", getDiffString(job.Type), job.ID)

	for _, field := range job.Fields {
		out += fmt.Sprintf("%s\n", formatFieldDiff(field, ""))
	}

	for _, object := range job.Objects {
		out += fmt.Sprintf("%s\n", formatObjectDiff(object, ""))
	}

	for _, tg := range job.TaskGroups {
		out += fmt.Sprintf("%s\n", formatTaskGroupDiff(tg))
	}

	return out
}

func formatTaskGroupDiff(tg *api.TaskGroupDiff) string {
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
				color = "[light_yellow]"
			case scheduler.UpdateTypeDestructiveUpdate:
				color = "[yellow]"
			}
			updates = append(updates, fmt.Sprintf("[reset]%s%d %s", color, count, updateType))
		}
		out += fmt.Sprintf(" (%s[reset])\n", strings.Join(updates, ", "))
	} else {
		out += "[reset]\n"
	}

	for _, field := range tg.Fields {
		out += fmt.Sprintf("%s\n", formatFieldDiff(field, "  "))
	}

	for _, object := range tg.Objects {
		out += fmt.Sprintf("%s\n", formatObjectDiff(object, "  "))
	}

	for _, task := range tg.Tasks {
		out += fmt.Sprintf("%s\n", formatTaskDiff(task))
	}

	return out
}

func formatTaskDiff(task *api.TaskDiff) string {
	out := fmt.Sprintf("  %s[bold]Task: %q", getDiffString(task.Type), task.Name)
	if len(task.Annotations) != 0 {
		out += fmt.Sprintf(" [reset](%s)\n", strings.Join(task.Annotations, ", "))
	} else {
		out += "\n"
	}

	if task.Type != "Edited" {
		return out
	}

	for _, field := range task.Fields {
		out += fmt.Sprintf("%s\n", formatFieldDiff(field, "    "))
	}

	for _, object := range task.Objects {
		out += fmt.Sprintf("%s\n", formatObjectDiff(object, "    "))
	}

	return out
}

func formatFieldDiff(diff *api.FieldDiff, prefix string) string {
	switch diff.Type {
	case "Added":
		return fmt.Sprintf("%s%s%s: %q", prefix, getDiffString(diff.Type), diff.Name, diff.New)
	case "Deleted":
		return fmt.Sprintf("%s%s%s: %q", prefix, getDiffString(diff.Type), diff.Name, diff.Old)
	case "Edited":
		return fmt.Sprintf("%s%s%s: %q => %q", prefix, getDiffString(diff.Type), diff.Name, diff.Old, diff.New)
	default:
		return fmt.Sprintf("%s%s: %q", prefix, diff.Name, diff.New)
	}
}

func formatObjectDiff(diff *api.ObjectDiff, prefix string) string {
	diffChar := getDiffString(diff.Type)
	out := fmt.Sprintf("%s%s%s {\n", prefix, diffChar, diff.Name)

	newPrefix := prefix + "  "
	numFields := len(diff.Fields)
	numObjects := len(diff.Objects)
	haveObjects := numObjects != 0
	for i, field := range diff.Fields {
		out += formatFieldDiff(field, newPrefix)
		if i+1 != numFields || haveObjects {
			out += "\n"
		}
	}

	for i, object := range diff.Objects {
		out += formatObjectDiff(object, newPrefix)
		if i+1 != numObjects {
			out += "\n"
		}
	}

	return fmt.Sprintf("%s\n%s%s}", out, prefix, diffChar)
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
