package agent

//go:generate codecgen -d 101 -o fs_endpoint.generated.go fs_endpoint.go

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/tomb.v1"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/hashicorp/nomad/client/allocdir"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hpcloud/tail/watch"
	"github.com/ugorji/go/codec"
)

var (
	allocIDNotPresentErr  = fmt.Errorf("must provide a valid alloc id")
	fileNameNotPresentErr = fmt.Errorf("must provide a file name")
	taskNotPresentErr     = fmt.Errorf("must provide task name")
	logTypeNotPresentErr  = fmt.Errorf("must provide log type (stdout/stderr)")
	clientNotRunning      = fmt.Errorf("node is not running a Nomad Client")
	invalidOrigin         = fmt.Errorf("origin must be start or end")
)

const (
	// streamFrameSize is the maximum number of bytes to send in a single frame
	streamFrameSize = 64 * 1024

	// streamHeartbeatRate is the rate at which a heartbeat will occur to detect
	// a closed connection without sending any additional data
	streamHeartbeatRate = 1 * time.Second

	// streamBatchWindow is the window in which file content is batched before
	// being flushed if the frame size has not been hit.
	streamBatchWindow = 200 * time.Millisecond

	// nextLogCheckRate is the rate at which we check for a log entry greater
	// than what we are watching for. This is to handle the case in which logs
	// rotate faster than we can detect and we have to rely on a normal
	// directory listing.
	nextLogCheckRate = 100 * time.Millisecond

	// deleteEvent and truncateEvent are the file events that can be sent in a
	// StreamFrame
	deleteEvent   = "file deleted"
	truncateEvent = "file truncated"

	// OriginStart and OriginEnd are the available parameters for the origin
	// argument when streaming a file. They respectively offset from the start
	// and end of a file.
	OriginStart = "start"
	OriginEnd   = "end"
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
	case strings.HasPrefix(path, "logs/"):
		return s.Logs(resp, req)
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
	// plainTxt determines whether we frame or just send plain text data.
	plainTxt bool

	out     io.WriteCloser
	enc     *codec.Encoder
	encLock sync.Mutex

	frameSize int

	heartbeat *time.Ticker
	flusher   *time.Ticker

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
	Err     error
}

