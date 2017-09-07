package agent

import (
	"net/http"
)

func (s *HTTPServer) MetricsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method == "GET" {
		return s.newMetricsRequest(resp, req)
	}
	return nil, CodedError(405, ErrInvalidMethod)
}

func (s *HTTPServer) newMetricsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return s.agent.InmemSink.DisplayMetrics(resp, req)
}
