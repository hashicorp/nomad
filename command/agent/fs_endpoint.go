package agent

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/tomb.v1"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hpcloud/tail/watch"
	"github.com/ugorji/go/codec"
)

var (
	allocIDNotPresentErr  = fmt.Errorf("must provide a valid alloc id")
	fileNameNotPresentErr = fmt.Errorf("must provide a file name")
	clientNotRunning      = fmt.Errorf("node is not running a Nomad Client")
	invalidOrigin         = fmt.Errorf("origin must be start or end")
)

const (
	// streamFrameSize is the maximum number of bytes to send in a single frame
	streamFrameSize = 64 * 1024

	// streamHeartbeatRate is the rate at which a heartbeat will occur to detect
	// a closed connection without sending any additional data
	streamHeartbeatRate = 10 * time.Second

	// streamBatchWindow is the window in which file content is batched before
	// being flushed if the frame size has not been hit.
	streamBatchWindow = 200 * time.Millisecond

	deleteEvent   = "file deleted"
	truncateEvent = "file truncated"
)

func (s *HTTPServer) FsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.agent.client == nil {
		return nil, clientNotRunning
	}

	path := strings.TrimPrefix(req.URL.Path, "/v1/client/fs/")
	switch {
	case strings.HasPrefix(path, "ls/"):
		return s.DirectoryListRequest(resp, req)
	case strings.HasPrefix(path, "stat/"):
		return s.FileStatRequest(resp, req)
	case strings.HasPrefix(path, "readat/"):
		return s.FileReadAtRequest(resp, req)
	case strings.HasPrefix(path, "cat/"):
		return s.FileCatRequest(resp, req)
	case strings.HasPrefix(path, "stream/"):
		return s.Stream(resp, req)
	default:
		return nil, CodedError(404, ErrInvalidMethod)
	}
}

func (s *HTTPServer) DirectoryListRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var allocID, path string

	if allocID = strings.TrimPrefix(req.URL.Path, "/v1/client/fs/ls/"); allocID == "" {
		return nil, allocIDNotPresentErr
	}
	if path = req.URL.Query().Get("path"); path == "" {
		path = "/"
	}
	fs, err := s.agent.client.GetAllocFS(allocID)
	if err != nil {
		return nil, err
	}
	return fs.List(path)
}

func (s *HTTPServer) FileStatRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var allocID, path string
	if allocID = strings.TrimPrefix(req.URL.Path, "/v1/client/fs/stat/"); allocID == "" {
		return nil, allocIDNotPresentErr
	}
	if path = req.URL.Query().Get("path"); path == "" {
		return nil, fileNameNotPresentErr
	}
	fs, err := s.agent.client.GetAllocFS(allocID)
	if err != nil {
		return nil, err
	}
	return fs.Stat(path)
}

func (s *HTTPServer) FileReadAtRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var allocID, path string
	var offset, limit int64
	var err error

	q := req.URL.Query()

	if allocID = strings.TrimPrefix(req.URL.Path, "/v1/client/fs/readat/"); allocID == "" {
		return nil, allocIDNotPresentErr
	}
	if path = q.Get("path"); path == "" {
		return nil, fileNameNotPresentErr
	}

	if offset, err = strconv.ParseInt(q.Get("offset"), 10, 64); err != nil {
		return nil, fmt.Errorf("error parsing offset: %v", err)
	}

	// Parse the limit
	if limitStr := q.Get("limit"); limitStr != "" {
		if limit, err = strconv.ParseInt(limitStr, 10, 64); err != nil {
			return nil, fmt.Errorf("error parsing limit: %v", err)
		}
	}

	fs, err := s.agent.client.GetAllocFS(allocID)
	if err != nil {
		return nil, err
	}

	rc, err := fs.ReadAt(path, offset)
	if limit > 0 {
		rc = &ReadCloserWrapper{
			Reader: io.LimitReader(rc, limit),
			Closer: rc,
		}
	}

	if err != nil {
		return nil, err
	}

	io.Copy(resp, rc)
	return nil, rc.Close()
}

// ReadCloserWrapper wraps a LimitReader so that a file is closed once it has been
// read
type ReadCloserWrapper struct {
	io.Reader
	io.Closer
}

func (s *HTTPServer) FileCatRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var allocID, path string
	var err error

	q := req.URL.Query()

	if allocID = strings.TrimPrefix(req.URL.Path, "/v1/client/fs/cat/"); allocID == "" {
		return nil, allocIDNotPresentErr
	}
	if path = q.Get("path"); path == "" {
		return nil, fileNameNotPresentErr
	}
	fs, err := s.agent.client.GetAllocFS(allocID)
	if err != nil {
		return nil, err
	}

	fileInfo, err := fs.Stat(path)
	if err != nil {
		return nil, err
	}
	if fileInfo.IsDir {
		return nil, fmt.Errorf("file %q is a directory", path)
	}

	r, err := fs.ReadAt(path, int64(0))
	if err != nil {
		return nil, err
	}
	io.Copy(resp, r)
	return nil, r.Close()
}

