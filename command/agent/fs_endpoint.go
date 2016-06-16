package agent

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

var (
	allocIDNotPresentErr  = fmt.Errorf("must provide a valid alloc id")
	fileNameNotPresentErr = fmt.Errorf("must provide a file name")
	clientNotRunning      = fmt.Errorf("node is not running a Nomad Client")
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
	if limit, err = strconv.ParseInt(q.Get("limit"), 10, 64); err != nil {
		return nil, fmt.Errorf("error parsing limit: %v", err)
	}
	fs, err := s.agent.client.GetAllocFS(allocID)
	if err != nil {
		return nil, err
	}
	r, err := fs.ReadAt(path, offset, limit)
	if err != nil {
		return nil, err
	}
	io.Copy(resp, r)
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

	r, err := fs.ReadAt(path, int64(0), fileInfo.Size)
	if err != nil {
		return nil, err
	}
	io.Copy(resp, r)
	return nil, nil
}
