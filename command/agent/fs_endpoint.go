// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

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
	"github.com/hashicorp/go-msgpack/codec"
	cstructs "github.com/hashicorp/nomad/client/structs"
	"github.com/hashicorp/nomad/nomad/structs"
)

var (
	allocIDNotPresentErr  = CodedError(400, "must provide a valid alloc id")
	fileNameNotPresentErr = CodedError(400, "must provide a file name")
	taskNotPresentErr     = CodedError(400, "must provide task name")
	logTypeNotPresentErr  = CodedError(400, "must provide log type (stdout/stderr)")
	clientNotRunning      = CodedError(400, "node is not running a Nomad Client")
	invalidOrigin         = CodedError(400, "origin must be start or end")
)

func (s *HTTPServer) FsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	path := strings.TrimPrefix(req.URL.Path, "/v1/client/fs/")
	switch {
	case strings.HasPrefix(path, "ls/"):
		return s.DirectoryListRequest(resp, req)
	case strings.HasPrefix(path, "stat/"):
		return s.FileStatRequest(resp, req)
	case strings.HasPrefix(path, "readat/"):
		return s.wrapUntrustedContent(s.FileReadAtRequest)(resp, req)
	case strings.HasPrefix(path, "cat/"):
		return s.wrapUntrustedContent(s.FileCatRequest)(resp, req)
	case strings.HasPrefix(path, "stream/"):
		return s.Stream(resp, req)
	case strings.HasPrefix(path, "logs/"):
		// Logs are *trusted* content because the endpoint
		// explicitly sets the Content-Type to text/plain or
		// application/json depending on the value of the ?plain=
		// parameter.
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

	// Create the request
	args := &cstructs.FsListRequest{
		AllocID: allocID,
		Path:    path,
	}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

	// Make the RPC
	localClient, remoteClient, localServer := s.rpcHandlerForAlloc(allocID)

	var reply cstructs.FsListResponse
	var rpcErr error
	if localClient {
		rpcErr = s.agent.Client().ClientRPC("FileSystem.List", &args, &reply)
	} else if remoteClient {
		rpcErr = s.agent.Client().RPC("FileSystem.List", &args, &reply)
	} else if localServer {
		rpcErr = s.agent.Server().RPC("FileSystem.List", &args, &reply)
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) || structs.IsErrUnknownAllocation(rpcErr) || structs.IsErrNoSuchFileOrDirectory(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		}

		return nil, rpcErr
	}

	return reply.Files, nil
}

func (s *HTTPServer) FileStatRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var allocID, path string
	if allocID = strings.TrimPrefix(req.URL.Path, "/v1/client/fs/stat/"); allocID == "" {
		return nil, allocIDNotPresentErr
	}
	if path = req.URL.Query().Get("path"); path == "" {
		return nil, fileNameNotPresentErr
	}

	// Create the request
	args := &cstructs.FsStatRequest{
		AllocID: allocID,
		Path:    path,
	}
	s.parse(resp, req, &args.QueryOptions.Region, &args.QueryOptions)

	// Make the RPC
	localClient, remoteClient, localServer := s.rpcHandlerForAlloc(allocID)

	var reply cstructs.FsStatResponse
	var rpcErr error
	if localClient {
		rpcErr = s.agent.Client().ClientRPC("FileSystem.Stat", &args, &reply)
	} else if remoteClient {
		rpcErr = s.agent.Client().RPC("FileSystem.Stat", &args, &reply)
	} else if localServer {
		rpcErr = s.agent.Server().RPC("FileSystem.Stat", &args, &reply)
	}

	if rpcErr != nil {
		if structs.IsErrNoNodeConn(rpcErr) || structs.IsErrUnknownAllocation(rpcErr) || structs.IsErrNoSuchFileOrDirectory(rpcErr) {
			rpcErr = CodedError(404, rpcErr.Error())
		}

		return nil, rpcErr
	}

	return reply.Info, nil
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

	// Create the request arguments
	fsReq := &cstructs.FsStreamRequest{
		AllocID:   allocID,
		Path:      path,
		Offset:    offset,
		Origin:    "start",
		Limit:     limit,
		PlainText: true,
	}
	s.parse(resp, req, &fsReq.QueryOptions.Region, &fsReq.QueryOptions)

	// Make the request
	return s.fsStreamImpl(resp, req, "FileSystem.Stream", fsReq, fsReq.AllocID)
}

func (s *HTTPServer) FileCatRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var allocID, path string

	q := req.URL.Query()

	if allocID = strings.TrimPrefix(req.URL.Path, "/v1/client/fs/cat/"); allocID == "" {
		return nil, allocIDNotPresentErr
	}
	if path = q.Get("path"); path == "" {
		return nil, fileNameNotPresentErr
	}

	// Create the request arguments
	fsReq := &cstructs.FsStreamRequest{
		AllocID:   allocID,
		Path:      path,
		Origin:    "start",
		PlainText: true,
	}
	s.parse(resp, req, &fsReq.QueryOptions.Region, &fsReq.QueryOptions)

	// Make the request
	return s.fsStreamImpl(resp, req, "FileSystem.Stream", fsReq, fsReq.AllocID)
}

