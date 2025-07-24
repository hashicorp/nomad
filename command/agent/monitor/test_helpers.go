// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package monitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/hashicorp/go-msgpack/v2/codec"
	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

func ExportMonitorClient_TestHelper(req cstructs.MonitorExportRequest, c StreamingClient, userTimeout <-chan time.Time) (*strings.Builder, error) {
	var (
		builder     strings.Builder
		returnedErr error
		timeout     <-chan time.Time
	)
	handler, err := c.StreamingRpcHandler("Agent.MonitorExport")
	if err != nil {
		return nil, err
	}

	// create pipe
	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()

	errCh := make(chan error)
	streamMsg := make(chan *cstructs.StreamErrWrapper)

	go handler(p2)

	// Start decoder
	go func() {
		decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
		for {
			var msg cstructs.StreamErrWrapper
			err := decoder.Decode(&msg)
			streamMsg <- &msg
			if err != nil {
				errCh <- err
				return
			}

		}
	}()

	// send request
	encoder := codec.NewEncoder(p1, structs.MsgpackHandle)
	if err := encoder.Encode(req); err != nil {
		return nil, err
	}
	if userTimeout != nil {
		timeout = userTimeout
	}

OUTER:
	for {
		select {
		case <-timeout:
			return nil, errors.New("expected to be unreachable")
		case err := <-errCh:
			if err != nil && err != io.EOF {
				return nil, err
			}
		case message := <-streamMsg:
			var frame sframer.StreamFrame

			if message.Error != nil {
				returnedErr = message.Error
			}

			if len(message.Payload) != 0 {
				err = json.Unmarshal(message.Payload, &frame)
				returnedErr = err
				builder.Write(frame.Data)
			} else {
				break OUTER
			}
		}
	}
	return &builder, returnedErr
}

// NewMockExportMonitor returns an ExportMonitor configured to read
// from the user namespace instead
func NewMockExportMonitor(opts MonitorExportOpts) (*ExportMonitor, error) {
	var bufSize int

	cmd, multiReader, prepErr := mockCliReader(opts)
	if prepErr != nil {
		return nil, prepErr
	}
	ExportReader := ExportReader{multiReader, cmd, true, opts.Follow}

	if opts.bufSize == 0 {
		bufSize = defaultBufSize
	} else {
		bufSize = opts.bufSize
	}
	sw := ExportMonitor{
		logger:       opts.Logger,
		doneCh:       make(chan struct{}, 1),
		logCh:        make(chan []byte, bufSize),
		bufSize:      bufSize,
		ExportReader: ExportReader,
	}
	return &sw, nil
}

func mockCliReader(opts MonitorExportOpts) (*exec.Cmd, io.Reader, error) {
	cmdDuration := "72 hours"
	if opts.LogsSince != "" {
		parsedDur, err := time.ParseDuration(opts.LogsSince)
		if err != nil {
			return nil, nil, err
		}
		cmdDuration = parsedDur.String()
	}
	// Mock
	cmdArgs := []string{"--user --no-pager", "--since", fmt.Sprintf("%s ago", cmdDuration)}

	if opts.Follow {
		cmdArgs = append(cmdArgs, "-f")
	}
	cmd := exec.CommandContext(opts.Context, "journalctl", cmdArgs...)

	// set up reader
	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stdErr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}
	multiReader := io.MultiReader(stdOut, stdErr)

	return cmd, multiReader, nil
}
