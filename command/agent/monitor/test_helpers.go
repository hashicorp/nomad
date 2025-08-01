// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package monitor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-msgpack/v2/codec"
	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/shoenig/test/must"
)

// StreamingClient is an interface that implements the StreamingRpcHandler function
type StreamingClient interface {
	StreamingRpcHandler(string) (structs.StreamingRpcHandler, error)
}

var writeLine = []byte(fmt.Sprintf("[INFO] log log log made of wood you are heavy but so good, %v\n", time.Now()))

func PrepFile(t *testing.T) *os.File {
	const loopCount = 100
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

// ExportMonitorClient_TestHelper consolidates streaming test setup for use in
// client and server RPChandler tests
func ExportMonitorClient_TestHelper(req cstructs.MonitorExportRequest, c StreamingClient,
	userTimeout <-chan time.Time) (*strings.Builder, error) {
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
