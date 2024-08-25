// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"fmt"
	"io"
	"strings"

	"github.com/posener/complete"
)

type JobTagCommand struct {
	Meta

	Stdin  io.Reader
	Stdout io.WriteCloser
	Stderr io.WriteCloser
}

func (c *JobTagCommand) Help() string {
	helpText := `
Usage: nomad job tag [options] <jobname>

  Save a job version to prevent it from being garbage-collected and allow it to
  be diffed and reverted by name.
  
  Example usage:
 
    nomad job tag -name "My Golden Version" -description "The version of the job we can roll back to in the future if needed" <jobname>

    nomad job tag -version 3 -name "My Golden Version" <jobname>

  The first of the above will tag the latest version of the job, while the second
  will specifically tag version 3 of the job.

Tag Specific Options:

  -name <version-name>
    Specifies the name of the version to tag. This is a required field.

  -description <description>
    Specifies a description for the version. This is an optional field.

  -version <version>
    Specifies the version of the job to tag. If not provided, the latest version
    of the job will be tagged.


General Options:

  ` + generalOptionsUsage(usageOptsNoNamespace) + `
`
	return strings.TrimSpace(helpText)
}

func (c *JobTagCommand) Synopsis() string {
	return "Save a job version to prevent it from being garbage-collected and allow it to be diffed and reverted by name."
}

func (c *JobTagCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-name":        complete.PredictAnything,
			"-description": complete.PredictAnything,
			"-version":     complete.PredictNothing,
		})
}

func (c *JobTagCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *JobTagCommand) Name() string { return "job tag" }

