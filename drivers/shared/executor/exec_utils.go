package executor

import (
	"io"
	"os"
	"sync"
	"syscall"

	"github.com/hashicorp/nomad/plugins/drivers"
	dproto "github.com/hashicorp/nomad/plugins/drivers/proto"
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

func handleStdin(stdin io.WriteCloser, wg sync.WaitGroup, stream drivers.ExecTaskStream, errCh chan<- error) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			m, err := stream.Recv()
			if err == io.EOF {
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
				// it's odd to close a tty of a running session, so ignore it
			} else {
				// ignore heartbeats or unexpected tty events
			}
		}
	}()
}

func handleStdout(reader io.Reader, wg sync.WaitGroup, send func(*drivers.ExecTaskStreamingResponseMsg) error, errCh chan<- error) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			buf := make([]byte, 4096)
			n, err := reader.Read(buf)
			if err == io.EOF {
				if err := send(&drivers.ExecTaskStreamingResponseMsg{
					Stdout: &dproto.ExecTaskStreamingOperation{
						Close: true,
					},
				}); err != nil {
					errCh <- err
					return
				}
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

func handleStderr(reader io.Reader, wg sync.WaitGroup, send func(*drivers.ExecTaskStreamingResponseMsg) error, errCh chan<- error) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			buf := make([]byte, 4096)
			n, err := reader.Read(buf)
			if err == io.EOF {
				if err := send(&drivers.ExecTaskStreamingResponseMsg{
					Stderr: &dproto.ExecTaskStreamingOperation{
						Close: true,
					},
				}); err != nil {
					errCh <- err
					return
				}
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
