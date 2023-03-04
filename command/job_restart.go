package command

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-set"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/hashicorp/nomad/helper"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

const (
	// jobRestartWaitAsk is the special token used to indicate that the
	// command should ask user for confirmation between batches.
	jobRestartWaitAsk = "ask"

	// jobRestartFailedToPlaceNewAllocation is the error returned when a
	// reschedule fails to create new placements.
	jobRestartFailedToPlaceNewAllocation = "Failed to place new allocation"
)

var (
	// jobRestartBatchSizeValueRegex validates that the value passed to
	// -batch-size is an integer optionally followed by a % sign.
	//
	// Use ^...$ to make sure we're matching over the entire input to avoid
	// partial matches such as 10%20%.
	jobRestartBatchSizeValueRegex = regexp.MustCompile(`^(\d+)%?$`)
)

type JobRestartCommand struct {
	Meta

	// ui is a cli.ConcurrentUi that wraps the UI passed in Meta so that
	// goroutines can safely write to the terminal output concurrently.
	ui *cli.ConcurrentUi

	// client is the Nomad API client shared by all functions in the command to
	// reuse the same connection.
	client *api.Client

	// Configuration values read and parsed from command flags and args.
	allTasks         bool
	batchSize        int
	batchSizePercent bool
	batchWait        time.Duration
	batchWaitAsk     bool
	failOnError      bool
	groups           *set.Set[string]
	jobID            string
	noShutdownDelay  bool
	reschedule       bool
	tasks            *set.Set[string]
	verbose          bool
	length           int
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
  'alloc-lifecycle' and 'read-job' capabilities for the job's namespace.

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

  -fail-on-error
    Fail command as soon as an allocation restart fails. By default errors are
    accumulated and displayed when the command exits.

  -group=<group-name>
    Only restart allocations for the given group. Can be specified multiple
    times. If no group is set all allocations for the job are restarted.

  -no-shutdown-delay
    Ignore the group and task 'shutdown_delay' configuration so there is no
    delay between service deregistration and task shutdown or restart. Note
    that using this flag will result in failed network connections to the
    allocation being restarted.

  -reschedule
    If set, allocations are stopped and rescheduled instead of restarted
    in-place. Since the group is not modified the restart does not create a new
    deployment, and so values defined in 'update' blocks, such as
    'max_parallel', are not taken into account. This option cannot be used with
    '-task'.

  -task=<task-name>
    Specify the task to restart. Can be specified multiple times. If groups are
    also specified the task must exist in at least one of them. If no task is
    set only tasks that are currently running are restarted. For example,
    non-sidecar tasks that already ran are not restarted unless '-all-tasks' is
    used instead. This option cannot be used with '-all-tasks' or
    '-reschedule'.

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
			"-fail-on-error":     complete.PredictNothing,
			"-no-shutdown-delay": complete.PredictNothing,
			"-reschedule":        complete.PredictNothing,
			"-task":              complete.PredictAnything,
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
	c.ui = &cli.ConcurrentUi{Ui: c.Ui}

	// Parse and validate command line arguments.
	code, err := c.parseAndValidate(args)
	if err != nil {
		c.ui.Error(err.Error())
		c.ui.Error(commandErrorText(c))
		return code
	}
	if code != 0 {
		return code
	}

	c.client, err = c.Meta.Client()
	if err != nil {
		c.ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	// Use prefix matching to find job.
	var namespace string
	c.jobID, namespace, err = c.JobIDByPrefix(c.client, c.jobID, nil)
	if err != nil {
		c.ui.Error(err.Error())
		return 1
	}
	c.client.SetNamespace(namespace)

	// Retrieve the job history so we can properly determine if a group or task
	// exists in the specific allocation job version.
	jobVersions, _, _, err := c.client.Jobs().Versions(c.jobID, false, nil)
	if err != nil {
		c.ui.Error(fmt.Sprintf("Error retrieving versions of job %q: %s", c.jobID, err))
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
		c.ui.Error(fmt.Sprintf("Error retrieving allocations for job %q: %v", c.jobID, err))
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
		c.ui.Output("No allocations to restart")
		return 0
	}

	// Calculate absolute batch size based on the number of eligible
	// allocations if the value provided is a percentage.
	// Round values up to increase parallelism.
	if c.batchSizePercent {
		c.batchSize = int(math.Ceil(float64(len(restartAllocs)*c.batchSize) / 100))
	}

	// Restart allocations in batches.
	c.ui.Output(c.Colorize().Color(fmt.Sprintf(
		"[bold]==> %s: Restarting %s[reset]",
		formatTime(time.Now()),
		english.Plural(len(restartAllocs), "allocation", "allocations"),
	)))

	// Handle SIGINT to prevent accidental cancellations of the long-lived
	// restart loop. activeCh is blocked while a signal is being handled to
	// prevent new work from starting while the user is deciding if they want
	// to cancel the command or not.
	activeCh := make(chan any)
	sigsCh := make(chan os.Signal, 1)
	signal.Notify(sigsCh, syscall.SIGINT)
	go c.handleSignal(sigsCh, activeCh)

	var restarts multierror.Group
	for restartCount, alloc := range restartAllocs {
		// Block and wait before each iteration if the command is handling an
		// interrupt signal.
		<-activeCh

		// Print new batch header every time we restart a multiple of the batch
		// size which indicate we're starting a new batch.
		// Skip batch header if batch size is one because it's redundant.
		if restartCount%c.batchSize == 0 && c.batchSize > 1 {
			batchNumber := restartCount/c.batchSize + 1
			remaining := len(restartAllocs) - restartCount

			c.ui.Output(c.Colorize().Color(fmt.Sprintf(
				"[bold]==> %s: Restarting %s batch of %d allocations[reset]",
				formatTime(time.Now()),
				humanize.Ordinal(batchNumber),
				helper.Min(c.batchSize, remaining),
			)))
		}

		// Restart allocation. Wrap the callback function to capture the
		// allocID loop variable and prevent it from changing inside the
		// goroutine at each iteration.
		restarts.Go(func(allocStubWithJob AllocationListStubWithJob) func() error {
			return func() error {
				return c.handleAlloc(allocStubWithJob)
			}
		}(alloc))

		// Check if we restarted enough allocations to complete a batch or if
		// we restarted the last allocation.
		batchComplete := (restartCount+1)%c.batchSize == 0
		restartComplete := restartCount+1 == len(restartAllocs)
		if batchComplete || restartComplete {
			// Block and wait for restarts to finish and if the command is
			// handling an interrupt signal.
			mErr := restarts.Wait()
			<-activeCh

			// Exit early if an error happens and -fail-on-error is set or if
			// the error will happen for all restarts.
			err := mErr.ErrorOrNil()
			if err != nil && (c.failOnError || c.isErrNotRecoverable(err)) {
				break
			}

			// Exit loop before sleeping or asking for user input if we're done
			// with the last batch.
			if restartComplete {
				break
			}

			// Exit early if -batch-wait=ask and user provided a negative
			// answer.
			if c.batchWaitAsk && !c.shouldProceed() {
				c.ui.Output("\nJob restart canceled.")
				return 0
			}

			// Sleep if -batch-wait is set of if -batch-wait=ask and user
			// provided an interval.
			if c.batchWait > 0 {
				c.ui.Output(c.Colorize().Color(fmt.Sprintf(
					"[bold]==> %s: Waiting %s before restarting the next batch[reset]",
					formatTime(time.Now()),
					c.batchWait,
				)))
				time.Sleep(c.batchWait)
			}
		}
	}

	c.ui.Output(c.Colorize().Color(fmt.Sprintf(
		"[bold]==> %s: Finished job restart[reset]",
		formatTime(time.Now()),
	)))

	err = restarts.Wait().ErrorOrNil()
	if err != nil {
		if mErr, ok := err.(*multierror.Error); ok {
			// Format multierror because some errors may be deeply nested
			// resulting in very long outputs.
			mErr.ErrorFormat = c.errorFormat
		}
		c.ui.Error(fmt.Sprintf("\nErrors while restarting job:\n%s", strings.TrimSpace(err.Error())))
		return 1
	}

	c.ui.Output("\nAll allocations restarted successfully!")
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
	flags.Usage = func() { c.ui.Output(c.Help()) }
	flags.BoolVar(&c.allTasks, "all-tasks", false, "")
	flags.StringVar(&batchSizeStr, "batch-size", "1", "")
	flags.StringVar(&batchWaitStr, "batch-wait", "0s", "")
	flags.BoolVar(&c.failOnError, "fail-on-error", false, "")
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
		return 1, fmt.Errorf("Invalid -batch-size value %q: %v", batchSizeStr, err)
	}
	if c.batchSize == 0 {
		return 1, fmt.Errorf(
			"Invalid -batch-size value %q: number value must be greater than zero",
			batchSizeStr,
		)
	}

	// Parse and validate -batch-wait.
	if strings.ToLower(batchWaitStr) == jobRestartWaitAsk {
		if !isTty() {
			return 1, fmt.Errorf(
				"Invalid -batch-wait value %[1]q: %[1]q cannot be used when terminal is not interactive",
				jobRestartWaitAsk,
			)
		}
		c.batchWaitAsk = true
	} else {
		c.batchWait, err = time.ParseDuration(batchWaitStr)
		if err != nil {
			return 1, fmt.Errorf("Invalid -batch-wait value %q: %v", batchWaitStr, err)
		}
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
				c.ui.Output(c.Colorize().Color(fmt.Sprintf(
					"[dark_gray]    %s: Skipping allocation %q because desired status is %q and client status is %q[reset]",
					formatTime(time.Now()),
					shortAllocID,
					stub.ClientStatus,
					stub.DesiredStatus,
				)))
			}
			continue
		}

		// Skip allocations for groups that were not requested.
		if c.groups.Size() > 0 {
			if !c.groups.Contains(stub.TaskGroup) {
				if c.verbose {
					c.ui.Output(c.Colorize().Color(fmt.Sprintf(
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
			for _, taskName := range c.tasks.Slice() {
				if stub.HasTask(taskName) {
					hasTask = true
					break
				}
			}

			if !hasTask {
				if c.verbose {
					c.ui.Output(c.Colorize().Color(fmt.Sprintf(
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

// shouldProceed blocks and waits for the user to provide a valid input.
// Returns true if the answer is positive.
func (c *JobRestartCommand) shouldProceed() bool {
	for {
		answer, err := c.ui.Ask(fmt.Sprintf(
			"==> %s: Proceed with next batch? [Y/n/<duration>]",
			formatTime(time.Now()),
		))
		if err != nil {
			if err.Error() == "interrupted" {
				return false
			}
			c.ui.Output(err.Error())
			continue
		}

		switch strings.TrimSpace(strings.ToLower(answer)) {
		case "y", "yes", "":
			return true
		case "n", "no":
			return false
		default:
			// Check if user passed a time duration and configure the command
			// to use that moving forward.
			c.batchWait, err = time.ParseDuration(answer)
			if err == nil {
				c.batchWaitAsk = false

				c.ui.Output(c.Colorize().Color(fmt.Sprintf(
					"[bold]==> %s: Proceeding restarts with new wait time of %s[reset]",
					formatTime(time.Now()),
					c.batchWait,
				)))
				return true
			}

			c.ui.Output(fmt.Sprintf(
				"    %s: Invalid option %q",
				formatTime(time.Now()),
				answer,
			))
		}
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
		return fmt.Errorf("%s %v", msg, err)
	}
	return nil
}

// restartAlloc restarts an allocation in place and blocks until the tasks are
// done restarting.
func (c *JobRestartCommand) restartAlloc(alloc AllocationListStubWithJob) error {
	shortAllocID := limit(alloc.ID, c.length)

	if c.allTasks {
		c.ui.Output(fmt.Sprintf(
			"    %s: Restarting all tasks in allocation %q for group %q",
			formatTime(time.Now()),
			shortAllocID,
			alloc.TaskGroup,
		))

		return c.client.Allocations().RestartAllTasks(&api.Allocation{ID: alloc.ID}, nil)
	}

	if c.tasks.Size() == 0 {
		c.ui.Output(fmt.Sprintf(
			"    %s: Restarting running tasks in allocation %q for group %q",
			formatTime(time.Now()),
			shortAllocID,
			alloc.TaskGroup,
		))

		return c.client.Allocations().Restart(&api.Allocation{ID: alloc.ID}, "", nil)
	}

	// Run restarts concurrently when specific tasks were requested.
	var restarts multierror.Group
	for _, task := range c.tasks.Slice() {
		if !alloc.HasTask(task) {
			continue
		}

		c.ui.Output(fmt.Sprintf(
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
					return fmt.Errorf("Failed to restart task %q: %v", taskName, err)
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

	c.ui.Output(fmt.Sprintf(
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
		return fmt.Errorf("Failed to stop allocation %q: %v", shortAllocID, err)
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
	go c.monitorReplacementAlloc(ctx, alloc, resp.LastIndex, errCh)

	// If we receive an error and nil it's safe to ignore the error since the
	// nil result is what we are looking for.
	return <-errCh
}

// monitorPlacementFailures searches for evaluations of the allocation job that
// have placement failures.
//
// Returns an error in errCh if anything goes wrong or if there are placement
// failures for the allocation task group.
func (c *JobRestartCommand) monitorPlacementFailures(ctx context.Context, alloc AllocationListStubWithJob, index uint64, errCh chan<- error) {
	q := &api.QueryOptions{WaitIndex: index}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		evals, qm, err := c.client.Jobs().Evaluations(alloc.JobID, q)
		if err != nil {
			errCh <- fmt.Errorf("Failed to retrieve evaluations for job %q: %v", alloc.JobID, err)
			return
		}

		for _, eval := range evals {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Skip evaluations created before the allocation was stopped.
			if eval.CreateIndex < index {
				continue
			}

			failures := eval.FailedTGAllocs[alloc.TaskGroup]
			if failures != nil {
				errCh <- fmt.Errorf("%s:\n%s",
					jobRestartFailedToPlaceNewAllocation,
					formatAllocMetrics(failures, false, strings.Repeat(" ", 4)),
				)
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
func (c *JobRestartCommand) monitorReplacementAlloc(ctx context.Context, alloc AllocationListStubWithJob, index uint64, errCh chan<- error) {
	q := &api.QueryOptions{WaitIndex: index}
	var nextAllocID string
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		oldAlloc, qm, err := c.client.Allocations().Info(alloc.ID, q)
		if err != nil {
			errCh <- fmt.Errorf("Failed to retrieve allocation %q: %v", limit(alloc.ID, c.length), err)
			return
		}
		if oldAlloc.NextAllocation != "" {
			nextAllocID = oldAlloc.NextAllocation
			break
		}
		q.WaitIndex = qm.LastIndex
	}

	// Reset the blocking query to the initial index because old allocation
	// update may happen after the new allocation transitioned to "running".
	q = &api.QueryOptions{WaitIndex: index}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		newAlloc, qm, err := c.client.Allocations().Info(nextAllocID, q)
		if err != nil {
			errCh <- fmt.Errorf("Failed to retrieve replacement allocation %q: %v", limit(nextAllocID, c.length), err)
			return
		}
		if newAlloc.ClientStatus == api.AllocClientStatusRunning {
			errCh <- nil
			return
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
				c.ui.Output("\nCanceling job restart process")
				os.Exit(0)
			}
		case activeCh <- struct{}{}:
		}
	}
}

// shouldExit asks the user for confirmation if they would like to interrupt
// the command. Returns true if the answer is positive.
func (c *JobRestartCommand) shouldExit() bool {
	if !isTty() {
		return true
	}

	exitQuestion := `
Are you sure you want to stop the restart process?
Allocations not restarted yet will not be restarted. [y/N]`

	for {
		answer, err := c.ui.Ask(exitQuestion)
		if err != nil {
			if err.Error() != "interrupted" {
				c.ui.Error(err.Error())
			}
			return true
		}

		switch strings.TrimSpace(strings.ToLower(answer)) {
		case "y", "yes":
			return true
		case "n", "no", "":
			return false
		default:
			c.ui.Output(fmt.Sprintf("Invalid option %q", answer))
		}
	}
}

// isErrNotRecoverable returns true when the error is likely to impact all
// restarts and so there is not reason to keep going.
func (c *JobRestartCommand) isErrNotRecoverable(err error) bool {
	patterns := []string{
		api.PermissionDeniedErrorContent,
		jobRestartFailedToPlaceNewAllocation,
	}
	for _, pattern := range patterns {
		if strings.Contains(err.Error(), pattern) {
			return true
		}
	}
	return false
}

// errorFormat is a multierror.ErrorFormatFunc that uses 2 spaces instead of a
// tab before each error bullet point. This prevents deeply nested errors from
// outputting very long lines.
func (c *JobRestartCommand) errorFormat(es []error) string {
	space := strings.Repeat(" ", 2)

	if len(es) == 1 {
		return fmt.Sprintf("1 error occurred:\n%s* %s\n\n", space, es[0])
	}

	points := make([]string, len(es))
	for i, err := range es {
		points[i] = fmt.Sprintf("* %s", err)
	}

	return fmt.Sprintf(
		"%d errors occurred:\n%s%s\n\n",
		len(es), space, strings.Join(points, fmt.Sprintf("\n%s", space)))
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
