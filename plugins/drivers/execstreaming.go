package drivers

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/nomad/plugins/drivers/proto"
)

// StreamToExecOptions is a convenience method to convert exec stream into
// ExecOptions object.
func StreamToExecOptions(
	ctx context.Context,
	command []string,
	tty bool,
	stream ExecTaskStream) (*ExecOptions, <-chan error) {

	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()
	resize := make(chan TerminalSize, 2)

	errCh := make(chan error, 3)

	// handle input
	go func() {
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				return
			} else if err != nil {
				errCh <- err
				return
			}

			if msg.Stdin != nil && !msg.Stdin.Close {
				_, err := inWriter.Write(msg.Stdin.Data)
				if err != nil {
					errCh <- err
					return
				}
			} else if msg.Stdin != nil && msg.Stdin.Close {
				inWriter.Close()
			} else if msg.TtySize != nil {
				select {
				case resize <- TerminalSize{
					Height: int(msg.TtySize.Height),
					Width:  int(msg.TtySize.Width),
				}:
				case <-ctx.Done():
					// process terminated before resize is processed
					return
				}
			} else if isHeartbeat(msg) {
				// do nothing
			} else {
				errCh <- fmt.Errorf("unexpected message type: %#v", msg)
			}
		}
	}()

	var sendLock sync.Mutex
	send := func(v *ExecTaskStreamingResponseMsg) error {
		sendLock.Lock()
		defer sendLock.Unlock()

		return stream.Send(v)
	}

	var outWg sync.WaitGroup
	outWg.Add(2)
	// handle Stdout
	go func() {
		defer outWg.Done()

		reader := outReader
		bytes := make([]byte, 1024)
		msg := &ExecTaskStreamingResponseMsg{Stdout: &proto.ExecTaskStreamingIOOperation{}}

		for {
			n, err := reader.Read(bytes)
			// always send data if we read some
			if n != 0 {
				msg.Stdout.Data = bytes[:n]
				if err := send(msg); err != nil {
					errCh <- err
					break
				}
			}

			// then handle error
			if err == io.EOF || err == io.ErrClosedPipe {
				msg.Stdout.Data = nil
				msg.Stdout.Close = true

				if err := send(msg); err != nil {
					errCh <- err
				}
				break
			}

			if err != nil {
				errCh <- err
				break
			}
		}

	}()
	// handle Stderr
	go func() {
		defer outWg.Done()

		reader := errReader
		bytes := make([]byte, 1024)
		msg := &ExecTaskStreamingResponseMsg{Stderr: &proto.ExecTaskStreamingIOOperation{}}

		for {
			n, err := reader.Read(bytes)
			// always send data if we read some
			if n != 0 {
				msg.Stderr.Data = bytes[:n]
				if err := send(msg); err != nil {
					errCh <- err
					break
				}
			}

			// then handle error
			if err == io.EOF || err == io.ErrClosedPipe {
				msg.Stderr.Data = nil
				msg.Stderr.Close = true

				if err := send(msg); err != nil {
					errCh <- err
				}
				break
			}

			if err != nil {
				errCh <- err
				break
			}
		}

	}()

	doneCh := make(chan error, 1)
	go func() {
		outWg.Wait()

		select {
		case err := <-errCh:
			doneCh <- err
		default:
		}
		close(doneCh)
	}()

	return &ExecOptions{
		Command: command,
		Tty:     tty,

		Stdin:  inReader,
		Stdout: outWriter,
		Stderr: errWriter,

		ResizeCh: resize,
	}, doneCh
}

func NewExecStreamingResponseExit(exitCode int) *ExecTaskStreamingResponseMsg {
	return &ExecTaskStreamingResponseMsg{
		Exited: true,
		Result: &proto.ExitResult{
			ExitCode: int32(exitCode),
		},
	}

}

func isHeartbeat(r *ExecTaskStreamingRequestMsg) bool {
	return r.Stdin == nil && r.Setup == nil && r.TtySize == nil
}
