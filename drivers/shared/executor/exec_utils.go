package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/drivers"
	dproto "github.com/hashicorp/nomad/plugins/drivers/proto"
	"github.com/kr/pty"
	"golang.org/x/sys/unix"
)

func cmdExitResult(ps *os.ProcessState) *drivers.ExecTaskStreamingResponseMsg {
	exitCode := -1

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
	go func() {
		for {
			m, err := stream.Recv()
			logger.Info("received stdin", "msg", m, "error", err)
			if isClosedError(err) {
				return
			} else if err != nil {
				errCh <- err
				return
			}

			if m.Stdin != nil && len(m.Stdin.Data) != 0 {
				_, err := stdin.Write(m.Stdin.Data)
				if err != nil {
					errCh <- err
				}
			} else if m.Stdin != nil && m.Stdin.Close {
				stdin.Close()
			} else {
				// ignore heartbeats or unexpected tty events
			}
		}
	}()
}

func handleStdout(logger hclog.Logger, reader io.Reader, wg *sync.WaitGroup, send func(*drivers.ExecTaskStreamingResponseMsg) error, errCh chan<- error) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			buf := make([]byte, 4096)
			n, err := reader.Read(buf)
			logger.Info("sending stdout", "bytes", string(buf[:n]), "error", err, "isioe", isClosedError(err))
			if isClosedError(err) {
				if err := send(&drivers.ExecTaskStreamingResponseMsg{
					Stdout: &dproto.ExecTaskStreamingOperation{
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

			// tty only reportsstdout
			if err := send(&drivers.ExecTaskStreamingResponseMsg{
				Stdout: &dproto.ExecTaskStreamingOperation{
					Data: buf[:n],
				},
			}); err != nil {
				errCh <- err
				return
			}

		}
	}()
}

func handleStderr(logger hclog.Logger, reader io.Reader, wg *sync.WaitGroup, send func(*drivers.ExecTaskStreamingResponseMsg) error, errCh chan<- error) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			buf := make([]byte, 4096)
			n, err := reader.Read(buf)
			logger.Info("sending stdout", "bytes", string(buf[:n]), "error", err)
			if isClosedError(err) {
				if err := send(&drivers.ExecTaskStreamingResponseMsg{
					Stderr: &dproto.ExecTaskStreamingOperation{
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

			if err := send(&drivers.ExecTaskStreamingResponseMsg{
				Stderr: &dproto.ExecTaskStreamingOperation{
					Data: buf[:n],
				},
			}); err != nil {
				errCh <- err
				return
			}

		}
	}()
}

func isClosedError(err error) bool {
	if err == nil {
		return false
	}

	return err == io.EOF ||
		err == io.ErrClosedPipe ||
		strings.Contains(err.Error(), unix.EIO.Error())
}

func startExec(ctx context.Context, tty bool,
	logger hclog.Logger,
	stream drivers.ExecTaskStream,
	setTTY func(*os.File) error,
	setIO func(stdin io.Reader, stdout, stderr io.Writer) error,
	startFn func() error,
	waitFn func() (*os.ProcessState, error),
) error {
	if tty {
		return startExecTty(ctx, logger, stream, setTTY, startFn, waitFn)
	}
	return startExecNoTty(ctx, logger, stream, setIO, startFn, waitFn)
}

func startExecTty(ctx context.Context,
	logger hclog.Logger,
	stream drivers.ExecTaskStream,
	setTTY func(tty *os.File) error,
	startFn func() error,
	waitFn func() (*os.ProcessState, error),
) error {
	pty, tty, err := pty.Open()
	if err != nil {
		return fmt.Errorf("failed to open a tty: %v", err)
	}
	defer tty.Close()
	defer pty.Close()

	if err := setTTY(tty); err != nil {
		return fmt.Errorf("failed to set command tty: %v", err)
	}
	if err := startFn(); err != nil {
		return fmt.Errorf("failed to start command: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	handleStdin(logger, pty, stream, errCh)
	// when tty is on, stdout and stderr point to the same pty so only read once
	handleStdout(logger, pty, &wg, stream.Send, errCh)

	ps, err := waitFn()
	logger.Warn("command done", "error", err)
	if err != nil {
		logger.Warn("failed to wait for cmd", "error", err)
	}

	tty.Close()

	// wait until we get all process output
	wg.Wait()

	if ps != nil {
		// wait to flush out output
		stream.Send(cmdExitResult(ps))
	}

	select {
	case cerr := <-errCh:
		return cerr
	default:
		return err
	}
}

func startExecNoTty(ctx context.Context,
	logger hclog.Logger,
	stream drivers.ExecTaskStream,
	setIO func(stdin io.Reader, stdout, stderr io.Writer) error,
	startFn func() error,
	waitFn func() (*os.ProcessState, error),
) error {
	var sendLock sync.Mutex
	send := func(v *drivers.ExecTaskStreamingResponseMsg) error {
		sendLock.Lock()
		defer sendLock.Unlock()

		return stream.Send(v)
	}

	stdinPr, stdinPw := io.Pipe()
	stdoutPr, stdoutPw := io.Pipe()
	stderrPr, stderrPw := io.Pipe()

	if err := setIO(stdinPr, stdoutPw, stderrPw); err != nil {
		return fmt.Errorf("failed to set command io: %v", err)
	}

	if err := startFn(); err != nil {
		return fmt.Errorf("failed to start command: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	handleStdin(logger, stdinPw, stream, errCh)
	handleStdout(logger, stdoutPr, &wg, send, errCh)
	handleStderr(logger, stderrPr, &wg, send, errCh)

	// wait until we get all process output
	wg.Wait()

	ps, err := waitFn()
	logger.Warn("command done", "error", err)
	if err != nil {
		logger.Warn("failed to wait for cmd", "error", err)
	}

	if ps != nil {
		// wait to flush out output
		stream.Send(cmdExitResult(ps))
	}

	select {
	case cerr := <-errCh:
		return cerr
	default:
		return err
	}
}
