package agent

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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
	// frameSize is the maximum number of bytes to send in a single frame
	frameSize = 64 * 1024

	// streamHeartbeatRate is the rate at which a heartbeat will occur to detect
	// a closed connection without sending any additional data
	streamHeartbeatRate = 10 * time.Second

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

	var rc io.ReadCloser
	if limit > 0 {
		rc, err = fs.LimitReadAt(path, offset, limit)
	} else {
		rc, err = fs.ReadAt(path, offset)
	}

	if err != nil {
		return nil, err
	}

	defer rc.Close()
	io.Copy(resp, rc)
	return nil, nil
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
	return nil, nil
}

// StreamFrame is used to frame data of a file when streaming
type StreamFrame struct {
	// Offset is the offset the data was read from
	Offset int64

	// Data is the read data with Base64 byte encoding
	Data string

	// File is the file that the data was read from
	File string

	// FileEvent is the last file event that occured that could cause the
	// streams position to change or end
	FileEvent string
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
	// Create a JSON encoder
	enc := codec.NewEncoder(output, jsonHandle)

	// Get the reader
	f, err := fs.ReadAt(path, offset)
	if err != nil {
		return err
	}
	defer f.Close()

	// Create a tomb to cancel watch events
	t := tomb.Tomb{}
	defer t.Done()

	// Create the heartbeat timer
	ticker := time.NewTimer(streamHeartbeatRate)
	defer ticker.Stop()

	// Create a variable to allow setting the last event
	var lastEvent string

	// Only create the file change watcher once. But we need to do it after we
	// read and reach EOF.
	var changes *watch.FileChanges

	// Start streaming the data
OUTER:
	for {
		// Create a frame
		frame := StreamFrame{
			Offset: offset,
			File:   path,
		}
		data := make([]byte, frameSize)

		if lastEvent != "" {
			frame.FileEvent = lastEvent
			lastEvent = ""
		}

		// Read up to the max frame size
		n, err := f.Read(data)

		// Update the offset
		offset += int64(n)

		// Convert the data to Base64
		frame.Data = base64.StdEncoding.EncodeToString(data[:n])

		// Return non-EOF errors
		if err != nil && err != io.EOF {
			return err
		}

		// Send the frame
		if err := enc.Encode(&frame); err != nil {
			return err
		}

		// Just keep reading
		if err == nil {
			continue
		}

		// If EOF is hit, wait for a change to the file but periodically
		// heartbeat to ensure the socket is not closed
		if changes == nil {
			changes, err = fs.ChangeEvents(path, offset, &t)
			if err != nil {
				return err
			}
		}

		// Reset the heartbeat timer as we just started waiting
		ticker.Reset(streamHeartbeatRate)

		for {
			select {
			case <-changes.Modified:
				continue OUTER
			case <-changes.Deleted:
				// Send a heartbeat frame with the delete
				hFrame := StreamFrame{
					Offset:    offset,
					File:      path,
					FileEvent: deleteEvent,
				}

				if err := enc.Encode(&hFrame); err != nil {
					// The defer on the tomb will stop the watch
					return err
				}

				return nil
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
			case <-t.Dying():
				return nil
			case <-ticker.C:
				// Send a heartbeat frame
				hFrame := StreamFrame{
					Offset: offset,
					File:   path,
				}

				if err := enc.Encode(&hFrame); err != nil {
					// The defer on the tomb will stop the watch
					return err
				}

				ticker.Reset(streamHeartbeatRate)
			}
		}
	}

	return nil
}
