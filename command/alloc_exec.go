package command

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/docker/docker/pkg/term"
	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/api/contexts"
	"github.com/posener/complete"
)

type AllocExecCommand struct {
	Meta

	Stdin  io.Reader
	Stdout io.WriteCloser
	Stderr io.WriteCloser
}

func (l *AllocExecCommand) Help() string {
	helpText := `
Usage: nomad alloc exec [options] <allocation> <command>

  Run command inside the environment of the given allocation and task.

General Options:

  ` + generalOptionsUsage() + `

Exec Specific Options:

  -task <task-name>
    Sets the task to exec command in

  -job <job-id>
    Use a random allocation from the specified job ID.

  -i, --stdin=true
    Pass stdin to the container, defaults to true

  -t, --tty=true
    Allocate a pseudo-tty, defaults to true if stdin is detected to be a tty session
  `
	return strings.TrimSpace(helpText)
}

func (l *AllocExecCommand) Synopsis() string {
	return "Execute commands in task"
}

func (c *AllocExecCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"--task":  complete.PredictAnything,
			"-job":    complete.PredictAnything,
			"-t":      complete.PredictNothing,
			"--tty":   complete.PredictNothing,
			"--stdin": complete.PredictNothing,
		})
}

func (l *AllocExecCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictFunc(func(a complete.Args) []string {
		client, err := l.Meta.Client()
		if err != nil {
			return nil
		}

		resp, _, err := client.Search().PrefixSearch(a.Last, contexts.Allocs, nil)
		if err != nil {
			return []string{}
		}
		return resp.Matches[contexts.Allocs]
	})
}

func (l *AllocExecCommand) Name() string { return "alloc exec" }

func (l *AllocExecCommand) Run(args []string) int {
	var job bool
	var stdinShortOpt, stdinLongOpt, ttyShortOpt, ttyLongOpt bool
	var task string

	flags := l.Meta.FlagSet(l.Name(), FlagSetClient)
	flags.Usage = func() { l.Ui.Output(l.Help()) }
	flags.BoolVar(&job, "job", false, "")
	flags.StringVar(&task, "task", "", "")

	flags.BoolVar(&stdinShortOpt, "i", true, "")
	flags.BoolVar(&stdinLongOpt, "stdin", true, "")

	stdinTty := isStdinTty()
	flags.BoolVar(&ttyShortOpt, "t", stdinTty, "")
	flags.BoolVar(&ttyLongOpt, "tty", stdinTty, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	// try to infer stdin value if one option doesn't match defaults
	stdinOpt := stdinShortOpt && stdinLongOpt

	ttyOpt := stdinTty
	if ttyLongOpt != stdinTty || ttyShortOpt != stdinTty {
		ttyOpt = !stdinTty
	}

	if !stdinOpt {
		// if stdin is disabled, disable tty
		// TODO: detect if user passed incompatible settings and report
		ttyOpt = false
	}

	if numArgs := len(args); numArgs < 1 {
		if job {
			l.Ui.Error("A job ID is required")
		} else {
			l.Ui.Error("An allocation ID is required")
		}

		l.Ui.Error(commandErrorText(l))
		return 1
	} else if numArgs < 2 {
		l.Ui.Error("A command is required")
		l.Ui.Error(commandErrorText(l))
		return 1
	}

	command := args[1:]

	client, err := l.Meta.Client()
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Error initializing client: %v", err))
		return 1
	}

	// If -job is specified, use random allocation, otherwise use provided allocation
	allocID := args[0]
	if job {
		allocID, err = getRandomJobAlloc(client, args[0])
		if err != nil {
			l.Ui.Error(fmt.Sprintf("Error fetching allocations: %v", err))
			return 1
		}
	}

	length := shortId

	// Query the allocation info
	if len(allocID) == 1 {
		l.Ui.Error(fmt.Sprintf("Alloc ID must contain at least two characters."))
		return 1
	}

	allocID = sanitizeUUIDPrefix(allocID)
	allocs, _, err := client.Allocations().PrefixList(allocID)
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Error querying allocation: %v", err))
		return 1
	}
	if len(allocs) == 0 {
		l.Ui.Error(fmt.Sprintf("No allocation(s) with prefix or id %q found", allocID))
		return 1
	}
	if len(allocs) > 1 {
		// Format the allocs
		out := formatAllocListStubs(allocs, false, length)
		l.Ui.Error(fmt.Sprintf("Prefix matched multiple allocations\n\n%s", out))
		return 1
	}
	// Prefix lookup matched a single allocation
	alloc, _, err := client.Allocations().Info(allocs[0].ID, nil)
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
		return 1
	}

	if task == "" {
		// Try to determine the tasks name from the allocation
		var tasks []*api.Task
		for _, tg := range alloc.Job.TaskGroups {
			if *tg.Name == alloc.TaskGroup {
				if len(tg.Tasks) == 1 {
					task = tg.Tasks[0].Name
					break
				}

				tasks = tg.Tasks
				break
			}
		}

		if task == "" {
			l.Ui.Error(fmt.Sprintf("Allocation %q is running the following tasks:", limit(alloc.ID, length)))
			for _, t := range tasks {
				l.Ui.Error(fmt.Sprintf("  * %s", t.Name))
			}
			l.Ui.Error("\nPlease specify the task.")
			return 1
		}
	}

	if l.Stdin == nil {
		l.Stdin = os.Stdin
	}
	if l.Stdout == nil {
		l.Stdout = os.Stdout
	}
	if l.Stderr == nil {
		l.Stderr = os.Stderr
	}

	var stdin io.Reader = l.Stdin
	if !stdinOpt {
		stdin = bytes.NewReader(nil)
	}

	code, err := l.execImpl(client, alloc, task, ttyOpt, command, stdin, l.Stdout, l.Stderr)
	if err != nil {
		l.Ui.Error(fmt.Sprintf("failed to exec into task: %v", err))
		return 1
	}

	return code
}

