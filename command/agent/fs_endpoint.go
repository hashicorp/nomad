package agent

import (
	"fmt"
	"net/http"
	"strconv"
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
	allocID := strings.TrimPrefix(req.URL.Path, "/v1/client/fs/stat/")
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
	allocID := strings.TrimPrefix(req.URL.Path, "/v1/client/fs/readat/")
	path := req.URL.Query().Get("path")
	ofs := req.URL.Query().Get("offset")
	if ofs == "" {
		ofs = "0"
	}

	offset, err := strconv.ParseInt(ofs, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing offset: %v", err)
	}
	lim := req.URL.Query().Get("limit")
	limit, err := strconv.ParseInt(lim, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing limit: %v", err)
	}

	if path == "" {
		resp.WriteHeader(http.StatusNotFound)
		return nil, fmt.Errorf("must provide a file name")
	}
	if allocID == "" {
		resp.WriteHeader(http.StatusNotFound)
		return nil, fmt.Errorf("alloc id not found")
	}
	if err = s.agent.client.FSReadAt(allocID, path, offset, limit, resp); err != nil {
		return nil, err
	}
	return nil, nil

}
