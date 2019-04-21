package executor

import (
	"io"
	"os"
	"strings"
	"sync"
	"syscall"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/nomad/plugins/drivers"
	dproto "github.com/hashicorp/nomad/plugins/drivers/proto"
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