// Stream streams the content of a file blocking on EOF.
// The parameters are:
//   - path: path to file to stream.
//   - follow: A boolean of whether to follow the file, defaults to true.
//   - offset: The offset to start streaming data at, defaults to zero.
//   - origin: Either "start" or "end" and defines from where the offset is
//     applied. Defaults to "start".
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

	follow := true
	if followStr := q.Get("follow"); followStr != "" {
		if follow, err = strconv.ParseBool(followStr); err != nil {
			return nil, fmt.Errorf("failed to parse follow field to boolean: %v", err)
		}
	}

	var offset int64
	offsetString := q.Get("offset")
	if offsetString != "" {
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

	// Create the request arguments
	fsReq := &cstructs.FsStreamRequest{
		AllocID: allocID,
		Path:    path,
		Origin:  origin,
		Offset:  offset,
		Follow:  follow,
	}
	s.parse(resp, req, &fsReq.QueryOptions.Region, &fsReq.QueryOptions)

	// Make the request
	return s.fsStreamImpl(resp, req, "FileSystem.Stream", fsReq, fsReq.AllocID)
}

// Logs streams the content of a log blocking on EOF. The parameters are:
//   - task: task name to stream logs for.
//   - type: stdout/stderr to stream.
//   - follow: A boolean of whether to follow the logs.
//   - offset: The offset to start streaming data at, defaults to zero.
//   - origin: Either "start" or "end" and defines from where the offset is
//     applied. Defaults to "start".
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
			return nil, CodedError(400, fmt.Sprintf("failed to parse follow field to boolean: %v", err))
		}
	}

	if plainStr := q.Get("plain"); plainStr != "" {
		if plain, err = strconv.ParseBool(plainStr); err != nil {
			return nil, CodedError(400, fmt.Sprintf("failed to parse plain field to boolean: %v", err))
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
			return nil, CodedError(400, fmt.Sprintf("error parsing offset: %v", err))
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

	// Force the Content-Type to avoid Go's http.ResponseWriter from
	// detecting an incorrect or unsafe one.
	if plain {
		resp.Header().Set("Content-Type", "text/plain")
	} else {
		resp.Header().Set("Content-Type", "application/json")
	}

	// Make the request
	return s.fsStreamImpl(resp, req, "FileSystem.Logs", fsReq, fsReq.AllocID)
}

// fsStreamImpl is used to make a streaming filesystem call that serializes the
// args and then expects a stream of StreamErrWrapper results where the payload
// is copied to the response body.
func (s *HTTPServer) fsStreamImpl(resp http.ResponseWriter,
	req *http.Request, method string, args interface{}, allocID string) (interface{}, error) {

	// Get the correct handler
	localClient, remoteClient, localServer := s.rpcHandlerForAlloc(allocID)
	var handler structs.StreamingRpcHandler
	var handlerErr error
	if localClient {
		handler, handlerErr = s.agent.Client().StreamingRpcHandler(method)
	} else if remoteClient {
		handler, handlerErr = s.agent.Client().RemoteStreamingRpcHandler(method)
	} else if localServer {
		handler, handlerErr = s.agent.Server().StreamingRpcHandler(method)
	}

	if handlerErr != nil {
		return nil, CodedError(500, handlerErr.Error())
	}

	// Create a pipe connecting the (possibly remote) handler to the http response
	httpPipe, handlerPipe := net.Pipe()
	decoder := codec.NewDecoder(httpPipe, structs.MsgpackHandle)
	encoder := codec.NewEncoder(httpPipe, structs.MsgpackHandle)

	// Create a goroutine that closes the pipe if the connection closes.
	ctx, cancel := context.WithCancel(req.Context())
	go func() {
		<-ctx.Done()
		httpPipe.Close()
	}()

	// Create an output that gets flushed on every write
	output := ioutils.NewWriteFlusher(resp)

	// Create a channel that decodes the results
	errCh := make(chan HTTPCodedError)
	go func() {
		defer cancel()

		// Send the request
		if err := encoder.Encode(args); err != nil {
			errCh <- CodedError(500, err.Error())
			return
		}

		for {
			select {
			case <-ctx.Done():
				errCh <- nil
				return
			default:
			}

			var res cstructs.StreamErrWrapper
			if err := decoder.Decode(&res); err != nil {
				errCh <- CodedError(500, err.Error())
				return
			}
			decoder.Reset(httpPipe)

			if err := res.Error; err != nil {
				code := 500
				if err.Code != nil {
					code = int(*err.Code)
				}

				errCh <- CodedError(code, err.Error())
				return
			}

			if _, err := io.Copy(output, bytes.NewReader(res.Payload)); err != nil {
				errCh <- CodedError(500, err.Error())
				return
			}
		}
	}()

	handler(handlerPipe)
	cancel()
	codedErr := <-errCh

	// Ignore EOF and ErrClosedPipe errors.
	if codedErr != nil &&
		(codedErr == io.EOF ||
			strings.Contains(codedErr.Error(), "closed") ||
			strings.Contains(codedErr.Error(), "EOF")) {
		codedErr = nil
	}
	return nil, codedErr
}