func (c *JobTagCommand) Run(args []string) int {
	var name, description, versionStr string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) } // TODO: what's this do?
	flags.StringVar(&name, "name", "", "")
	flags.StringVar(&description, "description", "", "")
	flags.StringVar(&versionStr, "version", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	if len(flags.Args()) != 1 {
		c.Ui.Error("This command takes one argument: <job>")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	var job = flags.Args()[0]

	// Debugging: log out the name, description, versionStr, and args
	fmt.Println("name: ", name)
	fmt.Println("description: ", description)
	fmt.Println("versionStr: ", versionStr)
	fmt.Println("job: ", job)
	fmt.Println("args: ", args)

	if job == "" {
		c.Ui.Error("A job name is required")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if name == "" {
		c.Ui.Error("A version name is required")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	// // var versionID uint64
	// // if versionStr != "" {
	// // TODO: handle when versionStr is empty, implying latest version should be tagged.
	// // (Perhaps at API layer?)
	// versionID, _, err := parseVersion(versionStr)
	// if err != nil {
	// 	c.Ui.Error(fmt.Sprintf("Error parsing version value %q: %v", versionStr, err))
	// 	return 1
	// }
	// // }

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	// Check if the job exists
	jobIDPrefix := strings.TrimSpace(job)
	jobID, namespace, err := c.JobIDByPrefix(client, jobIDPrefix, nil)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	// Log out the jobID, namespace, and err
	fmt.Println("jobID: ", jobID)
	fmt.Println("namespace: ", namespace)
	fmt.Println("err: ", err)

	// Call API's Jobs.TagVersion
	// Which has a signature like this:
	// func (j *Jobs) TagVersion(jobID string, version uint64, tag *JobTaggedVersion, q *WriteOptions) (*WriteMeta, error) {
	_, err = client.Jobs().TagVersion(jobID, versionStr, name, description, nil) // TODO: writeoptions nil???
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error tagging job version: %s", err))
		return 1
	}

	// TODO: This stuff should all generally be implemented by routing through the API, and eventually be fun in something like nomad/job_endpoint.go
	// ==============
	// q := &api.QueryOptions{Namespace: namespace}

	// // Prefix lookup matched a single job
	// versions, _, _, err := client.Jobs().Versions(jobID, false, q)
	// if err != nil {
	// 	c.Ui.Error(fmt.Sprintf("Error retrieving job versions: %s", err))
	// 	return 1
	// }

	// // Check to see if the version provided exists among versions
	// if versionStr != "" {
	// 	versionID, _, err := parseVersion(versionStr)
	// 	if err != nil {
	// 		c.Ui.Error(fmt.Sprintf("Error parsing version value %q: %v", versionStr, err))
	// 		return 1
	// 	}

	// 	var versionObject *api.Job
	// 	for _, v := range versions {
	// 		if *v.Version != versionID {
	// 			// log that it's not this one
	// 			fmt.Println("not version: ", versionID)
	// 			continue
	// 		}

	// 		// log that it is this one
	// 		fmt.Println("version to tag has been found: ", versionID)

	// 		versionObject = v
	// 		versionObject.TaggedVersion = &api.JobTaggedVersion{
	// 			Name:        name,
	// 			Description: description,
	// 			TaggedTime:  time.Now().Unix(), // TODO: nanos or millis?
	// 		}

	//     // // Do some server logs
	//     // c.Ui.Output(fmt.Sprintf("Tagged version %d of %e with name %q", versionID, jobID, name))

	//     // // Do I need to do something like versionObject (which is a job) .update()?
	//     // // Am I updating the wrong thing by updating the api object, and I need to do a struct conversion?

	//     // // Handle if the version is not found
	//     // if versionObject == nil {
	//     //   c.Ui.Error(fmt.Sprintf("Version %d not found", versionID))
	//     //   return 1
	//     // }

	// 		// // Tag the version
	// 		// // Log the versionObject's TaggedVersion and versionObject
	// 		// fmt.Println("versionObject.TaggedVersion: ", versionObject.TaggedVersion)
	// 		// fmt.Println("versionObject: ", versionObject)
	// 	}
	// } else {
	// 	// Tag the latest
	// 	panic("not implemented")
	// }

	return 0

	// First of all, accept the flags
	// Then, check if the job name is provided and exists.
	// If it doesn't exist, return an error

	// Then, check if the version is provided
	// Then, check if the name is provided
	// Then, check if the description is provided
	// Then, tag the job
	// Finally, return the result

}

// func (c *JobTagCommand) Run(args []string) int {

// 	var stdinOpt, ttyOpt bool
// 	var task, allocation, job, group, escapeChar string

// 	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
// 	flags.Usage = func() { c.Ui.Output(c.Help()) }
// 	flags.StringVar(&task, "task", "", "")
// 	flags.StringVar(&group, "group", "", "")
// 	flags.StringVar(&allocation, "alloc", "", "")
// 	flags.StringVar(&job, "job", "", "")
// 	flags.BoolVar(&stdinOpt, "i", true, "")
// 	flags.BoolVar(&ttyOpt, "t", isTty(), "")
// 	flags.StringVar(&escapeChar, "e", "~", "")

// 	if err := flags.Parse(args); err != nil {
// 		c.Ui.Error(fmt.Sprintf("Error parsing flags: %s", err))
// 		return 1
// 	}

// 	args = flags.Args()

// 	if len(args) < 1 {
// 		c.Ui.Error("An action name is required")
// 		c.Ui.Error(commandErrorText(c))
// 		return 1
// 	}

// 	if job == "" {
// 		c.Ui.Error("A job ID is required")
// 		c.Ui.Error(commandErrorText(c))
// 		return 1
// 	}

// 	if ttyOpt && !stdinOpt {
// 		c.Ui.Error("-i must be enabled if running with tty")
// 		c.Ui.Error(commandErrorText(c))
// 		return 1
// 	}

// 	if escapeChar == "none" {
// 		escapeChar = ""
// 	}

// 	if len(escapeChar) > 1 {
// 		c.Ui.Error("-e requires 'none' or a single character")
// 		c.Ui.Error(commandErrorText(c))
// 		return 1
// 	}

// 	client, err := c.Meta.Client()
// 	if err != nil {
// 		c.Ui.Error(fmt.Sprintf("Error initializing client: %v", err))
// 		return 1
// 	}

// 	var allocStub *api.AllocationListStub
// 	// If no allocation provided, grab a random one from the job
// 	if allocation == "" {

// 		// Group param cannot be empty if allocation is empty,
// 		// since we'll need to get a random allocation from the group
// 		if group == "" {
// 			c.Ui.Error("A group name is required if no allocation is provided")
// 			c.Ui.Error(commandErrorText(c))
// 			return 1
// 		}

// 		if task == "" {
// 			c.Ui.Error("A task name is required if no allocation is provided")
// 			c.Ui.Error(commandErrorText(c))
// 			return 1
// 		}

// 		jobID, ns, err := c.JobIDByPrefix(client, job, nil)
// 		if err != nil {
// 			c.Ui.Error(err.Error())
// 			return 1
// 		}

// 		allocStub, err = getRandomJobAlloc(client, jobID, group, ns)
// 		if err != nil {
// 			c.Ui.Error(fmt.Sprintf("Error fetching allocations: %v", err))
// 			return 1
// 		}
// 	} else {
// 		allocs, _, err := client.Allocations().PrefixList(sanitizeUUIDPrefix(allocation))
// 		if err != nil {
// 			c.Ui.Error(fmt.Sprintf("Error querying allocation: %v", err))
// 			return 1
// 		}

// 		if len(allocs) == 0 {
// 			c.Ui.Error(fmt.Sprintf("No allocation(s) with prefix or id %q found", allocation))
// 			return 1
// 		}

// 		if len(allocs) > 1 {
// 			out := formatAllocListStubs(allocs, false, shortId)
// 			c.Ui.Error(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", out))
// 			return 1
// 		}

// 		allocStub = allocs[0]
// 	}

// 	q := &api.QueryOptions{Namespace: allocStub.Namespace}
// 	alloc, _, err := client.Allocations().Info(allocStub.ID, q)
// 	if err != nil {
// 		c.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
// 		return 1
// 	}

// 	if task != "" {
// 		err = validateTaskExistsInAllocation(task, alloc)
// 	} else {
// 		task, err = lookupAllocTask(alloc)
// 	}
// 	if err != nil {
// 		c.Ui.Error(err.Error())
// 		return 1
// 	}

// 	if !stdinOpt {
// 		c.Stdin = bytes.NewReader(nil)
// 	}

// 	if c.Stdin == nil {
// 		c.Stdin = os.Stdin
// 	}

// 	if c.Stdout == nil {
// 		c.Stdout = os.Stdout
// 	}

// 	if c.Stderr == nil {
// 		c.Stderr = os.Stderr
// 	}

// 	action := args[0]

// 	code, err := c.execImpl(client, alloc, task, job, action, ttyOpt, escapeChar, c.Stdin, c.Stdout, c.Stderr)
// 	if err != nil {
// 		c.Ui.Error(fmt.Sprintf("failed to exec into task: %v", err))
// 		return 1
// 	}

// 	return code
// }
