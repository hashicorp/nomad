package agent

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Only create the prometheus handler once
	promHandler http.Handler
	promOnce    sync.Once
)

// MetricsRequest returns metrics for the agent. Metrics are JSON by default
// but Prometheus is an optional format.
func (s *HTTPServer) MetricsRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(405, ErrInvalidMethod)
	}

	if format := req.URL.Query().Get("format"); format == "prometheus" {
		s.prometheusHandler().ServeHTTP(resp, req)
		return nil, nil
	}

	return s.agent.InmemSink.DisplayMetrics(resp, req)
}

func (s *HTTPServer) prometheusHandler() http.Handler {
	promOnce.Do(func() {
		handlerOptions := promhttp.HandlerOpts{
			ErrorLog:           s.logger.Named("prometheus_handler").StandardLogger(nil),
			ErrorHandling:      promhttp.ContinueOnError,
			DisableCompression: true,
		}

		promHandler = promhttp.HandlerFor(prometheus.DefaultGatherer, handlerOptions)
	})
	return promHandler
}