func isStdinTty() bool {
	_, isTerminal := term.GetFdInfo(os.Stdin)
	return isTerminal
}

func setRawTerminal(stream interface{}) (cleanup func(), err error) {
	fd, isTerminal := term.GetFdInfo(stream)
	if !isTerminal {
		return nil, errors.New("not a terminal")
	}

	state, err := term.SetRawTerminal(fd)
	if err != nil {
		return nil, err
	}

	return func() { term.RestoreTerminal(fd, state) }, nil
}

func watchTerminalSize(out io.Writer, resize chan<- api.TerminalSize) (func(), error) {
	fd, isTerminal := term.GetFdInfo(out)
	if !isTerminal {
		return nil, errors.New("not a terminal")
	}

	ctx, cancel := context.WithCancel(context.Background())

	signalCh := make(chan os.Signal, 1)
	setupWindowNotification(signalCh)

	sendTerminalSize := func() {
		s, err := term.GetWinsize(fd)
		if err != nil {
			return
		}

		resize <- api.TerminalSize{
			Height: int32(s.Height),
			Width:  int32(s.Width),
		}
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-signalCh:
				sendTerminalSize()
			}
		}
	}()

	go func() {
		// send initial size
		sendTerminalSize()
	}()

	return cancel, nil
}

func (l *AllocExecCommand) execImpl(client *api.Client, alloc *api.Allocation, task string, tty bool,
	command []string, stdin io.Reader, stdout, stderr io.WriteCloser) (int, error) {

	sizeCh := make(chan api.TerminalSize, 1)

	if tty {
		if stdin == nil {
			return -1, fmt.Errorf("stdin is null")
		}

		inCleanup, err := setRawTerminal(stdin)
		if err != nil {
			return -1, err
		}
		defer inCleanup()

		outCleanup, err := setRawTerminal(stdout)
		if err != nil {
			return -1, err
		}
		defer outCleanup()

		sizeCleanup, err := watchTerminalSize(stdout, sizeCh)
		if err != nil {
			return -1, err
		}
		defer sizeCleanup()
	}

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range signalCh {
			cancelFn()
		}
	}()

	return client.Allocations().Exec(ctx,
		alloc, task, tty, command, stdin, stdout, stderr, sizeCh, nil)
}
