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

// StreamFramer is used to buffer and send frames as well as heartbeat.
type StreamFramer struct {
	out chan<- *StreamFrame

	frameSize int

	heartbeat *time.Ticker
	flusher   *time.Ticker

	shutdown   bool
	shutdownCh chan struct{}
	exitCh     chan struct{}

	// The mutex protects everything below
	l sync.Mutex

	// The current working frame
	f    StreamFrame
	data *bytes.Buffer

	// Captures whether the framer is running and any error that occurred to
	// cause it to stop.
	running bool
	err     error
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

// Err returns the error that caused the StreamFramer to exit
func (s *StreamFramer) Err() error {
	s.l.Lock()
	defer s.l.Unlock()
	return s.err
}

// run is the internal run method. It exits if Destroy is called or an error
// occurs, in which case the exit channel is closed.
func (s *StreamFramer) run() {
	var err error
	defer func() {
		s.l.Lock()
		s.running = false
		s.err = err
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
			s.f.Data = s.readData()
			err = s.send(&s.f)
			s.f.Clear()
			s.l.Unlock()
			if err != nil {
				return
			}
		case <-s.heartbeat.C:
			// Send a heartbeat frame
			if err = s.send(HeartbeatStreamFrame); err != nil {
				return
			}
		}
	}

	s.l.Lock()
	if !s.f.IsCleared() {
		s.f.Data = s.readData()
		err = s.send(&s.f)
		s.f.Clear()
	}
	s.l.Unlock()
}

// send takes a StreamFrame, encodes and sends it
func (s *StreamFramer) send(f *StreamFrame) error {
	sending := *f
	f.Data = nil

	select {
	case s.out <- &sending:
		return nil
	case <-s.exitCh:
		return nil
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
		if s.err != nil {
			return s.err
		}

		return fmt.Errorf("StreamFramer not running")
	}

	// Check if not mergeable
	if !s.f.IsCleared() && (s.f.File != file || s.f.FileEvent != fileEvent) {
		// Flush the old frame
		s.f.Data = s.readData()
		select {
		case <-s.exitCh:
			return nil
		default:
		}
		err := s.send(&s.f)
		s.f.Clear()
		if err != nil {
			return err
		}
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

		// Create a new frame to send it
		s.f.Data = s.readData()
		select {
		case <-s.exitCh:
			return nil
		default:
		}

		if err := s.send(&s.f); err != nil {
			return err
		}

		// Update the offset
		s.f.Offset += int64(len(s.f.Data))
	}

	if s.data.Len() == 0 {
		s.f.Clear()
	}

	return nil
}
