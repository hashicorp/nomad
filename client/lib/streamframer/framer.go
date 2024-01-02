// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package framer

import (
	"bytes"
	"fmt"
	"sync"
	"time"
)

var (
	// HeartbeatStreamFrame is the StreamFrame to send as a heartbeat, avoiding
	// creating many instances of the empty StreamFrame
	HeartbeatStreamFrame = &StreamFrame{}
)

// StreamFrame is used to frame data of a file when streaming
type StreamFrame struct {
	// Offset is the offset the data was read from
	Offset int64 `json:",omitempty"`

	// Data is the read data
	Data []byte `json:",omitempty"`

	// File is the file that the data was read from
	File string `json:",omitempty"`

	// FileEvent is the last file event that occurred that could cause the
	// streams position to change or end
	FileEvent string `json:",omitempty"`
}

// IsHeartbeat returns if the frame is a heartbeat frame
func (s *StreamFrame) IsHeartbeat() bool {
	return s.Offset == 0 && len(s.Data) == 0 && s.File == "" && s.FileEvent == ""
}

func (s *StreamFrame) Clear() {
	s.Offset = 0
	s.Data = nil
	s.File = ""
	s.FileEvent = ""
}

func (s *StreamFrame) IsCleared() bool {
	if s.Offset != 0 {
		return false
	} else if s.Data != nil {
		return false
	} else if s.File != "" {
		return false
	} else if s.FileEvent != "" {
		return false
	} else {
		return true
	}
}

func (s *StreamFrame) Copy() *StreamFrame {
	n := new(StreamFrame)
	*n = *s
	n.Data = make([]byte, len(s.Data))
	copy(n.Data, s.Data)
	return n
}

// StreamFramer is used to buffer and send frames as well as heartbeat.
type StreamFramer struct {
	// out is where frames are sent and is closed when no more frames will
	// be sent.
	out chan<- *StreamFrame

	frameSize int

	heartbeat *time.Ticker
	flusher   *time.Ticker

	// shutdown is true when a shutdown is triggered
	shutdown bool

	// shutdownCh is closed when no more Send()s will be called and run()
	// should flush pending frames before closing exitCh
	shutdownCh chan struct{}

	// exitCh is closed when the run() goroutine exits and no more frames
	// will be sent.
	exitCh chan struct{}

	// The mutex protects everything below
	l sync.Mutex

	// The current working frame
	f    *StreamFrame
	data *bytes.Buffer

	// Captures whether the framer is running
	running bool
}

// NewStreamFramer creates a new stream framer that will output StreamFrames to
// the passed output channel.
func NewStreamFramer(out chan<- *StreamFrame,
	heartbeatRate, batchWindow time.Duration, frameSize int) *StreamFramer {

	// Create the heartbeat and flush ticker
	heartbeat := time.NewTicker(heartbeatRate)
	flusher := time.NewTicker(batchWindow)

	return &StreamFramer{
		out:        out,
		frameSize:  frameSize,
		heartbeat:  heartbeat,
		flusher:    flusher,
		f:          new(StreamFrame),
		data:       bytes.NewBuffer(make([]byte, 0, 2*frameSize)),
		shutdownCh: make(chan struct{}),
		exitCh:     make(chan struct{}),
	}
}

// Destroy is used to cleanup the StreamFramer and flush any pending frames
func (s *StreamFramer) Destroy() {
	s.l.Lock()

	wasShutdown := s.shutdown
	s.shutdown = true

	if !wasShutdown {
		close(s.shutdownCh)
	}

	s.heartbeat.Stop()
	s.flusher.Stop()
	running := s.running
	s.l.Unlock()

	// Ensure things were flushed
	if running {
		<-s.exitCh
	}

	// Close out chan only after exitCh has exited
	if !wasShutdown {
		close(s.out)
	}
}

// Run starts a long lived goroutine that handles sending data as well as
// heartbeating
func (s *StreamFramer) Run() {
	s.l.Lock()
	defer s.l.Unlock()
	if s.running {
		return
	}

	s.running = true
	go s.run()
}

