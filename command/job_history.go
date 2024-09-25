// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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

  When ACLs are enabled, this command requires a token with the 'read-job'
  capability for the job's namespace. The 'list-jobs' capability is required to
  run the command with a job prefix instead of the exact job ID.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

History Options:

  -p
    Display the difference between each version of the job and a reference
    version. The reference version can be specified using the -diff-tag or
    -diff-version flags. If neither flag is set, the most recent version is used.

  -diff-tag
    Specifies the version of the job to compare against, referenced by
    tag name (defaults to latest). Mutually exclusive with -diff-version.
    This tag can be set using the "nomad job tag" command.

  -diff-version
    Specifies the version number of the job to compare against.
    Mutually exclusive with -diff-tag.

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
			"-p":            complete.PredictNothing,
			"-full":         complete.PredictNothing,
			"-version":      complete.PredictAnything,
			"-json":         complete.PredictNothing,
			"-t":            complete.PredictAnything,
			"-diff-tag":     complete.PredictNothing,
			"-diff-version": complete.PredictNothing,
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
	var tmpl, versionStr, diffTag, diffVersionFlag string
	var diffVersion *uint64

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&diff, "p", false, "")
	flags.BoolVar(&full, "full", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&versionStr, "version", "", "")
	flags.StringVar(&tmpl, "t", "", "")
	flags.StringVar(&diffTag, "diff-tag", "", "")
	flags.StringVar(&diffVersionFlag, "diff-version", "", "")

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

	if (diffTag != "" && !diff) || (diffVersionFlag != "" && !diff) {
		c.Ui.Error("-diff-tag and -diff-version can only be used with -p")
		return 1
	}

	if diffTag != "" && diffVersionFlag != "" {
		c.Ui.Error("-diff-tag and -diff-version are mutually exclusive")
		return 1
	}

	if diffVersionFlag != "" {
		parsedDiffVersion, err := strconv.ParseUint(diffVersionFlag, 10, 64)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error parsing -diff-version: %s", err))
			return 1
		}
		diffVersion = &parsedDiffVersion
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Check if the job exists
	jobIDPrefix := strings.TrimSpace(args[0])
	jobID, namespace, err := c.JobIDByPrefix(client, jobIDPrefix, nil)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	q := &api.QueryOptions{Namespace: namespace}

	// Prefix lookup matched a single job
	versionOptions := &api.VersionsOptions{
		Diffs:       diff,
		DiffTag:     diffTag,
		DiffVersion: diffVersion,
	}
	versions, diffs, _, err := client.Jobs().VersionsOpts(jobID, versionOptions, q)
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
		for i, v := range versions {
			if *v.Version != version {
				continue
			}

			job = v
			if i+1 <= len(diffs) {
				diff = diffs[i]
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

		if err := c.formatJobVersion(job, diff, full); err != nil {
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

	for i, version := range versions {
		var diff *api.JobDiff
		if i+1 <= dLen {
			diff = diffs[i]
		}

		if err := c.formatJobVersion(version, diff, full); err != nil {
			return err
		}

		// Insert a blank
		if i != vLen-1 {
			c.Ui.Output("")
		}
	}

	return nil
}

func (c *JobHistoryCommand) formatJobVersion(job *api.Job, diff *api.JobDiff, full bool) error {
	if job == nil {
		return fmt.Errorf("Error printing job history for non-existing job or job version")
	}

	basic := []string{
		fmt.Sprintf("Version|%d", *job.Version),
		fmt.Sprintf("Stable|%v", *job.Stable),
		fmt.Sprintf("Submit Date|%v", formatTime(time.Unix(0, *job.SubmitTime))),
	}
	// if tagged version is not nil
	if job.VersionTag != nil {
		basic = append(basic, fmt.Sprintf("Tag Name|%v", *&job.VersionTag.Name))
		if job.VersionTag.Description != "" {
			basic = append(basic, fmt.Sprintf("Tag Description|%v", *&job.VersionTag.Description))
		}
	}

	if diff != nil && diff.Type != "None" {
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