// StreamFrame is used to frame data of a file when streaming
type StreamFrame struct {
	// Offset is the offset the data was read from
	Offset int64 `json:",omitempty"`

	// Data is the read data
	Data []byte `json:",omitempty"`

	// File is the file that the data was read from
	File string `json:",omitempty"`

	// FileEvent is the last file event that occured that could cause the
	// streams position to change or end
	FileEvent string `json:",omitempty"`
}

// IsHeartbeat returns if the frame is a heartbeat frame
func (s *StreamFrame) IsHeartbeat() bool {
	return s.Offset == 0 && len(s.Data) == 0 && s.File == "" && s.FileEvent == ""
}

// StreamFramer is used to buffer and send frames as well as heartbeat.
type StreamFramer struct {
	out        io.WriteCloser
	enc        *codec.Encoder
	frameSize  int
	heartbeat  *time.Ticker
	flusher    *time.Ticker
	shutdownCh chan struct{}
	exitCh     chan struct{}

	outbound chan *StreamFrame

	// The mutex protects everything below
	l sync.Mutex

	// The current working frame
	f    *StreamFrame
	data *bytes.Buffer

	// Captures whether the framer is running and any error that occured to
	// cause it to stop.
	running bool
	err     error
}

// NewStreamFramer creates a new stream framer that will output StreamFrames to
// the passed output.
func NewStreamFramer(out io.WriteCloser, heartbeatRate, batchWindow time.Duration, frameSize int) *StreamFramer {
	// Create a JSON encoder
	enc := codec.NewEncoder(out, jsonHandle)

	// Create the heartbeat and flush ticker
	heartbeat := time.NewTicker(heartbeatRate)
	flusher := time.NewTicker(batchWindow)

	return &StreamFramer{
		out:        out,
		enc:        enc,
		frameSize:  frameSize,
		heartbeat:  heartbeat,
		flusher:    flusher,
		outbound:   make(chan *StreamFrame),
		data:       bytes.NewBuffer(make([]byte, 0, 2*frameSize)),
		shutdownCh: make(chan struct{}),
		exitCh:     make(chan struct{}),
	}
}

// Destroy is used to cleanup the StreamFramer and flush any pending frames
func (s *StreamFramer) Destroy() {
	s.l.Lock()
	wasRunning := s.running
	s.running = false
	s.f = nil
	close(s.shutdownCh)
	s.heartbeat.Stop()
	s.flusher.Stop()
	s.l.Unlock()

	// Ensure things were flushed
	if wasRunning {
		<-s.exitCh
	}
}

// Run starts a long lived goroutine that handles sending data as well as
// heartbeating
func (s *StreamFramer) Run() {
	s.l.Lock()
	if s.running {
		return
	}

	s.running = true
	s.l.Unlock()

	go s.run()
}

// ExitCh returns a channel that will be closed when the run loop terminates.
func (s *StreamFramer) ExitCh() <-chan struct{} {
	return s.exitCh
}

// run is the internal run method. It exits if Destroy is called or an error
// occurs, in which case the exit channel is closed.
func (s *StreamFramer) run() {
	// Store any error and mark it as not running
	var err error
	defer func() {
		s.l.Lock()
		s.err = err
		s.out.Close()
		close(s.exitCh)
		close(s.outbound)
		s.l.Unlock()
	}()

	// Start a heartbeat/flusher go-routine. This is done seprately to avoid blocking
	// the outbound channel.
	go func() {
		for {
			select {
			case <-s.shutdownCh:
				return
			case <-s.flusher.C:
				// Skip if there is nothing to flush
				s.l.Lock()
				if s.f == nil {
					s.l.Unlock()
					continue
				}

				// Read the data for the frame, and send it
				s.f.Data = s.readData()
				s.outbound <- s.f
				s.f = nil

				s.l.Unlock()
			case <-s.heartbeat.C:
				// Send a heartbeat frame
				s.outbound <- &StreamFrame{}
			}
		}
	}()

OUTER:
	for {
		select {
		case <-s.shutdownCh:
			break OUTER
		case o := <-s.outbound:
			// Send the frame and then clear the current working frame
			if err = s.enc.Encode(o); err != nil {
				return
			}
		}
	}

	// Flush any existing frames
	s.l.Lock()
	defer s.l.Unlock()
	select {
	case o := <-s.outbound:
		// Send the frame and then clear the current working frame
		if err = s.enc.Encode(o); err != nil {
			return
		}
	default:
	}

	if s.f != nil {
		s.f.Data = s.readData()
		s.enc.Encode(s.f)
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
	return s.data.Next(size)
}

// Send creates and sends a StreamFrame based on the passed parameters. An error
// is returned if the run routine hasn't run or encountered an error. Send is
// asyncronous and does not block for the data to be transferred.
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
	if s.f != nil && (s.f.File != file || s.f.FileEvent != fileEvent) {
		// Flush the old frame
		s.outbound <- &StreamFrame{
			Offset:    s.f.Offset,
			File:      s.f.File,
			FileEvent: s.f.FileEvent,
			Data:      s.readData(),
		}
		s.f = nil
	}

	// Store the new data as the current frame.
	if s.f == nil {
		s.f = &StreamFrame{
			Offset:    offset,
			File:      file,
			FileEvent: fileEvent,
		}
	}

	// Write the data to the buffer
	s.data.Write(data)

	// Handle the delete case in which there is no data
	if s.data.Len() == 0 && s.f.FileEvent != "" {
		s.outbound <- &StreamFrame{
			Offset:    s.f.Offset,
			File:      s.f.File,
			FileEvent: s.f.FileEvent,
		}
	}

	// Flush till we are under the max frame size
	for s.data.Len() >= s.frameSize {
		// Create a new frame to send it
		s.outbound <- &StreamFrame{
			Offset:    s.f.Offset,
			File:      s.f.File,
			FileEvent: s.f.FileEvent,
			Data:      s.readData(),
		}
	}

	if s.data.Len() == 0 {
		s.f = nil
	}

	return nil
}