// ExitCh returns a channel that will be closed when the run loop terminates.
func (s *StreamFramer) ExitCh() <-chan struct{} {
	return s.exitCh
}

// run is the internal run method. It exits if Destroy is called or an error
// occurs, in which case the exit channel is closed.
func (s *StreamFramer) run() {
	defer func() {
		s.l.Lock()
		s.running = false
		s.l.Unlock()
		close(s.exitCh)
	}()

OUTER:
	for {
		select {
		case <-s.shutdownCh:
			break OUTER
		case <-s.flusher.C:
			// Skip if there is nothing to flush
			s.l.Lock()
			if s.f.IsCleared() {
				s.l.Unlock()
				continue
			}

			// Read the data for the frame, and send it
			s.send()
			s.l.Unlock()
		case <-s.heartbeat.C:
			// Send a heartbeat frame
			select {
			case s.out <- HeartbeatStreamFrame:
			case <-s.shutdownCh:
			}
		}
	}

	s.l.Lock()
	// Send() may have left a partial frame. Send it now.
	if !s.f.IsCleared() {
		s.f.Data = s.readData()

		// Only send if there's actually data left
		if len(s.f.Data) > 0 {
			// Cannot select on shutdownCh as it's already closed
			// Cannot select on exitCh as it's only closed after this exits
			s.out <- s.f.Copy()
		}
	}
	s.l.Unlock()
}

// send takes a StreamFrame, encodes and sends it
func (s *StreamFramer) send() {
	// Ensure s.out has not already been closd by Destroy
	select {
	case <-s.exitCh:
		return
	default:
	}

	s.f.Data = s.readData()
	select {
	case s.out <- s.f.Copy():
		s.f.Clear()
	case <-s.exitCh:
	}
}

// readData is a helper which reads the buffered data returning up to the frame
// size of data. Must be called with the lock held. The returned value is
// invalid on the next read or write into the StreamFramer buffer
func (s *StreamFramer) readData() []byte {
	// Compute the amount to read from the buffer
	size := s.data.Len()
	if size > s.frameSize {
		size = s.frameSize
	}
	if size == 0 {
		return nil
	}
	d := s.data.Next(size)
	return d
}

// Send creates and sends a StreamFrame based on the passed parameters. An error
// is returned if the run routine hasn't run or encountered an error. Send is
// asynchronous and does not block for the data to be transferred.
func (s *StreamFramer) Send(file, fileEvent string, data []byte, offset int64) error {
	s.l.Lock()
	defer s.l.Unlock()
	// If we are not running, return the error that caused us to not run or
	// indicated that it was never started.
	if !s.running {
		return fmt.Errorf("StreamFramer not running")
	}

	// Check if not mergeable
	if !s.f.IsCleared() && (s.f.File != file || s.f.FileEvent != fileEvent) {
		// Flush the old frame
		s.send()
	}

	// Store the new data as the current frame.
	if s.f.IsCleared() {
		s.f.Offset = offset
		s.f.File = file
		s.f.FileEvent = fileEvent
	}

	// Write the data to the buffer
	s.data.Write(data)

	// Handle the delete case in which there is no data
	force := s.data.Len() == 0 && s.f.FileEvent != ""

	// Flush till we are under the max frame size
	for s.data.Len() >= s.frameSize || force {
		// Clear since are flushing the frame and capturing the file event.
		// Subsequent data frames will be flushed based on the data size alone
		// since they share the same fileevent.
		if force {
			force = false
		}

		// Ensure s.out has not already been closed by Destroy
		select {
		case <-s.exitCh:
			return nil
		default:
		}

		// Create a new frame to send it
		s.f.Data = s.readData()
		select {
		case s.out <- s.f.Copy():
		case <-s.exitCh:
			return nil
		}

		// Update the offset
		s.f.Offset += int64(len(s.f.Data))
	}

	return nil
}
