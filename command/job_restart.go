package command

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/hashicorp/nomad/helper"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

// jobRestartBatchSizeRegex validates that the value passed to -batch is an
// integer optionally followed by a % sign.
//
// Use ^...$ to make sure we're matching over the entire input to avoid partial
// matches such as 10%20%.
var jobRestartBatchValueRegex = regexp.MustCompile(`^(\d+)%?$`)

const (
	jobRestartBatchSizeAll = "all"
	jobRestartBathWaitAsk  = "ask"
)

type JobRestartCommand struct {
	Meta

	// ui is a cli.ConcurrentUi that wraps the UI passed in Meta so that
	// goroutines can safely write to the terminal output concurrently.
	ui cli.Ui

	// Configuration values parsed from the flags and args.
	allTasks         bool
	batchSize        int
	batchSizePercent bool
	batchSizeAll     bool
	batchWait        time.Duration
	batchWaitAsk     bool
	failOnError      bool
	jobID            string
	noShutdownDelay  bool
	reschedule       bool
	task             string
	verbose          bool
}

func (c *JobRestartCommand) Help() string {
	helpText := `
Usage: nomad job restart [options] <job> <task>

  Restart allocations for a particular job. This command can restart
  allocations in batches and wait until all restarted allocations are running
  again before proceeding to the next batch. It is also possible to wait some
  additional time before proceeding.

  Allocations can be restarted in-place or rescheduled. When restarting
  in-place the command may target only a specific task in the allocations. By
  default the command only restarts running tasks in-place. Use the option
  '-all-tasks' to restart all tasks, even the ones that have already completed,
  or the '-reschedule' option to stop and reschedule allocations.

  When ACLs are enabled, this command requires a token with the
  'alloc-lifecycle', 'read-job', and 'list-jobs' capabilities for the
  allocation's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsDefault) + `

Restart Options:

  -all-tasks
    If set, all tasks in the allocations are restarted, even the ones that
    have already completed. By default only running tasks are restarted. This
    option cannot be used with '-task' or the '<task>' argument.

  -batch <n | n% | all>
    Number of allocations to restart at once. It may be defined as a percentage
    value of the sum of all counts in the job groups or as 'all' to restart all
    allocations in a single batch. A batch size of 100% may require multiple
    batches if you have more allocations running than defined in the job, such
    as when canaries are waiting for promotion. Defaults to 1.

  -fail-on-error
    Fail command as soon as an allocation restart fails. By default errors are
    accumulated and displayed as warnings when the command exits.

  -no-shutdown-delay
    Ignore the group and task 'shutdown_delay' configuration so there is no
    delay between service deregistration and task shutdown or restart. Note
    that using this flag will result in failed network connections to the
    allocation being restarted.

  -reschedule
    If set, allocations are stopped and rescheduled instead of restarted
    in-place. This option cannot be use with '-task' or the '<task>' argument.

  -task <task-name>
    Specify the individual task to restart. If task name is given with both an
    argument and the '-task' option, preference is given to the '-task' option.
    By default only running tasks are restarted. This option cannot be used
    with '-all-tasks' or '-reschedule'.

  -verbose
    Display full information.

  -wait <duration | ask>
    Time to wait between restart batches. If set to 'ask' the command halts
    between batches and waits for user input on how to proceed. If the answer
    is a time duration all next batches will use this new value. Defaults to 0.
`
	return strings.TrimSpace(helpText)
}

func (c *JobRestartCommand) Synopsis() string {
	return "Restart all allocations for a job"
}

func (c *JobRestartCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-all-tasks":         complete.PredictNothing,
			"-batch":             complete.PredictAnything,
			"-wait":              complete.PredictAnything,
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

		switch len(a.Completed) {
		case 0:
			// Predict job name as first arg.
			resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Jobs, nil)
			if err != nil {
				return []string{}
			}
			return resp.Matches[contexts.Jobs]

		case 1:
			// Predict task name as second arg.
			resp, _, err := client.Search().FuzzySearch(a.Last, contexts.Jobs, nil)
			if err != nil {
				return []string{}
			}

			matches := make([]string, 0, len(resp.Matches[contexts.Tasks]))
			for _, m := range resp.Matches[contexts.Tasks] {
				if len(m.Scope) < 2 || m.Scope[1] != a.LastCompleted {
					continue
				}
				matches = append(matches, m.ID)
			}
			return matches
		default:
			// Predict nothing for other args.
			return nil
		}
	})
}

func (c *JobRestartCommand) Name() string { return "job restart" }

func (c *JobRestartCommand) Run(args []string) int {
	// Use a concurrent UI to allow goroutines to output messages to the
	// terminal.
	c.ui = &cli.ConcurrentUi{Ui: c.Ui}

	err := c.parseAndValidate(args)
	if err != nil {
		c.ui.Error(err.Error())
		c.ui.Error(commandErrorText(c))
		return 1
	}

	// Truncate IDs unless full length is requested.
	length := shortId
	if c.verbose {
		length = fullId
	}

	// Get the HTTP client.
	client, err := c.Meta.Client()
	if err != nil {
		c.ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	// Check if the job exists.
	jobs, _, err := client.Jobs().PrefixList(c.jobID)
	if err != nil {
		c.ui.Error(fmt.Sprintf("Error listing jobs: %s", err))
		return 1
	}
	if len(jobs) == 0 {
		c.ui.Error(fmt.Sprintf("No job(s) with prefix or id %q found", c.jobID))
		return 1
	}
	if len(jobs) > 1 {
		if (c.jobID != jobs[0].ID) || (c.allNamespaces() && jobs[0].ID == jobs[1].ID) {
			c.ui.Error(fmt.Sprintf("Prefix matched multiple jobs\n\n%s", createStatusListOutput(jobs, c.allNamespaces())))
			return 1
		}
	}

	// Update job ID with full value in case user passed a prefix.
	c.jobID = jobs[0].ID
	namespace := jobs[0].JobSummary.Namespace
	q := &api.QueryOptions{Namespace: namespace}

	// Fetch job details.
	job, _, err := client.Jobs().Info(c.jobID, q)
	if err != nil {
		c.ui.Error(fmt.Sprintf("Error retrieving job %s: %v", c.jobID, err))
		return 1
	}

	// Check that task exists in the job if provided.
	if c.task != "" {
		taskFound := false
		for _, tg := range job.TaskGroups {
			for _, t := range tg.Tasks {
				if t.Name == c.task {
					taskFound = true
					break
				}
			}
			if taskFound {
				break
			}
		}
		if !taskFound {
			c.ui.Error(fmt.Sprintf("No task named %q found in job %q", c.task, c.jobID))
			return 1
		}
	}

	// Calculate absolute batch size based on the total job group counts if the
	// value provided is a percentage.
	if c.batchSizePercent {
		totalCount := 0
		for _, tg := range job.TaskGroups {
			totalCount += *tg.Count
		}
		c.batchSize = totalCount * c.batchSize / 100
	}

	// Fetch job allocations.
	allocs, _, err := client.Jobs().Allocations(c.jobID, false, q)
	if err != nil {
		c.ui.Error(fmt.Sprintf("Error retrieving allocations for jobs %s: %v", c.jobID, err))
		return 1
	}

	// Restart allocations and collect results.
	errCh := make(chan error)
	outCh := make(chan error)
	go c.collectResults(errCh, outCh)

	if c.batchSizeAll {
		c.ui.Output(c.Colorize().Color(fmt.Sprintf(
			"[bold]==> %s: Restarting all allocations in a single batch[reset]",
			formatTime(time.Now()),
		)))
	}

	var wg sync.WaitGroup
	restarts := 0
	for i := 0; i < len(allocs); i++ {
		alloc := allocs[i]
		allocID := limit(alloc.ID, length)

		// Skip allocations that are not running.
		allocRunning := alloc.ClientStatus == api.AllocClientStatusRunning && alloc.DesiredStatus == api.AllocDesiredStatusRun
		if !allocRunning {
			if c.verbose {
				c.ui.Info(c.Colorize().Color(fmt.Sprintf(
					"[dark_gray]    %s: Skipping allocation %s because desired status is %q and client status is %q[reset]",
					formatTime(time.Now()),
					allocID,
					alloc.ClientStatus,
					alloc.DesiredStatus,
				)))
			}
			continue
		}

		// Check if we restarted enough allocations to complete a batch.
		if !c.batchSizeAll && restarts%c.batchSize == 0 {
			if restarts > 0 {
				// Wait for restarts to finish.
				wg.Wait()

				// Ask user to proceed or sleep.
				if c.batchWaitAsk {
					if !c.shouldProceed() {
						c.ui.Output("\nJob restart canceled.")
						return 0
					}
				} else if c.batchWait > 0 {
					c.ui.Output(c.Colorize().Color(fmt.Sprintf(
						"[bold]==> %s: Waiting %s before restarting the next batch[reset]",
						formatTime(time.Now()),
						c.batchWait,
					)))
					time.Sleep(c.batchWait)
				}
			}

			c.ui.Output(c.Colorize().Color(fmt.Sprintf(
				"[bold]==> %s: Restarting %s batch of %s[reset]",
				formatTime(time.Now()),
				humanize.Ordinal(restarts/c.batchSize+1),
				english.Plural(c.batchSize, "allocation", "allocations"),
			)))
		}

		// Restart allocation and increase counters.
		if c.task != "" {
			c.ui.Output(fmt.Sprintf(
				"    %s: Restarting task %q in allocation %q for group %q",
				formatTime(time.Now()),
				c.task,
				allocID,
				alloc.TaskGroup,
			))
		} else {
			c.ui.Output(fmt.Sprintf(
				"    %s: Restarting allocation %q for group %q",
				formatTime(time.Now()),
				allocID,
				alloc.TaskGroup,
			))
		}
		wg.Add(1)
		restarts++
		go c.handleAlloc(&wg, alloc.ID, namespace, errCh)
	}

	// Wait for the last batch to complete and collect any errors to display
	// them as warnings.
	wg.Wait()
	close(errCh)

	c.ui.Output(c.Colorize().Color(fmt.Sprintf(
		"[bold]==> %s: Finished restarting %s[reset]",
		formatTime(time.Now()),
		english.Plural(restarts, "allocation", "allocations"),
	)))

	mErr := <-outCh
	if mErr != nil {
		c.ui.Warn(fmt.Sprintf("\n%s", c.FormatWarnings(
			"Job Restarts",
			helper.MergeMultierrorWarnings(mErr),
		)))
		return 1
	}

	c.ui.Output("\nAll allocations restarted successfully!")
	return 0
}

// parseAndValidate parses and validates the arguments passed to the command.
func (c *JobRestartCommand) parseAndValidate(args []string) error {
	var batchSizeStr string
	var batchWaitStr string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.ui.Output(c.Help()) }
	flags.BoolVar(&c.allTasks, "all-tasks", false, "")
	flags.StringVar(&batchSizeStr, "batch", "1", "")
	flags.StringVar(&batchWaitStr, "wait", "0s", "")
	flags.BoolVar(&c.failOnError, "fail-on-error", false, "")
	flags.BoolVar(&c.noShutdownDelay, "no-shutdown-delay", false, "")
	flags.BoolVar(&c.reschedule, "reschedule", false, "")
	flags.StringVar(&c.task, "task", "", "")
	flags.BoolVar(&c.verbose, "verbose", false, "")

	err := flags.Parse(args)
	if err != nil {
		return fmt.Errorf("Error parsing command: %v", err)
	}

	// Check that we got one job or a job and a task.
	args = flags.Args()
	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("This command takes one or two argument: <job> <task-name>")
	}

	// Check for <task> argument if the -task flag was not provided.
	if c.task == "" && len(args) == 2 {
		c.task = strings.TrimSpace(args[1])
	}
	c.jobID = strings.TrimSpace(args[0])

	// Parse and validate -batch.
	if strings.ToLower(batchSizeStr) == jobRestartBatchSizeAll {
		c.batchSizeAll = true
	} else {
		matches := jobRestartBatchValueRegex.FindStringSubmatch(batchSizeStr)
		if len(matches) != 2 {
			return fmt.Errorf("Invalid -batch value %s: batch size must be an integer or a percentage", batchSizeStr)
		}

		c.batchSizePercent = strings.HasSuffix(batchSizeStr, "%")
		c.batchSize, err = strconv.Atoi(matches[1])
		if err != nil {
			return fmt.Errorf("Invalid -batch value %s: %v", batchSizeStr, err)
		}
		if c.batchSize == 0 {
			return fmt.Errorf("Invalid -batch value %s: number value must be greater than zero", batchSizeStr)
		}
	}

	// Parse and validate -wait.
	if strings.ToLower(batchWaitStr) == jobRestartBathWaitAsk {
		if !isTty() {
			return fmt.Errorf("The -wait option cannot be 'ask' when terminal is not interactive")
		}
		c.batchWaitAsk = true
	} else {
		c.batchWait, err = time.ParseDuration(batchWaitStr)
		if err != nil {
			return fmt.Errorf("Invalid -wait value %s: %v", batchWaitStr, err)
		}
	}

	// -all-tasks conflicts with -task and <task>.
	if c.allTasks && c.task != "" {
		return fmt.Errorf("The -all-tasks option cannot be used when a task is defined")
	}

	// -reschedule conflicts with -task and <task>.
	if c.reschedule && c.task != "" {
		return fmt.Errorf("The -reschedule option cannot be used when a task is defined")
	}

	return nil
}

// shouldProceed blocks and waits for the user to provide a valid input.
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
					"[bold]==> %s: Proceeding with restarts with new wait time of %s[reset]",
					formatTime(time.Now()),
					c.batchWait,
				)))
				return true
			}

			c.ui.Output(fmt.Sprintf(
				"    %s: Invalid option '%s'",
				formatTime(time.Now()),
				answer,
			))
		}
	}
}

// handleAlloc stops or restarts an allocation in-place. Blocks until the
// allocation is done restarting or the replacement allocation is running.
//
// It's called in goroutine so results must be returned via the resultCh.
func (c *JobRestartCommand) handleAlloc(wg *sync.WaitGroup, allocID string, namespace string, resultCh chan<- error) {
	defer wg.Done()

	client, err := c.Meta.Client()
	if err != nil {
		resultCh <- fmt.Errorf("Error initializing client: %v", err)
		return
	}

	alloc, _, err := client.Allocations().Info(allocID, nil)
	if err != nil {
		resultCh <- fmt.Errorf("Error retrieving allocation %s: %v", allocID, err)
		return
	}

	// Stopping an allocation triggers a reschedule.
	if c.reschedule {
		err = c.stopAlloc(client, alloc)
	} else {
		err = c.restartAlloc(client, alloc)
	}
	if err != nil {
		resultCh <- err
		return
	}
}

// restartAlloc restarts an allocation in place and blocks until the allocation
// is done restarting.
func (c *JobRestartCommand) restartAlloc(client *api.Client, alloc *api.Allocation) error {
	if c.allTasks {
		return client.Allocations().RestartAllTasks(alloc, nil)
	}
	return client.Allocations().Restart(alloc, c.task, nil)
}

// stopAlloc stops an allocation and blocks until the replacement allocation is
// running.
func (c *JobRestartCommand) stopAlloc(client *api.Client, alloc *api.Allocation) error {
	var q *api.QueryOptions
	if c.noShutdownDelay {
		q = &api.QueryOptions{
			Params: map[string]string{"no_shutdown_delay": "true"},
		}
	}

	// Stop allocation and use a blocking query to wait for the eval to
	// complete, indicating that the scheduler finished processing it.
	resp, err := client.Allocations().Stop(alloc, q)
	if err != nil {
		return fmt.Errorf("Error stopping allocation %s: %v", alloc.ID, err)
	}

	q = &api.QueryOptions{WaitIndex: 1}
	var qm *api.QueryMeta
	var eval *api.Evaluation
	for {
		eval, qm, err = client.Evaluations().Info(resp.EvalID, q)
		if err != nil {
			return fmt.Errorf("Error retrieving eval %s: %v", resp.EvalID, err)
		}
		if eval.Status != api.EvalStatusPending {
			break
		}
		q.WaitIndex = qm.LastIndex
	}

	switch eval.Status {
	case api.EvalStatusComplete, api.EvalStatusCancelled:
	default:
		return fmt.Errorf("Eval %s is %s: %s", eval.ID, eval.Status, eval.StatusDescription)
	}

	var mErr *multierror.Error
	for tg, metrics := range eval.FailedTGAllocs {
		failures := metrics.CoalescedFailures + 1
		mErr = multierror.Append(mErr, fmt.Errorf(
			"Task Group %q (failed to place %s):\n%s",
			tg,
			english.Plural(failures, "allocation", "allocations"),
			formatAllocMetrics(metrics, false, strings.Repeat(" ", 4)),
		))
	}
	if err := mErr.ErrorOrNil(); err != nil {
		return err
	}

	// Use a blocking query to re-fetch the allocation that was just stopped to
	// find the follow-up allocation.
	q = &api.QueryOptions{WaitIndex: 1}
	var nextAllocID string
	for {
		oldAlloc, qm, err := client.Allocations().Info(alloc.ID, q)
		if err != nil {
			return fmt.Errorf("Error retrieving allocation %s: %v", alloc.ID, err)
		}
		if oldAlloc.NextAllocation != "" {
			nextAllocID = oldAlloc.NextAllocation
			break
		}
		q.WaitIndex = qm.LastIndex
	}

	// Use a blocking query to wait for the follow-up allocation to be running.
	q = &api.QueryOptions{WaitIndex: 1}
	for {
		newAlloc, qm, err := client.Allocations().Info(nextAllocID, q)
		if err != nil {
			return fmt.Errorf("Error retrieving allocation %s: %v", alloc.ID, err)
		}
		if newAlloc.ClientStatus == api.AllocClientStatusRunning {
			break
		}
		q.WaitIndex = qm.LastIndex
	}

	return nil
}

// collectResults accumulates any error received in resultCh in a multierror.
// It must send the accumulated errors in outCh to indicate that it is done.
//
// If the command is set to fail on error it forces the command to exit with
// status code 1 when an error is received.
func (c *JobRestartCommand) collectResults(resultCh <-chan error, outCh chan<- error) {
	var mErr *multierror.Error

	defer func() {
		outCh <- mErr.ErrorOrNil()
	}()

	for err := range resultCh {
		if err != nil {
			if c.failOnError {
				c.ui.Error(c.Colorize().Color(fmt.Sprintf(
					"[bold]\n%s[reset]",
					strings.TrimSpace(err.Error()),
				)))
				os.Exit(1)
			}
			if c.verbose {
				c.ui.Warn(c.Colorize().Color(fmt.Sprintf(
					"[bold]%s[reset]",
					strings.TrimSpace(err.Error()),
				)))
			}
			mErr = multierror.Append(mErr, err)
		}
	}
}
