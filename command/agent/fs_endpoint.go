package agent

import (
	"fmt"
	"net/http"
	"strings"
)

func (s *HTTPServer) DirectoryListRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	allocID := strings.TrimPrefix(req.URL.Path, "/v1/client/fs/ls/")
	path := req.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	if allocID == "" {
		resp.WriteHeader(http.StatusNotFound)
		return nil, fmt.Errorf("alloc id not found")
	}
	files, err := s.agent.client.FSList(allocID, path)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (s *HTTPServer) FileStatRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	allocID := strings.TrimPrefix(req.URL.Path, "/v1/client/fs/ls/")
	path := req.URL.Query().Get("path")
	if path == "" {
		resp.WriteHeader(http.StatusNotFound)
		return nil, fmt.Errorf("must provide a file name")
	}
	if allocID == "" {
		resp.WriteHeader(http.StatusNotFound)
		return nil, fmt.Errorf("alloc id not found")
	}
	fileInfo, err := s.agent.client.FSStat(allocID, path)
	if err != nil {
		return nil, err
	}
	return fileInfo, nil
}

func (s *HTTPServer) FileReadAtRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return nil, nil
}
