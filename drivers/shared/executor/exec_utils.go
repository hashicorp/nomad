package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/drivers"
	dproto "github.com/hashicorp/nomad/plugins/drivers/proto"
)

// execHelper is a convenient wrapper for starting and executing commands, and handling their output
type execHelper struct {
	logger hclog.Logger

	// newTerminal function creates a tty appropriate for the command
	// The returned pty end of tty function is to be called after process start.
	newTerminal func() (pty func() (*os.File, error), tty *os.File, err error)

	// setTTY is a callback to configure the command with slave end of the tty of the terminal, when tty is enabled
	setTTY func(tty *os.File) error

	// setTTY is a callback to configure the command with std{in|out|err}, when tty is disabled
	setIO func(stdin io.Reader, stdout, stderr io.Writer) error

	// processStart starts the process, like `exec.Cmd.Start()`
	processStart func() error

	// processWait blocks until command terminates and returns its final state
	processWait func() (*os.ProcessState, error)
}

func (e *execHelper) run(ctx context.Context, tty bool, stream drivers.ExecTaskStream) error {
	if tty {
		return e.runTTY(ctx, stream)
	}
	return e.runNoTTY(ctx, stream)
}

func (e *execHelper) runTTY(ctx context.Context, stream drivers.ExecTaskStream) error {
	ptyF, tty, err := e.newTerminal()
	if err != nil {
		return fmt.Errorf("failed to open a tty: %v", err)
	}
	defer tty.Close()

	if err := e.setTTY(tty); err != nil {
		return fmt.Errorf("failed to set command tty: %v", err)
	}
	if err := e.processStart(); err != nil {
		return fmt.Errorf("failed to start command: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	pty, err := ptyF()
	if err != nil {
		return fmt.Errorf("failed to get pty: %v", err)
	}

	defer pty.Close()
	wg.Add(1)
	go handleStdin(e.logger, pty, stream, errCh)
	// when tty is on, stdout and stderr point to the same pty so only read once
	go handleStdout(e.logger, pty, &wg, stream.Send, errCh)

	ps, err := e.processWait()

	// force close streams to close out the stream copying goroutines
	tty.Close()

	// wait until we get all process output
	wg.Wait()

	// wait to flush out output
	stream.Send(cmdExitResult(ps, err))

	select {
	case cerr := <-errCh:
		return cerr
	default:
		return nil
	}
}

func (e *execHelper) runNoTTY(ctx context.Context, stream drivers.ExecTaskStream) error {
	var sendLock sync.Mutex
	send := func(v *drivers.ExecTaskStreamingResponseMsg) error {
		sendLock.Lock()
		defer sendLock.Unlock()

		return stream.Send(v)
	}

	stdinPr, stdinPw := io.Pipe()
	stdoutPr, stdoutPw := io.Pipe()
	stderrPr, stderrPw := io.Pipe()

	defer stdoutPw.Close()
	defer stderrPw.Close()

	if err := e.setIO(stdinPr, stdoutPw, stderrPw); err != nil {
		return fmt.Errorf("failed to set command io: %v", err)
	}

	if err := e.processStart(); err != nil {
		return fmt.Errorf("failed to start command: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	wg.Add(2)
	go handleStdin(e.logger, stdinPw, stream, errCh)
	go handleStdout(e.logger, stdoutPr, &wg, send, errCh)
	go handleStderr(e.logger, stderrPr, &wg, send, errCh)

	ps, err := e.processWait()

	// force close streams to close out the stream copying goroutines
	stdinPr.Close()
	stdoutPw.Close()
	stderrPw.Close()

	// wait until we get all process output
	wg.Wait()

	// wait to flush out output
	stream.Send(cmdExitResult(ps, err))

	select {
	case cerr := <-errCh:
		return cerr
	default:
		return nil
	}
}
func cmdExitResult(ps *os.ProcessState, err error) *drivers.ExecTaskStreamingResponseMsg {
	exitCode := -1

	if ps == nil {
		if ee, ok := err.(*exec.ExitError); ok {
			ps = ee.ProcessState
		}
	}

	if ps == nil {
		exitCode = -2
	} else if status, ok := ps.Sys().(syscall.WaitStatus); ok {
		exitCode = status.ExitStatus()
		if status.Signaled() {
			const exitSignalBase = 128
			signal := int(status.Signal())
			exitCode = exitSignalBase + signal
		}
	}

	return &drivers.ExecTaskStreamingResponseMsg{
		Exited: true,
		Result: &dproto.ExitResult{
			ExitCode: int32(exitCode),
		},
	}
}

func handleStdin(logger hclog.Logger, stdin io.WriteCloser, stream drivers.ExecTaskStream, errCh chan<- error) {
	for {
		m, err := stream.Recv()
		if isClosedError(err) {
			return
		} else if err != nil {
			errCh <- err
			return
		}

		if m.Stdin != nil {
			if len(m.Stdin.Data) != 0 {
				_, err := stdin.Write(m.Stdin.Data)
				if err != nil {
					errCh <- err
					return
				}
			}
			if m.Stdin.Close {
				stdin.Close()
			}
		} else if m.TtySize != nil {
			err := setTTYSize(stdin, m.TtySize.Height, m.TtySize.Width)
			if err != nil {
				errCh <- fmt.Errorf("failed to resize tty: %v", err)
				return
			}
		}
	}
}

func handleStdout(logger hclog.Logger, reader io.Reader, wg *sync.WaitGroup, send func(*drivers.ExecTaskStreamingResponseMsg) error, errCh chan<- error) {
	defer wg.Done()

	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		// always send output first if we read something
		if n > 0 {
			if err := send(&drivers.ExecTaskStreamingResponseMsg{
				Stdout: &dproto.ExecTaskStreamingIOOperation{
					Data: buf[:n],
				},
			}); err != nil {
				errCh <- err
				return
			}
		}

		// then process error
		if isClosedError(err) {
			if err := send(&drivers.ExecTaskStreamingResponseMsg{
				Stdout: &dproto.ExecTaskStreamingIOOperation{
					Close: true,
				},
			}); err != nil {
				errCh <- err
				return
			}
			return
		} else if err != nil {
			errCh <- err
			return
		}

	}
}

func handleStderr(logger hclog.Logger, reader io.Reader, wg *sync.WaitGroup, send func(*drivers.ExecTaskStreamingResponseMsg) error, errCh chan<- error) {
	defer wg.Done()

	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		// always send output first if we read something
		if n > 0 {
			if err := send(&drivers.ExecTaskStreamingResponseMsg{
				Stderr: &dproto.ExecTaskStreamingIOOperation{
					Data: buf[:n],
				},
			}); err != nil {
				errCh <- err
				return
			}
		}

		// then process error
		if isClosedError(err) {
			if err := send(&drivers.ExecTaskStreamingResponseMsg{
				Stderr: &dproto.ExecTaskStreamingIOOperation{
					Close: true,
				},
			}); err != nil {
				errCh <- err
				return
			}
			return
		} else if err != nil {
			errCh <- err
			return
		}

	}
}

func isClosedError(err error) bool {
	if err == nil {
		return false
	}

	return err == io.EOF ||
		err == io.ErrClosedPipe ||
		isUnixEIOErr(err)
}