// NewStreamFramer creates a new stream framer that will output StreamFrames to
// the passed output. If plainTxt is set we do not frame and just batch plain
// text data.
func NewStreamFramer(out io.WriteCloser, plainTxt bool,
	heartbeatRate, batchWindow time.Duration, frameSize int) *StreamFramer {

	// Create a JSON encoder
	enc := codec.NewEncoder(out, structs.JsonHandle)

	// Create the heartbeat and flush ticker
	heartbeat := time.NewTicker(heartbeatRate)
	flusher := time.NewTicker(batchWindow)

	return &StreamFramer{
		plainTxt:   plainTxt,
		out:        out,
		enc:        enc,
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
	close(s.shutdownCh)
	s.heartbeat.Stop()
	s.flusher.Stop()
	running := s.running
	s.l.Unlock()

	// Ensure things were flushed
	if running {
		<-s.exitCh
	}
	s.out.Close()
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
	var err error
	defer func() {
		close(s.exitCh)
		s.l.Lock()
		s.running = false
		s.Err = err
		s.l.Unlock()
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
	s.encLock.Lock()
	defer s.encLock.Unlock()
	if s.plainTxt {
		_, err := io.Copy(s.out, bytes.NewReader(f.Data))
		return err
	}
	return s.enc.Encode(f)
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
		if s.Err != nil {
			return s.Err
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
	force := false
	if s.data.Len() == 0 && s.f.FileEvent != "" {
		force = true
	}

	// Flush till we are under the max frame size
	for s.data.Len() >= s.frameSize || force {
		// Clear
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

	// Create the framer
	framer := NewStreamFramer(output, false, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
	framer.Run()
	defer framer.Destroy()

	err = s.stream(offset, path, fs, framer, nil)
	if err != nil && err != syscall.EPIPE {
		return nil, err
	}

	return nil, nil
}

// stream is the internal method to stream the content of a file. eofCancelCh is
// used to cancel the stream if triggered while at EOF. If the connection is
// broken an EPIPE error is returned
func (s *HTTPServer) stream(offset int64, path string,
	fs allocdir.AllocDirFS, framer *StreamFramer,
	eofCancelCh chan error) error {

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

	// parseFramerErr takes an error and returns an error. The error will
	// potentially change if it was caused by the connection being closed.
	parseFramerErr := func(e error) error {
		if e == nil {
			return nil
		}

		if strings.Contains(e.Error(), io.ErrClosedPipe.Error()) {
			// The pipe check is for tests
			return syscall.EPIPE
		}

		// The connection was closed by our peer
		if strings.Contains(e.Error(), syscall.EPIPE.Error()) || strings.Contains(e.Error(), syscall.ECONNRESET.Error()) {
			return syscall.EPIPE
		}

		return err
	}

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
		if n != 0 || lastEvent != "" {
			if err := framer.Send(path, lastEvent, data[:n], offset); err != nil {
				return parseFramerErr(err)
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
				return parseFramerErr(framer.Send(path, deleteEvent, nil, offset))
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
				return parseFramerErr(framer.Err)
			case err, ok := <-eofCancelCh:
				if !ok {
					return nil
				}

				return err
			}
		}
	}
}

// Logs streams the content of a log blocking on EOF. The parameters are:
// * task: task name to stream logs for.
// * type: stdout/stderr to stream.
// * follow: A boolean of whether to follow the logs.
// * offset: The offset to start streaming data at, defaults to zero.
// * origin: Either "start" or "end" and defines from where the offset is
//           applied. Defaults to "start".
func (s *HTTPServer) Logs(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var allocID, task, logType string
	var plain, follow bool
	var err error

	q := req.URL.Query()

	if allocID = strings.TrimPrefix(req.URL.Path, "/v1/client/fs/logs/"); allocID == "" {
		return nil, allocIDNotPresentErr
	}

	if task = q.Get("task"); task == "" {
		return nil, taskNotPresentErr
	}

	if followStr := q.Get("follow"); followStr != "" {
		if follow, err = strconv.ParseBool(followStr); err != nil {
			return nil, fmt.Errorf("Failed to parse follow field to boolean: %v", err)
		}
	}

	if plainStr := q.Get("plain"); plainStr != "" {
		if plain, err = strconv.ParseBool(plainStr); err != nil {
			return nil, fmt.Errorf("Failed to parse plain field to boolean: %v", err)
		}
	}

	logType = q.Get("type")
	switch logType {
	case "stdout", "stderr":
	default:
		return nil, logTypeNotPresentErr
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

	alloc, err := s.agent.client.GetClientAlloc(allocID)
	if err != nil {
		return nil, err
	}

	// Check that the task is there
	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)
	if tg == nil {
		return nil, fmt.Errorf("Failed to lookup task group for allocation")
	} else if taskStruct := tg.LookupTask(task); taskStruct == nil {
		return nil, CodedError(404, fmt.Sprintf("task group %q does not have task with name %q", alloc.TaskGroup, task))
	}

	state, ok := alloc.TaskStates[task]
	if !ok || state.StartedAt.IsZero() {
		return nil, CodedError(404, fmt.Sprintf("task %q not started yet. No logs available", task))
	}

	// Create an output that gets flushed on every write
	output := ioutils.NewWriteFlusher(resp)

	return nil, s.logs(follow, plain, offset, origin, task, logType, fs, output)
}

func (s *HTTPServer) logs(follow, plain bool, offset int64,
	origin, task, logType string,
	fs allocdir.AllocDirFS, output io.WriteCloser) error {

	// Create the framer
	framer := NewStreamFramer(output, plain, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
	framer.Run()
	defer framer.Destroy()

	// Path to the logs
	logPath := filepath.Join(allocdir.SharedAllocName, allocdir.LogDirName)

	// nextIdx is the next index to read logs from
	var nextIdx int64
	switch origin {
	case "start":
		nextIdx = 0
	case "end":
		nextIdx = math.MaxInt64
		offset *= -1
	default:
		return invalidOrigin
	}

	// Create a tomb to cancel watch events
	t := tomb.Tomb{}
	defer func() {
		t.Kill(nil)
		t.Done()
	}()

	for {
		// Logic for picking next file is:
		// 1) List log files
		// 2) Pick log file closest to desired index
		// 3) Open log file at correct offset
		// 3a) No error, read contents
		// 3b) If file doesn't exist, goto 1 as it may have been rotated out
		entries, err := fs.List(logPath)
		if err != nil {
			return fmt.Errorf("failed to list entries: %v", err)
		}

		// If we are not following logs, determine the max index for the logs we are
		// interested in so we can stop there.
		maxIndex := int64(math.MaxInt64)
		if !follow {
			_, idx, _, err := findClosest(entries, maxIndex, 0, task, logType)
			if err != nil {
				return err
			}
			maxIndex = idx
		}

		logEntry, idx, openOffset, err := findClosest(entries, nextIdx, offset, task, logType)
		if err != nil {
			return err
		}

		var eofCancelCh chan error
		exitAfter := false
		if !follow && idx > maxIndex {
			// Exceeded what was there initially so return
			return nil
		} else if !follow && idx == maxIndex {
			// At the end
			eofCancelCh = make(chan error)
			close(eofCancelCh)
			exitAfter = true
		} else {
			eofCancelCh = blockUntilNextLog(fs, &t, logPath, task, logType, idx+1)
		}

		p := filepath.Join(logPath, logEntry.Name)
		err = s.stream(openOffset, p, fs, framer, eofCancelCh)

		if err != nil {
			// Check if there was an error where the file does not exist. That means
			// it got rotated out from under us.
			if os.IsNotExist(err) {
				continue
			}

			// Check if the connection was closed
			if err == syscall.EPIPE {
				return nil
			}

			return fmt.Errorf("failed to stream %q: %v", p, err)
		}

		if exitAfter {
			return nil
		}

		//Since we successfully streamed, update the overall offset/idx.
		offset = int64(0)
		nextIdx = idx + 1
	}
}

// blockUntilNextLog returns a channel that will have data sent when the next
// log index or anything greater is created.
func blockUntilNextLog(fs allocdir.AllocDirFS, t *tomb.Tomb, logPath, task, logType string, nextIndex int64) chan error {
	nextPath := filepath.Join(logPath, fmt.Sprintf("%s.%s.%d", task, logType, nextIndex))
	next := make(chan error, 1)

	go func() {
		eofCancelCh, err := fs.BlockUntilExists(nextPath, t)
		if err != nil {
			next <- err
			close(next)
			return
		}

		ticker := time.NewTicker(nextLogCheckRate)
		defer ticker.Stop()
		scanCh := ticker.C
		for {
			select {
			case <-t.Dead():
				next <- fmt.Errorf("shutdown triggered")
				close(next)
				return
			case err := <-eofCancelCh:
				next <- err
				close(next)
				return
			case <-scanCh:
				entries, err := fs.List(logPath)
				if err != nil {
					next <- fmt.Errorf("failed to list entries: %v", err)
					close(next)
					return
				}

				indexes, err := logIndexes(entries, task, logType)
				if err != nil {
					next <- err
					close(next)
					return
				}

				// Scan and see if there are any entries larger than what we are
				// waiting for.
				for _, entry := range indexes {
					if entry.idx >= nextIndex {
						next <- nil
						close(next)
						return
					}
				}
			}
		}
	}()

	return next
}

// indexTuple and indexTupleArray are used to find the correct log entry to
// start streaming logs from
type indexTuple struct {
	idx   int64
	entry *allocdir.AllocFileInfo
}

type indexTupleArray []indexTuple

func (a indexTupleArray) Len() int           { return len(a) }
func (a indexTupleArray) Less(i, j int) bool { return a[i].idx < a[j].idx }
func (a indexTupleArray) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// logIndexes takes a set of entries and returns a indexTupleArray of
// the desired log file entries. If the indexes could not be determined, an
// error is returned.
func logIndexes(entries []*allocdir.AllocFileInfo, task, logType string) (indexTupleArray, error) {
	var indexes []indexTuple
	prefix := fmt.Sprintf("%s.%s.", task, logType)
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}

		// If nothing was trimmed, then it is not a match
		idxStr := strings.TrimPrefix(entry.Name, prefix)
		if idxStr == entry.Name {
			continue
		}

		// Convert to an int
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			return nil, fmt.Errorf("failed to convert %q to a log index: %v", idxStr, err)
		}

		indexes = append(indexes, indexTuple{idx: int64(idx), entry: entry})
	}

	return indexTupleArray(indexes), nil
}

// findClosest takes a list of entries, the desired log index and desired log
// offset (which can be negative, treated as offset from end), task name and log
// type and returns the log entry, the log index, the offset to read from and a
// potential error.
func findClosest(entries []*allocdir.AllocFileInfo, desiredIdx, desiredOffset int64,
	task, logType string) (*allocdir.AllocFileInfo, int64, int64, error) {

	// Build the matching indexes
	indexes, err := logIndexes(entries, task, logType)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(indexes) == 0 {
		return nil, 0, 0, fmt.Errorf("log entry for task %q and log type %q not found", task, logType)
	}

	// Binary search the indexes to get the desiredIdx
	sort.Sort(indexTupleArray(indexes))
	i := sort.Search(len(indexes), func(i int) bool { return indexes[i].idx >= desiredIdx })
	l := len(indexes)
	if i == l {
		// Use the last index if the number is bigger than all of them.
		i = l - 1
	}

	// Get to the correct offset
	offset := desiredOffset
	idx := int64(i)
	for {
		s := indexes[idx].entry.Size

		// Base case
		if offset == 0 {
			break
		} else if offset < 0 {
			// Going backwards
			if newOffset := s + offset; newOffset >= 0 {
				// Current file works
				offset = newOffset
				break
			} else if idx == 0 {
				// Already at the end
				offset = 0
				break
			} else {
				// Try the file before
				offset = newOffset
				idx -= 1
				continue
			}
		} else {
			// Going forward
			if offset <= s {
				// Current file works
				break
			} else if idx == int64(l-1) {
				// Already at the end
				offset = s
				break
			} else {
				// Try the next file
				offset = offset - s
				idx += 1
				continue
			}

		}
	}

	return indexes[idx].entry, indexes[idx].idx, offset, nil
}
