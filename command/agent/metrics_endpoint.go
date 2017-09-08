package agent

import (
	"net/http"
)

// MetricsRequest returns metrics in JSON format
func (s *HTTPServer) MetricsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method == "GET" {
		return s.agent.InmemSink.DisplayMetrics(resp, req)
	}
	return nil, CodedError(405, ErrInvalidMethod)
}
