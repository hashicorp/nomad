// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package monitor

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"syscall"

	"github.com/hashicorp/go-msgpack/v2/codec"
	sframer "github.com/hashicorp/nomad/client/lib/streamframer"
	cstructs "github.com/hashicorp/nomad/client/structs"
)

// StreamReader is used to process fixed length streams for consumers
// that rely on terminating the stream after hitting an EOF. The lock
// protects the buffer during reads
type StreamReader struct {
	sync.Mutex
	framer    *sframer.StreamFramer
	ch        <-chan []byte
	buf       []byte
	frameSize int64
}

// NewStreamReader takes a <-chan[]byte and *sframer.StreamFramer and returns
// a ready to use StreamReader that will allocate its buffer on first read
func NewStreamReader(ch <-chan []byte, framer *sframer.StreamFramer, frameSize int64) *StreamReader {
	return &StreamReader{
		ch:        ch,
		framer:    framer,
		frameSize: frameSize,
	}
}

// Read reads stream data into the StreamReader's buffer and copies that
// data into p
func (r *StreamReader) Read(p []byte) (n int, err error) {
	select {
	case data, ok := <-r.ch:
		if !ok && len(data) == 0 {
			return 0, io.EOF
		}
		r.Lock()
		r.buf = data
	default:
		return 0, nil
	}

	n = copy(p, r.buf)
	r.buf = r.buf[n:]
	r.Unlock()
	return n, nil
}

// StreamFixed streams any fixed length data stream. If limit is greater than
// zero, the stream will end once that many bytes have been read. If eofCancelCh
// is triggered while at EOF, read one more frame and cancel the stream on the
// next EOF. If the connection is broken an EPIPE error is returned.
func (r *StreamReader) StreamFixed(ctx context.Context, offset int64, path string, limit int64,
	eofCancelCh chan error, cancelAfterFirstEof bool) error {
	defer r.framer.Flush()
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
	streamFrameSize := r.frameSize

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

// Destroy wraps the underlying framer's Destroy() call
func (r *StreamReader) Destroy() {
	r.framer.Destroy()
}

// Run wraps the underlying framer's Run() call
func (r *StreamReader) Run() {
	r.framer.Run()
}

// StreamEncoder consolidates logic used by monitor RPC handlers to encode and
// return stream data
type StreamEncoder struct {
	buf        *bytes.Buffer
	conn       io.ReadWriteCloser
	encoder    *codec.Encoder
	frameCodec *codec.Encoder
	plainText  bool
}

// NewStreamEncoder takes buf *bytes.Buffer, conn io.ReadWriteCloser, encoder *codec.Encoder
// frameCodec *codec.Encoder,and plainText bool and returns a NewStreamEncoder
func NewStreamEncoder(buf *bytes.Buffer, conn io.ReadWriteCloser, encoder *codec.Encoder,
	frameCodec *codec.Encoder, plainText bool) StreamEncoder {
	return StreamEncoder{
		buf:        buf,
		conn:       conn,
		encoder:    encoder,
		frameCodec: frameCodec,
		plainText:  plainText,
	}
}

// EncodeStream reads and encodes data from a chan *sframer.Streamframe until the
// channel is closed. If eofCancel is true,EncodeStream continues to read from the closed
// channel until the underlying framer reports it has flushed it's final frame
func (s *StreamEncoder) EncodeStream(frames chan *sframer.StreamFrame,
	errCh chan error, ctx context.Context, framer *sframer.StreamFramer,
	eofCancel bool) (err error) {
	var streamErr error
	localFlush := false
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
					// No error, continue on and let exitCh control breaking
				}
				// Confirm framer.Flush and localFlush if we're expecting EOF
				if eofCancel {
					_, ok := <-framer.ExitCh()
					if !ok {
						if framer.IsFlushed() && !localFlush {
							localFlush = true
							continue
						} else if framer.IsFlushed() && localFlush {
							break OUTER
						}
					}
				} else {
					break OUTER
				}
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
