package executor

import (
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"

	"github.com/hashicorp/nomad/plugins/drivers"
	dproto "github.com/hashicorp/nomad/plugins/drivers/proto"
	"github.com/kr/pty"
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

func handleTTY(ptyf *os.File, wg sync.WaitGroup,
	input <-chan *drivers.ExecTaskStreamingRequestMsg,
	output chan<- *drivers.ExecTaskStreamingResponseMsg,
	errCh chan<- error) {

	wg.Add(2)

	// input handler
	go func() {
		defer wg.Done()

		for m := range input {
			if m.Stdin != nil && len(m.Stdin.Data) != 0 {
				ptyf.Write(m.Stdin.Data)
			} else if m.Stdin != nil && m.Stdin.Close {
				// it's odd to close a tty of a running session, so ignore it
			} else if m.TtySize != nil {
				pty.Setsize(ptyf, &pty.Winsize{
					Rows: uint16(m.TtySize.Height),
					Cols: uint16(m.TtySize.Width),
				})
			}
		}
	}()

	// handle output
	go func() {
		defer wg.Done()

		for {
			buf := make([]byte, 4096)
			n, err := ptyf.Read(buf)
			os.Stderr.WriteString(fmt.Sprintf("READ %v %s\n", err, string(buf[:n])))
			if err == io.EOF {
				output <- &drivers.ExecTaskStreamingResponseMsg{
					Stdout: &dproto.ExecTaskStreamingOperation{
						Close: true,
					},
				}
				return
			} else if err != nil {
				errCh <- err
				return
			}

			// tty only reports stdout
			output <- &drivers.ExecTaskStreamingResponseMsg{
				Stdout: &dproto.ExecTaskStreamingOperation{
					Data: buf[:n],
				},
			}

		}
	}()
}

func handleStdin(stdin io.WriteCloser, wg sync.WaitGroup, input <-chan *drivers.ExecTaskStreamingRequestMsg, errCh chan<- error) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		for m := range input {
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

func handleStdout(reader io.Reader, wg sync.WaitGroup, output chan<- *drivers.ExecTaskStreamingResponseMsg, errCh chan<- error) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			buf := make([]byte, 4096)
			n, err := reader.Read(buf)
			if err == io.EOF {
				output <- &drivers.ExecTaskStreamingResponseMsg{
					Stdout: &dproto.ExecTaskStreamingOperation{
						Close: true,
					},
				}
			} else if err != nil {
				errCh <- err
			}

			// tty only reports stdout
			output <- &drivers.ExecTaskStreamingResponseMsg{
				Stdout: &dproto.ExecTaskStreamingOperation{
					Data: buf[:n],
				},
			}

		}
	}()
}

func handleStderr(reader io.Reader, wg sync.WaitGroup, output chan<- *drivers.ExecTaskStreamingResponseMsg, errCh chan<- error) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			buf := make([]byte, 4096)
			n, err := reader.Read(buf)
			if err == io.EOF {
				output <- &drivers.ExecTaskStreamingResponseMsg{
					Stderr: &dproto.ExecTaskStreamingOperation{
						Close: true,
					},
				}
			} else if err != nil {
				errCh <- err
			}

			// tty only reports stdout
			output <- &drivers.ExecTaskStreamingResponseMsg{
				Stderr: &dproto.ExecTaskStreamingOperation{
					Data: buf[:n],
				},
			}

		}
	}()
}
