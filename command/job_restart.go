// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-set/v3"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

const (
	// jobRestartTimestampPrefixLength is the number of characters in the
	// "==> [timestamp]: " string that prefixes most of the outputs of this
	// command.
	jobRestartTimestampPrefixLength = 31

	// jobRestartBatchWaitAsk is the special token used to indicate that the
	// command should ask user for confirmation between batches.
	jobRestartBatchWaitAsk = "ask"

	// jobRestartOnErrorFail is the special token used to indicate that the
	// command should exit when a batch has errors.
	jobRestartOnErrorFail = "fail"

	// jobRestartOnErrorAks is the special token used to indicate that the
	// command should ask user for confirmation when a batch has errors.
	jobRestartOnErrorAsk = "ask"
)

var (
	// jobRestartBatchSizeValueRegex validates that the value passed to
	// -batch-size is an integer optionally followed by a % sign.
	//
	// Use ^...$ to make sure we're matching over the entire input to avoid
	// partial matches such as 10%20%.
	jobRestartBatchSizeValueRegex = regexp.MustCompile(`^(\d+)%?$`)
)

// ErrJobRestartPlacementFailure is an error that indicates a placement failure
type ErrJobRestartPlacementFailure struct {
	EvalID    string
	TaskGroup string
	Failures  *api.AllocationMetric
}

func (e ErrJobRestartPlacementFailure) Error() string {
	return fmt.Sprintf("Evaluation %q has placement failures for group %q:\n%s",
		e.EvalID,
		e.TaskGroup,
		formatAllocMetrics(e.Failures, false, strings.Repeat(" ", 4)),
	)
}

func (e ErrJobRestartPlacementFailure) Is(err error) bool {
	_, ok := err.(ErrJobRestartPlacementFailure)
	return ok
}

// JobRestartCommand is the implementation for the command that restarts a job.
type JobRestartCommand struct {
	Meta

	// client is the Nomad API client shared by all functions in the command to
	// reuse the same connection.
	client *api.Client

	// Configuration values read and parsed from command flags and args.
	allTasks         bool
	autoYes          bool
	batchSize        int
	batchSizePercent bool
	batchWait        time.Duration
	batchWaitAsk     bool
	groups           *set.Set[string]
	jobID            string
	noShutdownDelay  bool
	onError          string
	reschedule       bool
	tasks            *set.Set[string]
	verbose          bool
	length           int

	// canceled is set to true when the user gives a negative answer to any of
	// the questions.
	canceled bool

	// sigsCh is used to subscribe to signals from the operating system.
	sigsCh chan os.Signal
}

