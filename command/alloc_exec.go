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
	"github.com/hashicorp/nomad/helper/escapingio"
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

  -job
    Use a random allocation from the specified job ID.

  -i
    Pass stdin to the container, defaults to true.  Pass -i=false to disable.

  -t
    Allocate a pseudo-tty, defaults to true if stdin is detected to be a tty session.
    Pass -t=false to disable explicitly.

  -e <escape_char>
    Sets the escape character for sessions with a pty (default: '~').  The escape
    character is only recognized at the beginning of a line.  The escape character
    followed by a dot ('.') closes the connection.  Setting the character to
    'none' disables any escapes and makes the session fully transparent.
  `
	return strings.TrimSpace(helpText)
}

func (l *AllocExecCommand) Synopsis() string {
	return "Execute commands in task"
}

func (c *AllocExecCommand) AutocompleteFlags() complete.Flags {
	return mergeAutocompleteFlags(c.Meta.AutocompleteFlags(FlagSetClient),
		complete.Flags{
			"--task": complete.PredictAnything,
			"-job":   complete.PredictAnything,
			"-i":     complete.PredictNothing,
			"-t":     complete.PredictNothing,
			"-e":     complete.PredictSet("none", "~"),
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
	var job, stdinOpt, ttyOpt bool
	var task string
	var escapeChar string

	flags := l.Meta.FlagSet(l.Name(), FlagSetClient)
	flags.Usage = func() { l.Ui.Output(l.Help()) }
	flags.BoolVar(&job, "job", false, "")
	flags.StringVar(&task, "task", "", "")
	flags.StringVar(&escapeChar, "e", "~", "")

	flags.BoolVar(&stdinOpt, "i", true, "")

	inferredTty := isTty()
	flags.BoolVar(&ttyOpt, "t", inferredTty, "")

	if err := flags.Parse(args); err != nil {
		return 1
	}
	args = flags.Args()

	if ttyOpt && !stdinOpt {
		l.Ui.Error("-i must be enabled if running with tty")
		return 1
	}

	if escapeChar == "none" {
		escapeChar = ""
	} else if len(escapeChar) > 1 {
		l.Ui.Error("-e requires 'none' or a single character")
		return 1
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
	q := &api.QueryOptions{Namespace: allocs[0].Namespace}
	alloc, _, err := client.Allocations().Info(allocs[0].ID, q)
	if err != nil {
		l.Ui.Error(fmt.Sprintf("Error querying allocation: %s", err))
		return 1
	}

	if task == "" {
		task, err = lookupAllocTask(alloc)

		if err != nil {
			l.Ui.Error(err.Error())
			return 1
		}
	}

	if err := validateTaskExistsInAllocation(task, alloc); err != nil {
		l.Ui.Error(err.Error())
		return 1
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

	code, err := l.execImpl(client, alloc, task, ttyOpt, command, escapeChar, stdin, l.Stdout, l.Stderr)
	if err != nil {
		l.Ui.Error(fmt.Sprintf("failed to exec into task: %v", err))
		return 1
	}

	return code
}

// execImpl invokes the Alloc Exec api call, it also prepares and restores terminal states as necessary.
func (l *AllocExecCommand) execImpl(client *api.Client, alloc *api.Allocation, task string, tty bool,
	command []string, escapeChar string, stdin io.Reader, stdout, stderr io.WriteCloser) (int, error) {

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

	return client.Allocations().Exec(ctx,
		alloc, task, tty, command, stdin, stdout, stderr, sizeCh, nil)
}

// isTty returns true if both stdin and stdout are a TTY
func isTty() bool {
	_, isStdinTerminal := term.GetFdInfo(os.Stdin)
	_, isStdoutTerminal := term.GetFdInfo(os.Stdout)
	return isStdinTerminal && isStdoutTerminal
}

// setRawTerminal sets the stream terminal in raw mode, so process captures
// Ctrl+C and other commands to forward to remote process.
// It returns a cleanup function that restores terminal to original mode.
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

// setRawTerminalOutput sets the output stream in Windows to raw mode,
// so it disables LF -> CRLF translation.
// It's basically a no-op on unix.
func setRawTerminalOutput(stream interface{}) (cleanup func(), err error) {
	fd, isTerminal := term.GetFdInfo(stream)
	if !isTerminal {
		return nil, errors.New("not a terminal")
	}

	state, err := term.SetRawTerminalOutput(fd)
	if err != nil {
		return nil, err
	}

	return func() { term.RestoreTerminal(fd, state) }, nil
}

// watchTerminalSize watches terminal size changes to propagate to remote tty.
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
			Height: int(s.Height),
			Width:  int(s.Width),
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
