// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package command

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/helper/escapingio"
	"github.com/posener/complete"
)

type ActionCommand struct {
	Meta

	Stdin  io.Reader
	Stdout io.WriteCloser
	Stderr io.WriteCloser
}

func (c *ActionCommand) Help() string {
	helpText := `
Usage: nomad action [options] <action>

  Perform a predefined command inside the environment of the given allocation
  and task.

  When ACLs are enabled, this command requires a token with the 'alloc-exec',
  'read-job', and 'list-jobs' capabilities for a task's namespace. If
  the task driver does not have file system isolation (as with 'raw_exec'),
  this command requires the 'alloc-node-exec', 'read-job', and 'list-jobs'
  capabilities for the task's namespace.

General Options:

  ` + generalOptionsUsage(usageOptsNoNamespace) + `

Action Specific Options:

  -job <job-id>
    Specifies the job in which the Action is defined

  -alloc <allocation-id>
    Specifies the allocation in which the Action is defined. If not provided,
    a group and task name must be provided and a random allocation will be
    selected from the job.

  -task <task-name>
    Specifies the task in which the Action is defined. Required if no
    allocation is provided.

  -group=<group-name>
    Specifies the group in which the Action is defined. Required if no
    allocation is provided.

  -i
    Pass stdin to the container, defaults to true. Pass -i=false to disable.

  -t
    Allocate a pseudo-tty, defaults to true if stdin is detected to be a tty session.
    Pass -t=false to disable explicitly.

  -e <escape_char>
    Sets the escape character for sessions with a pty (default: '~'). The escape
    character is only recognized at the beginning of a line. The escape character
    followed by a dot ('.') closes the connection. Setting the character to
    'none' disables any escapes and makes the session fully transparent.
`
	return strings.TrimSpace(helpText)
}

func (c *ActionCommand) Synopsis() string {
	return "Run a pre-defined action from a Nomad task"
}

func (c *ActionCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"-job":   complete.PredictAnything,
			"-alloc": complete.PredictAnything,
			"-task":  complete.PredictAnything,
			"-group": complete.PredictAnything,
			"-i":     complete.PredictNothing,
			"-t":     complete.PredictNothing,
			"-e":     complete.PredictAnything,
		})
}

func (c *ActionCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *ActionCommand) Name() string { return "action" }

func (c *ActionCommand) Run(args []string) int {

	var stdinOpt, ttyOpt bool
	var task, allocation, job, group, escapeChar string

	flags := c.Meta.FlagSet(c.Name(), FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.StringVar(&task, "task", "", "")
	flags.StringVar(&group, "group", "", "")
	flags.StringVar(&allocation, "alloc", "", "")
	flags.StringVar(&job, "job", "", "")
	flags.BoolVar(&stdinOpt, "i", true, "")
	flags.BoolVar(&ttyOpt, "t", isTty(), "")
	flags.StringVar(&escapeChar, "e", "~", "")

	if err := flags.Parse(args); err != nil {
		c.Ui.Error(fmt.Sprintf("Error parsing flags: %s", err))
		return 1
	}

	args = flags.Args()

	if len(args) < 1 {
		c.Ui.Error("An action name is required")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if job == "" {
		c.Ui.Error("A job ID is required")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if ttyOpt && !stdinOpt {
		c.Ui.Error("-i must be enabled if running with tty")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	if escapeChar == "none" {
		escapeChar = ""
	}

	if len(escapeChar) > 1 {
		c.Ui.Error("-e requires 'none' or a single character")
		c.Ui.Error(commandErrorText(c))
		return 1
	}

	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	var allocStub *api.AllocationListStub
	// If no allocation provided, grab a random one from the job
	if allocation == "" {

		// Group param cannot be empty if allocation is empty,
		// since we'll need to get a random allocation from the group
		if group == "" {
			c.Ui.Error("A group name is required if no allocation is provided")
			c.Ui.Error(commandErrorText(c))
			return 1
		}

		if task == "" {
			c.Ui.Error("A task name is required if no allocation is provided")
			c.Ui.Error(commandErrorText(c))
			return 1
		}

		jobID, ns, err := c.JobIDByPrefix(client, job, nil)
		if err != nil {
			c.Ui.Error(err.Error())
			return 1
		}

		allocStub, err = getRandomJobAlloc(client, jobID, group, ns)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error fetching allocations: %v", err))
			return 1
		}
	} else {
		allocs, _, err := client.Allocations().PrefixList(sanitizeUUIDPrefix(allocation))
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error querying allocation: %v", err))
			return 1
		}

		if len(allocs) == 0 {
			c.Ui.Error(fmt.Sprintf("No allocation(s) with prefix or id %q found", allocation))
			return 1
		}

		if len(allocs) > 1 {
			out := formatAllocListStubs(allocs, false, shortId)
			c.Ui.Error(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", out))
			return 1
		}

		allocStub = allocs[0]
	}

	q := &api.QueryOptions{Namespace: allocStub.Namespace}
	alloc, _, err := client.Allocations().Info(allocStub.ID, q)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
		return 1
	}

	if task != "" {
		err = validateTaskExistsInAllocation(task, alloc)
	} else {
		task, err = lookupAllocTask(alloc)
	}
	if err != nil {
		c.Ui.Error(err.Error())
		return 1
	}

	if !stdinOpt {
		c.Stdin = bytes.NewReader(nil)
	}

	if c.Stdin == nil {
		c.Stdin = os.Stdin
	}

	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}

	if c.Stderr == nil {
		c.Stderr = os.Stderr
	}

	action := args[0]

	code, err := c.execImpl(client, alloc, task, job, action, ttyOpt, escapeChar, c.Stdin, c.Stdout, c.Stderr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("failed to exec into task: %v", err))
		return 1
	}

	return code
}

// execImpl invokes the Alloc Exec api call, it also prepares and restores terminal states as necessary.
func (c *ActionCommand) execImpl(client *api.Client, alloc *api.Allocation, task string, job string, action string, tty bool,
	escapeChar string, stdin io.Reader, stdout, stderr io.WriteCloser) (int, error) {

	sizeCh := make(chan api.TerminalSize, 1)

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	// When tty, ensures we capture all user input and monitor terminal resizes.
	if tty {
		if stdin == nil {
			return -1, fmt.Errorf("stdin is null")
		}

		inCleanup, err := setRawTerminal(stdin)
		if err != nil {
			return -1, err
		}
		defer inCleanup()

		outCleanup, err := setRawTerminalOutput(stdout)
		if err != nil {
			return -1, err
		}
		defer outCleanup()

		sizeCleanup, err := watchTerminalSize(stdout, sizeCh)
		if err != nil {
			return -1, err
		}
		defer sizeCleanup()

		if escapeChar != "" {
			stdin = escapingio.NewReader(stdin, escapeChar[0], func(c byte) bool {
				switch c {
				case '.':
					// need to restore tty state so error reporting here
					// gets emitted at beginning of line
					outCleanup()
					inCleanup()

					stderr.Write([]byte("\nConnection closed\n"))
					cancelFn()
					return true
				default:
					return false
				}
			})
		}
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range signalCh {
			cancelFn()
		}
	}()

	return client.Jobs().ActionExec(ctx, alloc, job, task,
		tty, make([]string, 0), action,
		stdin, stdout, stderr, sizeCh, nil)
}
