// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"strings"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type JobInspectCommand struct {
	Meta
}

func (c *JobInspectCommand) Help() string {
	helpText := `
Usage: nomad job inspect [options] <job>
Alias: nomad inspect

  Inspect is used to see the specification of a submitted job.

  When ACLs are enabled, this command requires a token with the 'read-job'
  capability for the job's namespace. The 'list-jobs' capability is required to
  run the command with a job prefix instead of the exact job ID.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Inspect Options:

  -version <job version>
    Display the job at the given job version. Defaults to current version.

  -json
    Output the job in its JSON format. Cannot be used with -hcl.

  -hcl
    Output the original HCL submitted with the job. Cannot be used with -json.

  -with-vars
    Include the original HCL2 variables submitted with the job. Can only be used
    with -hcl.

  -t
    Format and display job using a Go template.
`
	return strings.TrimSpace(helpText)
}

func (c *JobInspectCommand) Synopsis() string {
	return "Inspect a submitted job"
}

func (c *JobInspectCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-version":   complete.PredictAnything,
			"-hcl":       complete.PredictNothing,
			"-with-vars": complete.PredictNothing,
			"-json":      complete.PredictNothing,
			"-t":         complete.PredictAnything,
		})
}

func (c *JobInspectCommand) AutocompleteArgs() complete.Predictor {
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

func (c *JobInspectCommand) Name() string { return "job inspect" }

func (c *JobInspectCommand) Run(args []string) int {
	var json, hcl, withVars bool
	var tmpl, versionStr string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&json, "json", false, "")
	flags.BoolVar(&hcl, "hcl", false, "")
	flags.BoolVar(&withVars, "with-vars", false, "")
	flags.StringVar(&tmpl, "t", "", "")
	flags.StringVar(&versionStr, "version", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if hcl && json {
		c.Ui.Error("can only use one of -hcl or -json")
		return 1
	}
	if withVars && !hcl {
		c.Ui.Error("can only use -with-vars with -hcl")
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// If args not specified but output format is specified, format and output the jobs data list
	if len(args) == 0 && (json || len(tmpl) > 0) {
		jobs, _, err := client.Jobs().List(nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying jobs: %v", err))
			return 1
		}

		out, err := Format(json, tmpl, jobs)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	// Check that we got exactly one job
	if len(args) != 1 {
		c.Ui.Error("This command takes one argument: <job>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// Check if the job exists
	jobIDPrefix := strings.TrimSpace(args[0])
	jobID, namespace, err := c.JobIDByPrefix(client, jobIDPrefix, nil)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	var version *uint64
	if versionStr != "" {
		v, _, err := parseVersion(versionStr)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error parsing version value %q: %v", versionStr, err))
			return 1
		}

		version = &v
	}

	if hcl {
		out, err := getJobHCL(client, namespace, jobID, version)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error inspecting job: %s", err))
			return 1
		}
		c.Ui.Output(out.Source)

		if withVars {
			c.Ui.Warn(getWithVarsOutput(namespace, jobID, out.Variables, out.VariableFlags))
		}

		return 0
	}

	// Prefix lookup matched a single job
	job, err := getJob(client, namespace, jobID, version)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error inspecting job: %s", err))
		return 1
	}

	// If output format is specified, format and output the data
	if json || len(tmpl) > 0 {
		out, err := Format(json, tmpl, job)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		c.Ui.Output(out)
		return 0
	}

	// Print the contents of the job
	req := struct {
		Job *api.Job
	}{
		Job: job,
	}
	f, err := DataFormat("json", "")
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error getting formatter: %s", err))
		return 1
	}

	out, err := f.TransformData(req)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error formatting the data: %s", err))
		return 1
	}
	c.Ui.Output(out)
	return 0
}

// getJob retrieves the job optionally at a particular version.
func getJob(client *api.Client, namespace, jobID string, version *uint64) (*api.Job, error) {
	var q *api.QueryOptions
	if namespace != "" {
		q = &api.QueryOptions{Namespace: namespace}
	}
	if version == nil {
		job, _, err := client.Jobs().Info(jobID, q)
		return job, err
	}

	versions, _, _, err := client.Jobs().Versions(jobID, false, q)
	if err != nil {
		return nil, err
	}

	for _, j := range versions {
		if *j.Version != *version {
			continue
		}
		return j, nil
	}

	return nil, fmt.Errorf("job %q with version %d couldn't be found", jobID, *version)
}

// getJob retrieves the job optionally at a particular version.
func getJobHCL(client *api.Client, namespace, jobID string, version *uint64) (*api.JobSubmission, error) {
	var q *api.QueryOptions
	if namespace != "" {
		q = &api.QueryOptions{Namespace: namespace}
	}
	v := uint64(0)
	if version != nil {
		v = *version
	} else {
		job, _, err := client.Jobs().Info(jobID, q)
		if err != nil {
			return nil, err
		}
		v = *job.Version
	}
	submission, _, err := client.Jobs().Submission(jobID, int(v), q)

	if err != nil {
		return nil, fmt.Errorf("job %q with version %d couldn't be found", jobID, version)
	}
	return submission, err
}

func getWithVarsOutput(namespace, jobID string, uiVars string, varsMap map[string]string) string {
	runArgs := []string{}
	if namespace != "" {
		runArgs = append(runArgs, "-namespace")
		runArgs = append(runArgs, namespace)
	}

	for k, v := range varsMap {
		runArgs = append(runArgs, "-var")
		runArgs = append(runArgs, fmt.Sprintf("%s=%s", k, v))
	}
	for _, uiVar := range strings.Split(uiVars, "\n") {
		uiVar = strings.TrimSpace(uiVar)
		if uiVar != "" {
			runArgs = append(runArgs, "-var")
			runArgs = append(runArgs, uiVar)
		}
	}
	runArgs = append(runArgs, jobID)

	return fmt.Sprintf(`
To run this job as originally submitted:

$ nomad job inspect -namespace %s -hcl %s |
    nomad job run %s
`, namespace, jobID, strings.Join(runArgs, " "))

}
