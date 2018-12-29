package agent

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsRequest returns metrics for the agent. Metrics are JSON by default
// but Prometheus is an optional format.
func (s *HTTPServer) MetricsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	if format := req.URL.Query().Get("format"); format == "prometheus" {
		handlerOptions := promhttp.HandlerOpts{
			ErrorLog:           s.logger,
			ErrorHandling:      promhttp.ContinueOnError,
			DisableCompression: true,
		}

		handler := promhttp.HandlerFor(prometheus.DefaultGatherer, handlerOptions)
		handler.ServeHTTP(resp, req)
		return nil, nil
	}

	return s.agent.InmemSink.DisplayMetrics(resp, req)
}
