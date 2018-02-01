package agent

//go:generate codecgen -d 101 -o fs_endpoint.generated.go fs_endpoint.go

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/hashicorp/nomad/acl"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
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

func (s *HTTPServer) FsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var secret string
	var namespace string
	s.parseToken(req, &secret)
	parseNamespace(req, &namespace)

	var aclObj *acl.ACL
	if s.agent.client != nil {
		var err error
		aclObj, err = s.agent.Client().ResolveToken(secret)
		if err != nil {
			return nil, err
		}
	}

	path := strings.TrimPrefix(req.URL.Path, "/v1/client/fs/")
	switch {
	case strings.HasPrefix(path, "ls/"):
		return s.DirectoryListRequest(resp, req)
	case strings.HasPrefix(path, "stat/"):
		return s.FileStatRequest(resp, req)
	case strings.HasPrefix(path, "readat/"):
		if aclObj != nil && !aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadFS) {
			return nil, structs.ErrPermissionDenied
		}
		return s.FileReadAtRequest(resp, req)
	case strings.HasPrefix(path, "cat/"):
		if aclObj != nil && !aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadFS) {
			return nil, structs.ErrPermissionDenied
		}
		return s.FileCatRequest(resp, req)
	//case strings.HasPrefix(path, "stream/"):
	//if aclObj != nil && !aclObj.AllowNsOp(namespace, acl.NamespaceCapabilityReadFS) {
	//return nil, structs.ErrPermissionDenied
	//}
	//return s.Stream(resp, req)
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

/*

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
	framer := sframer.NewStreamFramer(output, false, streamHeartbeatRate, streamBatchWindow, streamFrameSize)
	framer.Run()
	defer framer.Destroy()

	err = s.stream(offset, path, fs, framer, nil)
	if err != nil && err != syscall.EPIPE {
		return nil, err
	}

	return nil, nil
}

// parseFramerErr takes an error and returns an error. The error will
// potentially change if it was caused by the connection being closed.
func parseFramerErr(err error) error {
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

	// Windows version of ECONNRESET
	//XXX(schmichael) I could find no existing error or constant to
	//                compare this against.
	if strings.Contains(errMsg, "forcibly closed") {
		return syscall.EPIPE
	}

	return err
}

// stream is the internal method to stream the content of a file. eofCancelCh is
// used to cancel the stream if triggered while at EOF. If the connection is
// broken an EPIPE error is returned
func (s *HTTPServer) stream(offset int64, path string,
	fs allocdir.AllocDirFS, framer *sframer.StreamFramer,
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
				return parseFramerErr(framer.Err())
			case err, ok := <-eofCancelCh:
				if !ok {
					return nil
				}

				return err
			}
		}
	}
}
*/

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

	// Create an output that gets flushed on every write
	output := ioutils.NewWriteFlusher(resp)

	localClient := s.agent.Client()
	localServer := s.agent.Server()

	// See if the local client can handle the request.
	localAlloc := false
	if localClient != nil {
		_, err := localClient.GetClientAlloc(allocID)
		if err == nil {
			localAlloc = true
		}
	}

	// Only use the client RPC to server if we don't have a server and the local
	// client can't handle the call.
	useClientRPC := localClient != nil && !localAlloc && localServer == nil

	// Use the server as a last case.
	useServerRPC := localServer != nil

	// Get the correct handler
	var handler structs.StreamingRpcHandler
	var handlerErr error
	if localAlloc {
		handler, handlerErr = localClient.StreamingRpcHandler("FileSystem.Logs")
	} else if useClientRPC {
		handler, handlerErr = localClient.RemoteStreamingRpcHandler("FileSystem.Logs")
	} else if useServerRPC {
		handler, handlerErr = localServer.StreamingRpcHandler("FileSystem.Logs")
	}

	if handlerErr != nil {
		return nil, CodedError(500, handlerErr.Error())
	}

	// Create the request arguments
	fsReq := &cstructs.FsLogsRequest{
		AllocID:   allocID,
		Task:      task,
		LogType:   logType,
		Offset:    offset,
		Origin:    origin,
		PlainText: plain,
		Follow:    follow,
	}
	s.parse(resp, req, &fsReq.QueryOptions.Region, &fsReq.QueryOptions)

	p1, p2 := net.Pipe()
	decoder := codec.NewDecoder(p1, structs.MsgpackHandle)
	encoder := codec.NewEncoder(p1, structs.MsgpackHandle)

	// Create a goroutine that closes the pipe if the connection closes.
	ctx, cancel := context.WithCancel(req.Context())
	go func() {
		<-ctx.Done()
		p1.Close()
	}()

	// Create a channel that decodes the results
	errCh := make(chan HTTPCodedError)
	go func() {
		// Send the request
		if err := encoder.Encode(fsReq); err != nil {
			errCh <- CodedError(500, err.Error())
			cancel()
			return
		}

		for {
			select {
			case <-ctx.Done():
				errCh <- nil
				cancel()
				return
			default:
			}

			var res cstructs.StreamErrWrapper
			if err := decoder.Decode(&res); err != nil {
				errCh <- CodedError(500, err.Error())
				cancel()
				return
			}

			if err := res.Error; err != nil {
				if err.Code != nil {
					errCh <- CodedError(int(*err.Code), err.Error())
					cancel()
					return
				}
			}

			if _, err := io.Copy(output, bytes.NewBuffer(res.Payload)); err != nil {
				errCh <- CodedError(500, err.Error())
				cancel()
				return
			}
		}
	}()

	handler(p2)
	cancel()
	codedErr := <-errCh
	if codedErr != nil &&
		(codedErr == io.EOF ||
			strings.Contains(codedErr.Error(), "closed") ||
			strings.Contains(codedErr.Error(), "EOF")) {
		codedErr = nil
	}
	return nil, codedErr
}
