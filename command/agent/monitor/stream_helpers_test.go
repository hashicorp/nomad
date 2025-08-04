// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package monitor

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/ci"
	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	"github.com/shoenig/test/must"
)

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
			file := PrepFile(t)
			goldenFileContents, err := os.ReadFile(file.Name())
			must.NoError(t, err)

			var wg sync.WaitGroup
			wg.Add(1)
			streamMsg := make(chan []byte, len(goldenFileContents))
			streamBytes(streamMsg, &wg, file)
			wg.Wait()

			frames := make(chan *sframer.StreamFrame, 32)
			frameSize := 1024
			errCh := make(chan error, 1)
			framer := sframer.NewStreamFramer(frames, 1*time.Second, 200*time.Millisecond, frameSize)
			streamReader := NewStreamReader(streamMsg, framer, int64(frameSize))
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
					must.EqError(t, err, tc.errString)
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

func TestScanServiceName(t *testing.T) {
	cases := []struct {
		testString string
		expectErr  bool
	}{
		{
			testString: `nomad`,
		},
		{
			testString: `nomad.socket`,
		},
		{
			testString: `nomad-client.service`,
		},
		{
			testString: `nomad.client.02.swap`,
		},
		{
			testString: `nomadhelper@54.device`,
		},
		{
			testString: `1.\@_-nomad@`,
			expectErr:  true,
		},
		{
			testString: `1./@_-nomad@.automount`,
			expectErr:  true,
		},
		{
			testString: `docker.path`,
			expectErr:  true,
		},
		{
			testString: `nomad.path.gotcha`,
			expectErr:  true,
		},
		{
			testString: `nomad/8.path`,
			expectErr:  true,
		},
		{
			testString: `nomad%.path`,
			expectErr:  true,
		},
		{
			testString: `nom4ad.path`,
			expectErr:  true,
		},
		{
			testString: `nomad,.path`,
			expectErr:  true,
		},
		{
			testString: `nomad.client`,
			expectErr:  true,
		},
		{
			testString: `nomad!.path`,
			expectErr:  true,
		},
		{
			testString: `nomad%http.timer`,
			expectErr:  true,
		},
		{
			testString: `nomad,http.mount`,
			expectErr:  true,
		},
		{
			testString: `nomad$http.service`,
			expectErr:  true,
		},
		{
			testString: `nomad$.http.service`,
			expectErr:  true,
		},
		{
			testString: `nomad$`,
			expectErr:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.testString, func(t *testing.T) {
			err := ScanServiceName(tc.testString)
			if !tc.expectErr {
				must.NoError(t, err)
			} else {
				must.Error(t, err)
			}

		})
	}
}
