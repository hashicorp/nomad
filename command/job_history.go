package command

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
	"github.com/ryanuber/columnize"
)

type JobHistoryCommand struct {
	Meta
	formatter DataFormatter
}

func (c *JobHistoryCommand) Help() string {
	helpText := `
Usage: nomad job history [options] <job>

  History is used to display the known versions of a particular job. The command
  can display the diff between job versions and can be useful for understanding
  the changes that occurred to the job as well as deciding job versions to revert
  to.

General Options:

  ` + generalOptionsUsage() + `

History Options:

  -p
    Display the difference between each job and its predecessor.

  -full
    Display the full job definition for each version.

  -version <job version>
    Display only the history for the given job version.

  -json
    Output the job versions in a JSON format.

  -t
    Format and display the job versions using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *JobHistoryCommand) Synopsis() string {
	return "Display all tracked versions of a job"
}

func (c *JobHistoryCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-p":       complete.PredictNothing,
			"-full":    complete.PredictNothing,
			"-version": complete.PredictAnything,
			"-json":    complete.PredictNothing,
			"-t":       complete.PredictAnything,
		})
}

func (c *JobHistoryCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := c.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Jobs, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Jobs]
	})
}

func (c *JobHistoryCommand) Name() string { return "job history" }

func (c *JobHistoryCommand) Run(args []string) int {
	var json, diff, full bool
	var tmpl, versionStr string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&diff, "p", false, "")
	flags.BoolVar(&full, "full", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&versionStr, "version", "", "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one node
	args = flags.Args()
	if l := len(args); l < 1 || l > 2 {
		c.Ui.Error("This command takes one argument: <job>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if (json || len(tmpl) != 0) && (diff || full) {
		c.Ui.Error("-json and -t are exclusive with -p and -full")
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	jobID := args[0]

	// Check if the job exists
	jobs, _, err := client.Jobs().PrefixList(jobID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error listing jobs: %s", err))
		return 1
	}
	if len(jobs) == 0 {
		c.Ui.Error(fmt.Sprintf("No job(s) with prefix or id %q found", jobID))
		return 1
	}
	if len(jobs) > 1 && strings.TrimSpace(jobID) != jobs[0].ID {
		c.Ui.Error(fmt.Sprintf("Prefix matched multiple jobs\n\n%s", createStatusListOutput(jobs)))
		return 1
	}

	// Prefix lookup matched a single job
	versions, diffs, _, err := client.Jobs().Versions(jobs[0].ID, diff, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving job versions: %s", err))
		return 1
	}

	f, err := DataFormat("json", "")
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error getting formatter: %s", err))
		return 1
	}
	c.formatter = f

	if versionStr != "" {
		version, _, err := parseVersion(versionStr)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error parsing version value %q: %v", versionStr, err))
			return 1
		}

		var job *api.Job
		var diff *api.JobDiff
		var nextVersion uint64
		for i, v := range versions {
			if *v.Version != version {
				continue
			}

			job = v
			if i+1 <= len(diffs) {
				diff = diffs[i]
				nextVersion = *versions[i+1].Version
			}
		}

		if json || len(tmpl) > 0 {
			out, err := Format(json, tmpl, job)
			if err != nil {
				c.Ui.Error(err.Error())
				return 1
			}

			c.Ui.Output(out)
			return 0
		}

		if err := c.formatJobVersion(job, diff, nextVersion, full); err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

	} else {
		if json || len(tmpl) > 0 {
			out, err := Format(json, tmpl, versions)
			if err != nil {
				c.Ui.Error(err.Error())
				return 1
			}

			c.Ui.Output(out)
			return 0
		}

		if err := c.formatJobVersions(versions, diffs, full); err != nil {
			c.Ui.Error(err.Error())
			return 1
		}
	}

	return 0
}

// parseVersion parses the version flag and returns the index, whether it
// was set and potentially an error during parsing.
func parseVersion(input string) (uint64, bool, error) {
	if input == "" {
		return 0, false, nil
	}

	u, err := strconv.ParseUint(input, 10, 64)
	return u, true, err
}

func (c *JobHistoryCommand) formatJobVersions(versions []*api.Job, diffs []*api.JobDiff, full bool) error {
	vLen := len(versions)
	dLen := len(diffs)
	if dLen != 0 && vLen != dLen+1 {
		return fmt.Errorf("Number of job versions %d doesn't match number of diffs %d", vLen, dLen)
	}

	for i, version := range versions {
		var diff *api.JobDiff
		var nextVersion uint64
		if i+1 <= dLen {
			diff = diffs[i]
			nextVersion = *versions[i+1].Version
		}

		if err := c.formatJobVersion(version, diff, nextVersion, full); err != nil {
			return err
		}

		// Insert a blank
		if i != vLen-1 {
			c.Ui.Output("")
		}
	}

	return nil
}

func (c *JobHistoryCommand) formatJobVersion(job *api.Job, diff *api.JobDiff, nextVersion uint64, full bool) error {
	if job == nil {
		return fmt.Errorf("Error printing job history for non-existing job or job version")
	}

	basic := []string{
		fmt.Sprintf("Version|%d", *job.Version),
		fmt.Sprintf("Stable|%v", *job.Stable),
		fmt.Sprintf("Submit Date|%v", formatTime(time.Unix(0, *job.SubmitTime))),
	}

	if diff != nil {
		//diffStr := fmt.Sprintf("Difference between version %d and %d:", *job.Version, nextVersion)
		basic = append(basic, fmt.Sprintf("Diff|\n%s", strings.TrimSpace(formatJobDiff(diff, false))))
	}

	if full {
		out, err := c.formatter.TransformData(job)
		if err != nil {
			return fmt.Errorf("Error formatting the data: %s", err)
		}

		basic = append(basic, fmt.Sprintf("Full|JSON Job:\n%s", out))
	}

	columnConf := columnize.DefaultConfig()
	columnConf.Glue = " = "
	columnConf.NoTrim = true
	output := columnize.Format(basic, columnConf)

	c.Ui.Output(c.Colorize().Color(output))
	return nil
}
