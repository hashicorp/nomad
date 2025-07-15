// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/v2/codec"
	"github.com/hashicorp/nomad/ci"
	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

var writeLine = []byte("[INFO] log log log made of wood you are heavy but so good\n")

func prepFile(t *testing.T) *os.File {
	const loopCount = 10
	// Create test file to read from
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "log")
	must.NoError(t, err)

	for range loopCount {
		_, _ = f.Write(writeLine)
	}
	f.Close()

	// Create test file reader for stream set up
	goldenFilePath := f.Name()
	fileReader, err := os.Open(goldenFilePath)
	must.NoError(t, err)
	return fileReader
}
func streamFunc(t *testing.T, fileReader *os.File) (chan *sframer.StreamFrame, chan error) {
	//build stream from test file contents
	framesCh := make(chan *sframer.StreamFrame, 1)
	errCh := make(chan error, 1)
	offset := 0
	r := io.LimitReader(fileReader, 64)
	for {
		bytesHolder := make([]byte, 64)
		n, err := r.Read(bytesHolder)
		if err != nil && err != io.EOF {
			must.NoError(t, err)
		}

		if n == 0 && err == io.EOF {
			break
		}

		framesCh <- &sframer.StreamFrame{
			Offset: int64(offset),
			Data:   bytesHolder[offset:n],
			File:   fileReader.Name(),
		}
		offset += n
		if n != 0 && err == io.EOF {
			//break after sending if we hit EOF with bytes in buffer
			break
		}
	}

	close(framesCh)
	return framesCh, errCh
}
func TestClientStreamEncoder_EncodeStream(t *testing.T) {
	ci.Parallel(t)
	file := prepFile(t)

	testErr := errors.New("isErr")

	cases := []struct {
		name         string
		expected     string
		nomadLogPath string
		serviceName  string
		token        string
		onDisk       bool
		expectErr    bool
		err          error
	}{
		{
			name: "happy_path",
		},
		{
			name:      "error",
			onDisk:    true,
			expectErr: true,
			err:       testErr,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			//populate framesCh
			framesCh, errCh := streamFunc(t, file)

			// create pipes
			p1, p2 := net.Pipe()
			defer p1.Close()
			defer p2.Close()

			// prepare streamEncoder
			var buf bytes.Buffer
			frameCodec := codec.NewEncoder(&buf, structs.JsonHandle)
			encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
			streamEncoder := NewStreamEncoder(&buf, p1, encoder, frameCodec, false)

			// Start reading on decoder pipe
			streamMsg := make(chan *cstructs.StreamErrWrapper, 1)
			go func() {
				decoder := codec.NewDecoder(p2, structs.MsgpackHandle)
				for {
					var msg cstructs.StreamErrWrapper
					err := decoder.Decode(&msg)

					streamMsg <- &msg
					if err != nil {
						errCh <- err
					}
				}
			}()

			ctx, cancel := context.WithCancel(context.Background())
			// mimic error or EOF/signal to close encoder encoder goroutine
			go func() {
				if tc.expectErr {
					time.Sleep(time.Millisecond * 1)
					errCh <- tc.err
				} else {
					time.Sleep(time.Second * 3)
					cancel()
				}
			}()
			// Encode stream
			go func() {
				quit := time.After(5 * time.Second)
				streamErr := streamEncoder.EncodeStream(framesCh, errCh, ctx)
				if !tc.expectErr {
					must.NoError(t, streamErr)
				}
				// ensure gofunc exits before test ends
				if now := <-quit; now.After(time.Now()) {
					return
				}

			}()
			timeout := time.After(3 * time.Second)

			//verify stream contents are encoded as expected
		OUTER:
			for {
				select {
				case <-timeout:
					must.Unreachable(t)
				case err := <-errCh:
					if err != nil && err != io.EOF {
						if tc.expectErr {
							must.Eq(t, tc.err.Error(), err.Error())
						}
						must.NoError(t, err)
					}
				case message := <-streamMsg:
					var frame sframer.StreamFrame

					err := json.Unmarshal(message.Payload, &frame)
					if err != nil && err != io.EOF {
						if !strings.Contains(err.Error(), "unexpected end") {
							must.NoError(t, err)
						}
					}
					must.SliceContainsSubset(t, frame.Data, writeLine)
					break OUTER
				}
			}
		})

	}
}

func TestClientStreamReader_StreamFixed(t *testing.T) {
	ci.Parallel(t)

	streamBytes := func(streamCh chan []byte, wg *sync.WaitGroup, file *os.File) {
		go func() {
			defer close(streamCh)
			defer wg.Done()
			logChunk := make([]byte, len(writeLine))
			for {
				n, readErr := file.Read(logChunk)
				if readErr != nil && readErr != io.EOF {
					must.NoError(t, readErr)
				}

				streamCh <- logChunk[:n]
				if readErr == io.EOF {
					break
				}
			}
		}()
	}

	cases := []struct {
		name string

		eofCancel bool
		expectErr bool
		errString string
	}{
		{
			name:      "happy_path",
			eofCancel: true,
		},
		{
			name:      "Stream Framer not Running",
			expectErr: true,
			eofCancel: true,
			errString: "StreamFramer not running",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := prepFile(t)
			goldenFileContents, err := os.ReadFile(file.Name())
			must.NoError(t, err)

			var wg sync.WaitGroup
			wg.Add(1)
			streamMsg := make(chan []byte, len(goldenFileContents))
			streamBytes(streamMsg, &wg, file)
			wg.Wait()

			frames := make(chan *sframer.StreamFrame, 32)
			errCh := make(chan error, 1)
			framer := sframer.NewStreamFramer(frames, 1*time.Second, 200*time.Millisecond, 1024)
			streamReader := NewStreamReader(streamMsg, framer)
			ctx, cancel := context.WithCancel(context.Background())

			defer cancel()
			wg.Add(1) //block until streamReader completes

			go func() {
				defer wg.Done()
				defer streamReader.Destroy()
				if !tc.expectErr {
					streamReader.Run()
				}
				initialOffset := int64(0)
				err := streamReader.StreamFixed(ctx, initialOffset, "", 0, errCh, tc.eofCancel)
				if !tc.expectErr {
					must.NoError(t, err)
				} else {
					must.NotNil(t, err)
					must.Eq(t, tc.errString, err.Error())
				}

			}()
			wg.Wait()
			// Parse and validate the contents of the frames channel
			var streamErr error
			var builder strings.Builder
			var skipCount int

		OUTER:
			for skipCount < 2 {
				select {
				case frame, ok := <-frames:
					if !ok {
						select {
						case streamErr = <-errCh:
							must.NoError(t, streamErr) //we shouldn't hit an error here
						default:

						}
						break OUTER
					}
					builder.Write(frame.Data)
				case streamErr = <-errCh:
					must.NoError(t, streamErr) //we shouldn't hit an error here
				case <-ctx.Done():
					break OUTER
				default:
					skipCount++
					time.Sleep(1 * time.Millisecond) //makes the test a touch less flakey
				}
			}
			if !tc.expectErr {
				must.Eq(t, string(goldenFileContents), builder.String())
			}

		})

	}
}
