package agent

import (
	"encoding/json"
	"net/http"
	"sync"

	hclog "github.com/hashicorp/go-hclog"
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

		// Only return Prometheus formatted metrics if the user has enabled
		// this functionality.
		if !s.agent.GetConfig().Telemetry.PrometheusMetrics {
			return nil, CodedError(http.StatusUnsupportedMediaType, "Prometheus is not enabled")
		}
		s.prometheusHandler().ServeHTTP(resp, req)
		return nil, nil
	}

	return s.agent.GetMetricsSink().DisplayMetrics(resp, req)
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

// MetricsStreamRequest streams metrics back to the caller.
func (s *HTTPServer) MetricsStreamRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, CodedError(http.StatusMethodNotAllowed, ErrInvalidMethod)
	}

	//TODO: Fetch the ACL token, if any, and enforce agent policy.

	flusher, ok := resp.(http.Flusher)
	if !ok {
		return nil, CodedError(http.StatusForbidden, "streaming not supported")
	}

	resp.WriteHeader(http.StatusOK)

	// 0 byte write is needed before the Flush call so that if we are using
	// a gzip stream it will go ahead and write out the HTTP response header
	resp.Write([]byte(""))
	flusher.Flush()

	enc := metricsEncoder{
		logger:  s.logger,
		encoder: json.NewEncoder(resp),
		flusher: flusher,
	}
	enc.encoder.SetIndent("", "    ")
	s.agent.GetMetricsSink().Stream(req.Context(), enc)
	return nil, nil
}

type metricsEncoder struct {
	logger  hclog.Logger
	encoder *json.Encoder
	flusher http.Flusher
}

func (m metricsEncoder) Encode(summary interface{}) error {
	if err := m.encoder.Encode(summary); err != nil {

		m.logger.Error("failed to encode metrics summary", "error", err)
		return err
	}
	m.flusher.Flush()
	return nil
}