func (c *JobRestartCommand) Help() string {
	helpText := `
Usage: nomad job restart [options] <job>

  Restart or reschedule allocations for a particular job.

  Restarting the job calls the 'Restart Allocation' API endpoint to restart the
  tasks inside allocations, so the allocations themselves are not modified but
  rather restarted in-place.

  Rescheduling the job uses the 'Stop Allocation' API endpoint to stop the
  allocations and trigger the Nomad scheduler to compute new placements. This
  may cause the new allocations to be scheduled in different clients from the
  originals.

  This command can operate in batches and it waits until all restarted or
  rescheduled allocations are running again before proceeding to the next
  batch. It is also possible to specify additional time to wait between
  batches.

  Allocations can be restarted in-place or rescheduled. When restarting
  in-place the command may target specific tasks in the allocations, restart
  only tasks that are currently running, or restart all tasks, even the ones
  that have already run. Allocations can also be targeted by group. When both
  groups and tasks are defined only the tasks for the allocations of those
  groups are restarted.

  When rescheduling, the current allocations are stopped triggering the Nomad
  scheduler to create replacement allocations that may be placed in different
  clients. The command waits until the new allocations have client status
  'ready' before proceeding with the remaining batches. Services health checks
  are not taken into account.

  By default the command restarts all running tasks in-place with one
  allocation per batch.

  When ACLs are enabled, this command requires a token with the
  'alloc-lifecycle' and 'read-job' capabilities for the job's namespace. The
  'list-jobs' capability is required to run the command with a job prefix
  instead of the exact job ID.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Restart Options:

  -all-tasks
    If set, all tasks in the allocations are restarted, even the ones that
    have already run, such as non-sidecar tasks. Tasks will restart following
    their lifecycle order. This option cannot be used with '-task'.

  -batch-size=<n|n%>
    Number of allocations to restart at once. It may be defined as a percentage
    value of the current number of running allocations. Percentage values are
    rounded up to increase parallelism. Defaults to 1.

  -batch-wait=<duration|'ask'>
    Time to wait between restart batches. If set to 'ask' the command halts
    between batches and waits for user input on how to proceed. If the answer
    is a time duration all remaining batches will use this new value. Defaults
    to 0.

  -group=<group-name>
    Only restart allocations for the given group. Can be specified multiple
    times. If no group is set all allocations for the job are restarted.

  -no-shutdown-delay
    Ignore the group and task 'shutdown_delay' configuration so there is no
    delay between service deregistration and task shutdown or restart. Note
    that using this flag will result in failed network connections to the
    allocation being restarted.

  -on-error=<'ask'|'fail'>
    Determines what action to take when an error happens during a restart
    batch. If 'ask' the command stops and waits for user confirmation on how to
    proceed. If 'fail' the command exits immediately. Defaults to 'ask'.

  -reschedule
    If set, allocations are stopped and rescheduled instead of restarted
    in-place. Since the group is not modified the restart does not create a new
    deployment, and so values defined in 'update' blocks, such as
    'max_parallel', are not taken into account. This option cannot be used with
    '-task'. Only jobs of type 'batch', 'service', and 'system' can be
    rescheduled.

  -task=<task-name>
    Specify the task to restart. Can be specified multiple times. If groups are
    also specified the task must exist in at least one of them. If no task is
    set only tasks that are currently running are restarted. For example,
    non-sidecar tasks that already ran are not restarted unless '-all-tasks' is
    used instead. This option cannot be used with '-all-tasks' or
    '-reschedule'.

  -yes
    Automatic yes to prompts. If set, the command automatically restarts
    multi-region jobs only in the region targeted by the command, ignores batch
    errors, and automatically proceeds with the remaining batches without
    waiting. Use '-on-error' and '-batch-wait' to adjust these behaviors.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *JobRestartCommand) Synopsis() string {
	return "Restart or reschedule allocations for a job"
}

func (c *JobRestartCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-all-tasks":         complete.PredictNothing,
			"-batch-size":        complete.PredictAnything,
			"-batch-wait":        complete.PredictAnything,
			"-no-shutdown-delay": complete.PredictNothing,
			"-on-error":          complete.PredictSet(jobRestartOnErrorAsk, jobRestartOnErrorFail),
			"-reschedule":        complete.PredictNothing,
			"-task":              complete.PredictAnything,
			"-yes":               complete.PredictNothing,
			"-verbose":           complete.PredictNothing,
		})
}

func (c *JobRestartCommand) AutocompleteArgs() complete.Predictor {
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

func (c *JobRestartCommand) Name() string { return "job restart" }

func (c *JobRestartCommand) Run(args []string) int {
	// Parse and validate command line arguments.
	code, err := c.parseAndValidate(args)
	if err != nil {
		c.Ui.Error(err.Error())
		c.Ui.Error(commandErrorText(c))
		return code
	}
	if code != 0 {
		return code
	}

	c.client, err = c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	// Use prefix matching to find job.
	job, err := c.JobByPrefix(c.client, c.jobID, nil)
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	c.jobID = *job.ID
	if job.Namespace != nil {
		c.client.SetNamespace(*job.Namespace)
	}

	// Handle SIGINT to prevent accidental cancellations of the long-lived
	// restart loop. activeCh is blocked while a signal is being handled to
	// prevent new work from starting while the user is deciding if they want
	// to cancel the command or not.
	activeCh := make(chan any)
	c.sigsCh = make(chan os.Signal, 1)
	signal.Notify(c.sigsCh, os.Interrupt)
	defer signal.Stop(c.sigsCh)

	go c.handleSignal(c.sigsCh, activeCh)

	// Verify job type can be rescheduled.
	if c.reschedule {
		switch *job.Type {
		case api.JobTypeBatch, api.JobTypeService, api.JobTypeSystem:
		default:
			c.Ui.Error(fmt.Sprintf("Jobs of type %q are not allowed to be rescheduled.", *job.Type))
			return 1
		}
	}

	// Confirm that we should restart a multi-region job in a single region.
	if job.IsMultiregion() && !c.autoYes && !c.shouldRestartMultiregion() {
		c.Ui.Output("\nJob restart canceled.")
		return 0
	}

	// Retrieve the job history so we can properly determine if a group or task
	// exists in the specific allocation job version.
	jobVersions, _, _, err := c.client.Jobs().Versions(c.jobID, false, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving versions of job %q: %s", c.jobID, err))
		return 1
	}

	// Index jobs by version.
	jobVersionIndex := make(map[uint64]*api.Job, len(jobVersions))
	for _, job := range jobVersions {
		jobVersionIndex[*job.Version] = job
	}

	// Fetch all allocations for the job and filter out the ones that are not
	// eligible for restart.
	allocStubs, _, err := c.client.Jobs().Allocations(c.jobID, true, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving allocations for job %q: %v", c.jobID, err))
		return 1
	}
	allocStubsWithJob := make([]AllocationListStubWithJob, 0, len(allocStubs))
	for _, stub := range allocStubs {
		allocStubsWithJob = append(allocStubsWithJob, AllocationListStubWithJob{
			AllocationListStub: stub,
			Job:                jobVersionIndex[stub.JobVersion],
		})
	}
	restartAllocs := c.filterAllocs(allocStubsWithJob)

	// Exit early if there's nothing to do.
	if len(restartAllocs) == 0 {
		c.Ui.Output("No allocations to restart")
		return 0
	}

	// Calculate absolute batch size based on the number of eligible
	// allocations. Round values up to increase parallelism.
	if c.batchSizePercent {
		c.batchSize = int(math.Ceil(float64(len(restartAllocs)*c.batchSize) / 100))
	}

	c.Ui.Output(c.Colorize().Color(fmt.Sprintf(
		"[bold]==> %s: Restarting %s[reset]",
		formatTime(time.Now()),
		english.Plural(len(restartAllocs), "allocation", "allocations"),
	)))

	// restartErr accumulates the errors that happen in each batch.
	var restartErr *multierror.Error

	// Restart allocations in batches.
	batch := multierror.Group{}
	for restartCount, alloc := range restartAllocs {
		// Block and wait before each iteration if the command is handling an
		// interrupt signal.
		<-activeCh

		// Make sure there are not active deployments to prevent the restart
		// process from interfering with it.
		err := c.ensureNoActiveDeployment()
		if err != nil {
			restartErr = multierror.Append(restartErr, err)
			break
		}

		// Print new batch header every time we restart a multiple of the batch
		// size which indicates that we're starting a new batch.
		// Skip batch header if batch size is one because it's redundant.
		if restartCount%c.batchSize == 0 && c.batchSize > 1 {
			batchNumber := restartCount/c.batchSize + 1
			remaining := len(restartAllocs) - restartCount

			c.Ui.Output(c.Colorize().Color(fmt.Sprintf(
				"[bold]==> %s: Restarting %s batch of %d allocations[reset]",
				formatTime(time.Now()),
				humanize.Ordinal(batchNumber),
				min(c.batchSize, remaining),
			)))
		}

		// Restart allocation. Wrap the callback function to capture the
		// allocID loop variable and prevent it from changing inside the
		// goroutine at each iteration.
		batch.Go(func(allocStubWithJob AllocationListStubWithJob) func() error {
			return func() error {
				return c.handleAlloc(allocStubWithJob)
			}
		}(alloc))

		// Check if we restarted enough allocations to complete a batch or if
		// we restarted the last allocation.
		batchComplete := (restartCount+1)%c.batchSize == 0
		restartComplete := restartCount+1 == len(restartAllocs)
		if batchComplete || restartComplete {

			// Block and wait for the batch to finish. Handle the
			// *mutierror.Error response to add the custom formatting and to
			// convert it to an error to avoid problems where an empty
			// *multierror.Error is not considered a nil error.
			var batchErr error
			if batchMerr := batch.Wait(); batchMerr != nil {
				restartErr = multierror.Append(restartErr, batchMerr)
				batchMerr.ErrorFormat = c.errorFormat(jobRestartTimestampPrefixLength)
				batchErr = batchMerr.ErrorOrNil()
			}

			// Block if the command is handling an interrupt signal.
			<-activeCh

			// Exit loop before sleeping or asking for user input if we just
			// finished the last batch.
			if restartComplete {
				break
			}

			// Handle errors that happened in this batch.
			if batchErr != nil {
				// Exit early if -on-error is 'fail'.
				if c.onError == jobRestartOnErrorFail {
					c.Ui.Output(c.Colorize().Color(fmt.Sprintf(
						"[bold]==> %s: Stopping job restart due to error[reset]",
						formatTime(time.Now()),
					)))
					break
				}

				// Exit early if -yes but error is not recoverable.
				if c.autoYes && !c.isErrorRecoverable(batchErr) {
					c.Ui.Output(c.Colorize().Color(fmt.Sprintf(
						"[bold]==> %s: Stopping job restart due to unrecoverable error[reset]",
						formatTime(time.Now()),
					)))
					break
				}
			}

			// Check if we need to ask the user how to proceed. This is needed
			// in case -yes is not set and -batch-wait is 'ask' or an error
			// happened and -on-error is 'ask'.
			askUser := !c.autoYes && (c.batchWaitAsk || c.onError == jobRestartOnErrorAsk && batchErr != nil)
			if askUser {
				if batchErr != nil {
					// Print errors so user can decide what to below.
					c.Ui.Warn(c.Colorize().Color(fmt.Sprintf(
						"[bold]==> %s: %s[reset]", formatTime(time.Now()), batchErr,
					)))
				}

				// Exit early if user provides a negative answer.
				if !c.shouldProceed(batchErr) {
					c.Ui.Output(c.Colorize().Color(fmt.Sprintf(
						"[bold]==> %s: Job restart canceled[reset]",
						formatTime(time.Now()),
					)))
					c.canceled = true
					break
				}
			}

			// Sleep if -batch-wait is set or if -batch-wait is 'ask' and user
			// responded with a new interval above.
			if c.batchWait > 0 {
				c.Ui.Output(c.Colorize().Color(fmt.Sprintf(
					"[bold]==> %s: Waiting %s before restarting the next batch[reset]",
					formatTime(time.Now()),
					c.batchWait,
				)))
				time.Sleep(c.batchWait)
			}

			// Start a new batch.
			batch = multierror.Group{}
		}
	}

	if restartErr != nil && len(restartErr.Errors) > 0 {
		if !c.canceled {
			c.Ui.Output(c.Colorize().Color(fmt.Sprintf(
				"[bold]==> %s: Job restart finished with errors[reset]",
				formatTime(time.Now()),
			)))
		}

		restartErr.ErrorFormat = c.errorFormat(0)
		c.Ui.Error(fmt.Sprintf("\n%s", restartErr))
		return 1
	}

	if !c.canceled {
		c.Ui.Output(c.Colorize().Color(fmt.Sprintf(
			"[bold]==> %s: Job restart finished[reset]",
			formatTime(time.Now()),
		)))

		c.Ui.Output("\nJob restarted successfully!")
	}
	return 0
}

// parseAndValidate parses and validates the arguments passed to the command.
//
// This function mutates the command and is not thread-safe so it must be
// called only once and early in the command lifecycle.
func (c *JobRestartCommand) parseAndValidate(args []string) (int, error) {
	var batchSizeStr string
	var batchWaitStr string
	var groups []string
	var tasks []string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&c.allTasks, "all-tasks", false, "")
	flags.BoolVar(&c.autoYes, "yes", false, "")
	flags.StringVar(&batchSizeStr, "batch-size", "1", "")
	flags.StringVar(&batchWaitStr, "batch-wait", "0s", "")
	flags.StringVar(&c.onError, "on-error", jobRestartOnErrorAsk, "")
	flags.BoolVar(&c.noShutdownDelay, "no-shutdown-delay", false, "")
	flags.BoolVar(&c.reschedule, "reschedule", false, "")
	flags.BoolVar(&c.verbose, "verbose", false, "")
	flags.Var((funcVar)(func(s string) error {
		groups = append(groups, s)
		return nil
	}), "group", "")
	flags.Var((funcVar)(func(s string) error {
		tasks = append(tasks, s)
		return nil
	}), "task", "")

	err := flags.Parse(args)
	if err != nil {
		// Let the flags library handle and print the error message.
		return 1, nil
	}

	// Truncate IDs unless full length is requested.
	c.length = shortId
	if c.verbose {
		c.length = fullId
	}

	// Check that we got exactly one job.
	args = flags.Args()
	if len(args) != 1 {
		return 1, fmt.Errorf("This command takes one argument: <job>")
	}
	c.jobID = strings.TrimSpace(args[0])

	// Parse and validate -batch-size.
	matches := jobRestartBatchSizeValueRegex.FindStringSubmatch(batchSizeStr)
	if len(matches) != 2 {
		return 1, fmt.Errorf(
			"Invalid -batch-size value %q: batch size must be an integer or a percentage",
			batchSizeStr,
		)
	}

	c.batchSizePercent = strings.HasSuffix(batchSizeStr, "%")
	c.batchSize, err = strconv.Atoi(matches[1])
	if err != nil {
		return 1, fmt.Errorf("Invalid -batch-size value %q: %w", batchSizeStr, err)
	}
	if c.batchSize == 0 {
		return 1, fmt.Errorf(
			"Invalid -batch-size value %q: number value must be greater than zero",
			batchSizeStr,
		)
	}

	// Parse and validate -batch-wait.
	if strings.ToLower(batchWaitStr) == jobRestartBatchWaitAsk {
		if !isTty() && !c.autoYes {
			return 1, fmt.Errorf(
				"Invalid -batch-wait value %[1]q: %[1]q cannot be used when terminal is not interactive",
				jobRestartBatchWaitAsk,
			)
		}
		c.batchWaitAsk = true
	} else {
		c.batchWait, err = time.ParseDuration(batchWaitStr)
		if err != nil {
			return 1, fmt.Errorf("Invalid -batch-wait value %q: %w", batchWaitStr, err)
		}
	}

	// Parse and validate -on-error.
	switch c.onError {
	case jobRestartOnErrorAsk:
		if !isTty() && !c.autoYes {
			return 1, fmt.Errorf(
				"Invalid -on-error value %[1]q: %[1]q cannot be used when terminal is not interactive",
				jobRestartOnErrorAsk,
			)
		}
	case jobRestartOnErrorFail:
	default:
		return 1, fmt.Errorf(
			"Invalid -on-error value %q: valid options are %q and %q",
			c.onError,
			jobRestartOnErrorAsk,
			jobRestartOnErrorFail,
		)
	}

	// -all-tasks conflicts with -task and <task>.
	if c.allTasks && len(tasks) != 0 {
		return 1, fmt.Errorf("The -all-tasks option cannot be used with -task")
	}

	// -reschedule conflicts with -task and <task>.
	if c.reschedule && len(tasks) != 0 {
		return 1, fmt.Errorf("The -reschedule option cannot be used with -task")
	}

	// Dedup tasks and groups.
	c.groups = set.From(groups)
	c.tasks = set.From(tasks)

	return 0, nil
}

// filterAllocs returns a slice of the allocations that should be restarted.
func (c *JobRestartCommand) filterAllocs(stubs []AllocationListStubWithJob) []AllocationListStubWithJob {
	result := []AllocationListStubWithJob{}
	for _, stub := range stubs {
		shortAllocID := limit(stub.ID, c.length)

		// Skip allocations that are not running.
		if !stub.IsRunning() {
			if c.verbose {
				c.Ui.Output(c.Colorize().Color(fmt.Sprintf(
					"[dark_gray]    %s: Skipping allocation %q because desired status is %q and client status is %q[reset]",
					formatTime(time.Now()),
					shortAllocID,
					stub.ClientStatus,
					stub.DesiredStatus,
				)))
			}
			continue
		}

		// Skip allocations that have already been replaced.
		if stub.NextAllocation != "" {
			if c.verbose {
				c.Ui.Output(c.Colorize().Color(fmt.Sprintf(
					"[dark_gray]    %s: Skipping allocation %q because it has already been replaced by %q[reset]",
					formatTime(time.Now()),
					shortAllocID,
					limit(stub.NextAllocation, c.length),
				)))
			}
			continue
		}

		// Skip allocations for groups that were not requested.
		if c.groups.Size() > 0 {
			if !c.groups.Contains(stub.TaskGroup) {
				if c.verbose {
					c.Ui.Output(c.Colorize().Color(fmt.Sprintf(
						"[dark_gray]    %s: Skipping allocation %q because it doesn't have any of requested groups[reset]",
						formatTime(time.Now()),
						shortAllocID,
					)))
				}
				continue
			}
		}

		// Skip allocations that don't have any of the requested tasks.
		if c.tasks.Size() > 0 {
			hasTask := false
			for taskName := range c.tasks.Items() {
				if stub.HasTask(taskName) {
					hasTask = true
					break
				}
			}

			if !hasTask {
				if c.verbose {
					c.Ui.Output(c.Colorize().Color(fmt.Sprintf(
						"[dark_gray]    %s: Skipping allocation %q because it doesn't have any of requested tasks[reset]",
						formatTime(time.Now()),
						shortAllocID,
					)))
				}
				continue
			}
		}

		result = append(result, stub)
	}

	return result
}

// ensureNoActiveDeployment returns an error if the job has an active
// deployment.
func (c *JobRestartCommand) ensureNoActiveDeployment() error {
	deployments, _, err := c.client.Jobs().Deployments(c.jobID, true, nil)
	if err != nil {
		return fmt.Errorf("Error retrieving deployments for job %q: %v", c.jobID, err)

	}

	for _, d := range deployments {
		switch d.Status {
		case api.DeploymentStatusFailed, api.DeploymentStatusSuccessful, api.DeploymentStatusCancelled:
			// Deployment is terminal so it's safe to proceed.
		default:
			return fmt.Errorf("Deployment %q is %q", limit(d.ID, c.length), d.Status)
		}
	}
	return nil
}

// shouldRestartMultiregion blocks and waits for the user to confirm if the
// restart of a multi-region job should proceed. Returns true if the answer is
// positive.
func (c *JobRestartCommand) shouldRestartMultiregion() bool {
	question := fmt.Sprintf(
		"Are you sure you want to restart multi-region job %q in a single region? [y/N]",
		c.jobID,
	)

	return c.askQuestion(
		question,
		false,
		func(answer string) (bool, error) {
			switch strings.TrimSpace(strings.ToLower(answer)) {
			case "", "n", "no":
				return false, nil
			case "y", "yes":
				return true, nil
			default:
				return false, fmt.Errorf("Invalid answer %q", answer)
			}
		})
}

// shouldProceed blocks and waits for the user to provide a valid input on how
// to proceed. Returns true if the answer is positive.
//
// The flags -batch-wait and -on-error have an 'ask' option. This function
// handles both to prevent asking the user twice in case they are both set to
// 'ask' and an error happens.
func (c *JobRestartCommand) shouldProceed(err error) bool {
	var question, options string

	if err == nil {
		question = "Proceed with the next batch?"
		options = "Y/n"
	} else {
		question = "Ignore the errors above and proceed with the next batch?"
		options = "y/N" // Defaults to 'no' if an error happens.

		if !c.isErrorRecoverable(err) {
			question = `The errors above are likely to happen again.
Ignore them anyway and proceed with the next batch?`
		}
	}

	// If -batch-wait is 'ask' the user can provide a new wait duration.
	if c.batchWaitAsk {
		options += "/<wait duration>"
	}

	return c.askQuestion(
		fmt.Sprintf("%s [%s]", question, options),
		false,
		func(answer string) (bool, error) {
			switch strings.ToLower(answer) {
			case "":
				// Proceed by default only if there is no error.
				return err == nil, nil
			case "y", "yes":
				return true, nil
			case "n", "no":
				return false, nil
			default:
				if c.batchWaitAsk {
					// Check if user passed a time duration and adjust the
					// command to use that moving forward.
					batchWait, err := time.ParseDuration(answer)
					if err != nil {
						return false, fmt.Errorf("Invalid answer %q", answer)
					}

					c.batchWaitAsk = false
					c.batchWait = batchWait
					c.Ui.Output(c.Colorize().Color(fmt.Sprintf(
						"[bold]==> %s: Proceeding restarts with new wait time of %s[reset]",
						formatTime(time.Now()),
						c.batchWait,
					)))
					return true, nil
				} else {
					return false, fmt.Errorf("Invalid answer %q", answer)
				}
			}
		})
}

// shouldExit blocks and waits for the user for confirmation if they would like
// to interrupt the command. Returns true if the answer is positive.
func (c *JobRestartCommand) shouldExit() bool {
	question := `Restart interrupted, no more allocations will be restarted.
Are you sure you want to stop the restart process? [y/N]`

	return c.askQuestion(
		question,
		true,
		func(answer string) (bool, error) {
			switch strings.ToLower(answer) {
			case "n", "no", "":
				return false, nil
			case "y", "yes":
				return true, nil
			default:
				return false, fmt.Errorf("Invalid answer %q", answer)
			}
		})
}

// askQuestion asks question to user until they provide a valid response.
func (c *JobRestartCommand) askQuestion(question string, onError bool, cb func(string) (bool, error)) bool {
	prefixedQuestion := fmt.Sprintf(
		"[bold]==> %s: %s[reset]",
		formatTime(time.Now()),
		indentString(question, jobRestartTimestampPrefixLength),
	)

	// Let ui.Ask() handle interrupt signals.
	signal.Stop(c.sigsCh)
	defer func() {
		signal.Notify(c.sigsCh, os.Interrupt)
	}()

	for {
		answer, err := c.Ui.Ask(c.Colorize().Color(prefixedQuestion))
		if err != nil {
			if err.Error() != "interrupted" {
				c.Ui.Output(err.Error())
			}
			return onError
		}

		exit, err := cb(strings.TrimSpace(answer))
		if err != nil {
			c.Ui.Output(fmt.Sprintf("%s%s", strings.Repeat(" ", jobRestartTimestampPrefixLength), err))
			continue
		}
		return exit
	}
}

// handleAlloc stops or restarts an allocation in-place. Blocks until the
// allocation  is done restarting or the rescheduled allocation is running.
func (c *JobRestartCommand) handleAlloc(alloc AllocationListStubWithJob) error {
	var err error
	if c.reschedule {
		// Stopping an allocation triggers a reschedule.
		err = c.stopAlloc(alloc)
	} else {
		err = c.restartAlloc(alloc)
	}
	if err != nil {
		msg := fmt.Sprintf("Error restarting allocation %q:", limit(alloc.ID, c.length))
		if mErr, ok := err.(*multierror.Error); ok {
			// Unwrap the errors and prefix them with a common message to
			// prevent deep nesting of errors.
			return multierror.Prefix(mErr, msg)
		}
		return fmt.Errorf("%s %w", msg, err)
	}
	return nil
}

// restartAlloc restarts an allocation in place and blocks until the tasks are
// done restarting.
func (c *JobRestartCommand) restartAlloc(alloc AllocationListStubWithJob) error {
	shortAllocID := limit(alloc.ID, c.length)

	if c.allTasks {
		c.Ui.Output(fmt.Sprintf(
			"    %s: Restarting all tasks in allocation %q for group %q",
			formatTime(time.Now()),
			shortAllocID,
			alloc.TaskGroup,
		))

		return c.client.Allocations().RestartAllTasks(&api.Allocation{ID: alloc.ID}, nil)
	}

	if c.tasks.Size() == 0 {
		c.Ui.Output(fmt.Sprintf(
			"    %s: Restarting running tasks in allocation %q for group %q",
			formatTime(time.Now()),
			shortAllocID,
			alloc.TaskGroup,
		))

		return c.client.Allocations().Restart(&api.Allocation{ID: alloc.ID}, "", nil)
	}

	// Run restarts concurrently when specific tasks were requested.
	var restarts multierror.Group
	for task := range c.tasks.Items() {
		if !alloc.HasTask(task) {
			continue
		}

		c.Ui.Output(fmt.Sprintf(
			"    %s: Restarting task %q in allocation %q for group %q",
			formatTime(time.Now()),
			task,
			shortAllocID,
			alloc.TaskGroup,
		))

		restarts.Go(func(taskName string) func() error {
			return func() error {
				err := c.client.Allocations().Restart(&api.Allocation{ID: alloc.ID}, taskName, nil)
				if err != nil {
					return fmt.Errorf("Failed to restart task %q: %w", taskName, err)
				}
				return nil
			}
		}(task))
	}
	return restarts.Wait().ErrorOrNil()
}

// stopAlloc stops an allocation and blocks until the replacement allocation is
// running.
func (c *JobRestartCommand) stopAlloc(alloc AllocationListStubWithJob) error {
	shortAllocID := limit(alloc.ID, c.length)

	c.Ui.Output(fmt.Sprintf(
		"    %s: Rescheduling allocation %q for group %q",
		formatTime(time.Now()),
		shortAllocID,
		alloc.TaskGroup,
	))

	var q *api.QueryOptions
	if c.noShutdownDelay {
		q = &api.QueryOptions{
			Params: map[string]string{"no_shutdown_delay": "true"},
		}
	}

	// Stop allocation and wait for its replacement to be running or for a
	// blocked evaluation that prevents placements for this task group to
	// happen.
	resp, err := c.client.Allocations().Stop(&api.Allocation{ID: alloc.ID}, q)
	if err != nil {
		return fmt.Errorf("Failed to stop allocation: %w", err)
	}

	// Allocations for system jobs do not get replaced by the scheduler after
	// being stopped, so an eval is needed to trigger the reconciler.
	if alloc.isSystemJob() {
		opts := api.EvalOptions{
			ForceReschedule: true,
		}
		_, _, err := c.client.Jobs().EvaluateWithOpts(*alloc.Job.ID, opts, nil)
		if err != nil {
			return fmt.Errorf("Failed evaluate job: %w", err)
		}
	}

	// errCh receives an error if anything goes wrong or nil when the
	// replacement allocation is running.
	// Use a buffered channel to prevent both goroutine from blocking trying to
	// send a result back.
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Pass the LastIndex from the Stop() call to only monitor data that was
	// created after the Stop() call.
	go c.monitorPlacementFailures(ctx, alloc, resp.LastIndex, errCh)
	go c.monitorReplacementAlloc(ctx, alloc, errCh)

	// This process may take a while, so ping user from time to time to
	// indicate the command is still alive.
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.Ui.Output(fmt.Sprintf(
				"    %s: Still waiting for allocation %q to be replaced",
				formatTime(time.Now()),
				shortAllocID,
			))
		case err := <-errCh:
			return err
		}
	}
}

// monitorPlacementFailures searches for evaluations of the allocation job that
// have placement failures.
//
// Returns an error in errCh if anything goes wrong or if there are placement
// failures for the allocation task group.
func (c *JobRestartCommand) monitorPlacementFailures(
	ctx context.Context,
	alloc AllocationListStubWithJob,
	index uint64,
	errCh chan<- error,
) {
	q := &api.QueryOptions{WaitIndex: index}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		evals, qm, err := c.client.Jobs().Evaluations(alloc.JobID, q)
		if err != nil {
			errCh <- fmt.Errorf("Failed to retrieve evaluations for job %q: %w", alloc.JobID, err)
			return
		}

		for _, eval := range evals {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Skip evaluations created before the allocation was stopped or
			// that are not blocked.
			if eval.CreateIndex < index || eval.Status != api.EvalStatusBlocked {
				continue
			}

			failures := eval.FailedTGAllocs[alloc.TaskGroup]
			if failures != nil {
				errCh <- ErrJobRestartPlacementFailure{
					EvalID:    limit(eval.ID, c.length),
					TaskGroup: alloc.TaskGroup,
					Failures:  failures,
				}
				return
			}
		}
		q.WaitIndex = qm.LastIndex
	}
}

// monitorReplacementAlloc waits for the allocation to have a follow-up
// placement and for the new allocation be running.
//
// Returns an error in errCh if anything goes wrong or nil when the new
// allocation is running.
func (c *JobRestartCommand) monitorReplacementAlloc(
	ctx context.Context,
	allocStub AllocationListStubWithJob,
	errCh chan<- error,
) {
	currentAllocID := allocStub.ID
	q := &api.QueryOptions{WaitIndex: 1}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		alloc, qm, err := c.client.Allocations().Info(currentAllocID, q)
		if err != nil {
			errCh <- fmt.Errorf("Failed to retrieve allocation %q: %w", limit(currentAllocID, c.length), err)
			return
		}

		// Follow replacement allocations. We expect the original allocation to
		// be replaced, but the replacements may be themselves replaced in
		// cases such as the allocation failing.
		if alloc.NextAllocation != "" {
			c.Ui.Output(fmt.Sprintf(
				"    %s: Allocation %q replaced by %[3]q, waiting for %[3]q to start running",
				formatTime(time.Now()),
				limit(alloc.ID, c.length),
				limit(alloc.NextAllocation, c.length),
			))
			currentAllocID = alloc.NextAllocation

			// Reset the blocking query so the Info() API call returns the new
			// allocation immediately.
			q.WaitIndex = 1
			continue
		}

		switch alloc.ClientStatus {
		case api.AllocClientStatusRunning:
			// Make sure the running allocation we found is a replacement, not
			// the original one.
			if alloc.ID != allocStub.ID {
				c.Ui.Output(fmt.Sprintf(
					"    %s: Allocation %q is %q",
					formatTime(time.Now()),
					limit(alloc.ID, c.length),
					alloc.ClientStatus,
				))
				errCh <- nil
				return
			}

		default:
			if c.verbose {
				c.Ui.Output(c.Colorize().Color(fmt.Sprintf(
					"[dark_gray]    %s: Allocation %q is %q[reset]",
					formatTime(time.Now()),
					limit(alloc.ID, c.length),
					alloc.ClientStatus,
				)))
			}
		}

		q.WaitIndex = qm.LastIndex
	}
}

// handleSignal receives input signals and blocks the activeCh until the user
// confirms how to proceed.
//
// Exit immediately if the user confirms the interrupt, otherwise resume the
// command and feed activeCh to unblock it.
func (c *JobRestartCommand) handleSignal(sigsCh chan os.Signal, activeCh chan any) {
	for {
		select {
		case <-sigsCh:
			// Consume activeCh to prevent the main loop from proceeding.
			select {
			case <-activeCh:
			default:
			}

			if c.shouldExit() {
				c.Ui.Output("\nCanceling job restart process")
				os.Exit(0)
			}
		case activeCh <- struct{}{}:
		}
	}
}

// isErrorRecoverable returns true when the error is likely to impact all
// restarts and so there is not reason to keep going.
func (c *JobRestartCommand) isErrorRecoverable(err error) bool {
	if err == nil {
		return true
	}

	if errors.Is(err, ErrJobRestartPlacementFailure{}) {
		return false
	}

	if strings.Contains(err.Error(), api.PermissionDeniedErrorContent) {
		return false
	}

	return true
}

// errorFormat returns a multierror.ErrorFormatFunc that indents each line,
// except for the first one, of the resulting error string with the given
// number of spaces.
func (c *JobRestartCommand) errorFormat(indent int) func([]error) string {
	return func(es []error) string {
		points := make([]string, len(es))
		for i, err := range es {
			points[i] = fmt.Sprintf("* %s", strings.TrimSpace(err.Error()))
		}

		out := fmt.Sprintf(
			"%s occurred while restarting job:\n%s",
			english.Plural(len(es), "error", "errors"),
			strings.Join(points, "\n"),
		)
		return indentString(out, indent)
	}
}

// AllocationListStubWithJob combines an AllocationListStub with its
// corresponding job at the right version.
type AllocationListStubWithJob struct {
	*api.AllocationListStub
	Job *api.Job
}

// HasTask returns true if the allocation has the given task in the specific
// job version it was created.
func (a *AllocationListStubWithJob) HasTask(name string) bool {
	// Check task state first as it's the fastest and most reliable source.
	if _, ok := a.TaskStates[name]; ok {
		return true
	}

	// But task states are only set when the client updates its allocations
	// with the server, so they may not be available yet. Lookup the task in
	// the job version as a fallback.
	if a.Job == nil {
		return false
	}

	var taskGroup *api.TaskGroup
	for _, tg := range a.Job.TaskGroups {
		if tg.Name == nil || *tg.Name != a.TaskGroup {
			continue
		}
		taskGroup = tg
	}
	if taskGroup == nil {
		return false
	}

	for _, task := range taskGroup.Tasks {
		if task.Name == name {
			return true
		}
	}

	return false
}

// IsRunning returns true if the allocation's ClientStatus or DesiredStatus is
// running.
func (a *AllocationListStubWithJob) IsRunning() bool {
	return a.ClientStatus == api.AllocClientStatusRunning ||
		a.DesiredStatus == api.AllocDesiredStatusRun
}

// isSystemJob returns true if allocation's job type
// is "system", false otherwise
func (a *AllocationListStubWithJob) isSystemJob() bool {
	return a.Job != nil && a.Job.Type != nil && *a.Job.Type == api.JobTypeSystem
}
