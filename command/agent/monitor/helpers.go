// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/hashicorp/go-msgpack/v2/codec"
	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

// ScanServiceName checks that the length, prefix and suffix conform to
// systemd conventions and ensures the service name includes the word 'nomad'
func ScanServiceName(input string) error {
	// invalid if prefix and suffix together are < 255 char
	if len(input) > 255 {
		return errors.New("service name too long")
	}

	if isNomad := strings.Contains(input, "nomad"); !isNomad {
		return errors.New(`service name must include 'nomad' and conform to systemd conventions`)
	}

	// only allow ., :, @ , - , _ and \
	re := regexp.MustCompile(`[\w.:@_\-\\]*`)

	// Remove all matches and error if the returned string isn't empty
	// to ensure only characters that match are present
	safe := re.ReplaceAllString(input, "")
	if len(safe) != 0 {
		return fmt.Errorf("these strings did not match %s", safe)
	}

	// if there is a suffix, check against list of valid suffixes
	splitInput := strings.Split(input, ".")
	if len(splitInput) > 1 {
		suffix := splitInput[len(splitInput)-1]
		validSuffix := []string{
			"service",
			"socket",
			"device",
			"mount",
			"automount",
			"swap",
			"target",
			"path",
			"timer",
			"slice",
			"scope"}

		if valid := slices.Contains(validSuffix, suffix); !valid {
			return errors.New("invalid suffix")
		}
	}
	return nil
}

// Stream Helpers
type StreamReader struct {
	framer *sframer.StreamFramer
	ch     <-chan []byte
	buf    []byte
}

func NewStreamReader(ch <-chan []byte, framer *sframer.StreamFramer) *StreamReader {
	return &StreamReader{
		ch:     ch,
		framer: framer,
	}

}

func (r *StreamReader) Read(p []byte) (n int, err error) {
	select {
	case data, ok := <-r.ch:
		if !ok && len(data) == 0 {
			return 0, io.EOF
		}
		r.buf = data

	default:
		return 0, nil
	}

	n = copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

func (r *StreamReader) StreamFixed(ctx context.Context, offset int64, path string, limit int64,
	eofCancelCh chan error, cancelAfterFirstEof bool) error {

	parseFramerErr := func(err error) error {
		if err == nil {
			return nil
		}
		errMsg := err.Error()

		if strings.Contains(errMsg, io.ErrClosedPipe.Error()) {
			// The pipe check is for tests
			return syscall.EPIPE
		}

		// The connection was closed by our peer
		if strings.Contains(errMsg, syscall.EPIPE.Error()) || strings.Contains(errMsg, syscall.ECONNRESET.Error()) {
			return syscall.EPIPE
		}

		if strings.Contains(errMsg, "forcibly closed") {
			return syscall.EPIPE
		}

		return err
	}
	// streamFrameSize is the maximum number of bytes to send in a single frame
	streamFrameSize := int64(1024)

	bufSize := streamFrameSize
	if limit > 0 && limit < streamFrameSize {
		bufSize = limit
	}
	streamBuffer := make([]byte, bufSize)

	var lastEvent string

	// Only watch file when there is a need for it
	cancelReceived := cancelAfterFirstEof

OUTER:
	for {

		// Read up to the max frame size
		n, readErr := r.Read(streamBuffer)
		// Update the offset
		offset += int64(n)

		// Return non-EOF errors
		if readErr != nil && readErr != io.EOF {
			return readErr
		}

		// Send the frame
		if n != 0 || lastEvent != "" {
			if err := r.framer.Send(path, lastEvent, streamBuffer[:n], offset); err != nil {
				return parseFramerErr(err)
			}
		}

		// Clear the last event
		if lastEvent != "" {
			lastEvent = ""
		}

		// Just keep reading since we aren't at the end of the file so we can
		// avoid setting up a file event watcher.
		if readErr == nil {
			continue
		}

		// At this point we can stop without waiting for more changes,
		// because we have EOF and either we're not following at all,
		// or we received an event from the eofCancelCh channel
		// and last read was executed
		if cancelReceived {
			return nil
		}

		for {
			select {
			case <-r.framer.ExitCh():
				return nil
			case <-ctx.Done():
				return nil
			case _, ok := <-eofCancelCh:
				if !ok {
					return nil
				}
				cancelReceived = true
				continue OUTER
			}
		}
	}
}

func (r *StreamReader) Destroy() {
	r.framer.Destroy()
}

func (r *StreamReader) Run() {
	r.framer.Run()
}

type StreamEncoder struct {
	buf        *bytes.Buffer
	conn       io.ReadWriteCloser
	encoder    *codec.Encoder
	frameCodec *codec.Encoder
	plainText  bool
}

func NewStreamEncoder(buf *bytes.Buffer, conn io.ReadWriteCloser, encoder *codec.Encoder, frameCodec *codec.Encoder,
	plainText bool) StreamEncoder {
	return StreamEncoder{
		buf:        buf,
		conn:       conn,
		encoder:    encoder,
		frameCodec: frameCodec,
		plainText:  plainText,
	}
}

func (s *StreamEncoder) EncodeStream(frames chan *sframer.StreamFrame, errCh chan error, ctx context.Context) (err error) {
	var streamErr error
OUTER:
	for {
		select {
		case frame, ok := <-frames:
			if !ok {
				// frame may have been closed when an error
				// occurred. Check once more for an error.
				select {
				case streamErr = <-errCh:
					return streamErr
					// There was a pending error!
				default:
					// No error, continue on
				}
				break OUTER
			}
			var resp cstructs.StreamErrWrapper
			if s.plainText {
				resp.Payload = frame.Data
			} else {
				if err := s.frameCodec.Encode(frame); err != nil && err != io.EOF {
					return err
				}

				resp.Payload = s.buf.Bytes()
				s.buf.Reset()
			}
			if err := s.encoder.Encode(resp); err != nil {

				return err
			}
			s.encoder.Reset(s.conn)

		case <-ctx.Done():
			break OUTER
		}
	}
	return nil
}

// Test Helpers
type StreamingClient interface {
	StreamingRpcHandler(string) (structs.StreamingRpcHandler, error)
}

func ExportMonitorClient_TestHelper(req cstructs.MonitorExportRequest, c StreamingClient) (*strings.Builder, error) {
	var (
		builder     strings.Builder
		returnedErr error
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
	timeout := time.After(3 * time.Second)

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