// Stream streams the content of a file blocking on EOF.
// The parameters are:
// * path: path to file to stream.
// * offset: The offset to start streaming data at, defaults to zero.
// * origin: Either "start" or "end" and defines from where the offset is
//           applied. Defaults to "start".
func (s *HTTPServer) Stream(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var allocID, path string
	var err error

	q := req.URL.Query()

	if allocID = strings.TrimPrefix(req.URL.Path, "/v1/client/fs/stream/"); allocID == "" {
		return nil, allocIDNotPresentErr
	}

	if path = q.Get("path"); path == "" {
		return nil, fileNameNotPresentErr
	}

	var offset int64
	offsetString := q.Get("offset")
	if offsetString != "" {
		var err error
		if offset, err = strconv.ParseInt(offsetString, 10, 64); err != nil {
			return nil, fmt.Errorf("error parsing offset: %v", err)
		}
	}

	origin := q.Get("origin")
	switch origin {
	case "start", "end":
	case "":
		origin = "start"
	default:
		return nil, invalidOrigin
	}

	fs, err := s.agent.client.GetAllocFS(allocID)
	if err != nil {
		return nil, err
	}

	fileInfo, err := fs.Stat(path)
	if err != nil {
		return nil, err
	}
	if fileInfo.IsDir {
		return nil, fmt.Errorf("file %q is a directory", path)
	}

	// If offsetting from the end subtract from the size
	if origin == "end" {
		offset = fileInfo.Size - offset

	}

	// Create an output that gets flushed on every write
	output := ioutils.NewWriteFlusher(resp)

	return nil, s.stream(offset, path, fs, output)
}

func (s *HTTPServer) stream(offset int64, path string, fs allocdir.AllocDirFS, output io.WriteCloser) error {
	// Get the reader
	f, err := fs.ReadAt(path, offset)
	if err != nil {
		return err
	}
	defer f.Close()

	// Create a tomb to cancel watch events
	t := tomb.Tomb{}
	defer func() {
		t.Kill(nil)
		t.Done()
	}()

	// Create the framer
	framer := NewStreamFramer(output, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
	framer.Run()
	defer framer.Destroy()

	// Create a variable to allow setting the last event
	var lastEvent string

	// Only create the file change watcher once. But we need to do it after we
	// read and reach EOF.
	var changes *watch.FileChanges

	// Start streaming the data
	data := make([]byte, streamFrameSize)
OUTER:
	for {
		// Read up to the max frame size
		n, readErr := f.Read(data)

		// Update the offset
		offset += int64(n)

		// Return non-EOF errors
		if readErr != nil && readErr != io.EOF {
			return readErr
		}

		// Send the frame
		if n != 0 {
			if err := framer.Send(path, lastEvent, data[:n], offset); err != nil {
				return err
			}
		}

		// Clear the last event
		if lastEvent != "" {
			lastEvent = ""
		}

		// Just keep reading
		if readErr == nil {
			continue
		}

		// If EOF is hit, wait for a change to the file
		if changes == nil {
			changes, err = fs.ChangeEvents(path, offset, &t)
			if err != nil {
				return err
			}
		}

		for {
			select {
			case <-changes.Modified:
				continue OUTER
			case <-changes.Deleted:
				return framer.Send(path, deleteEvent, nil, offset)
			case <-changes.Truncated:
				// Close the current reader
				if err := f.Close(); err != nil {
					return err
				}

				// Get a new reader at offset zero
				offset = 0
				var err error
				f, err = fs.ReadAt(path, offset)
				if err != nil {
					return err
				}
				defer f.Close()

				// Store the last event
				lastEvent = truncateEvent
				continue OUTER
			case <-framer.ExitCh():
				return nil
			}
		}
	}

	return nil
}
