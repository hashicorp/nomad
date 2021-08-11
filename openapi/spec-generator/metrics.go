package main

import (
	"github.com/hashicorp/nomad/api"
	"net/http"
)

func (v *v1api) getMetricsPaths() []*apiPath {
	tags := []string{"Metrics"}

	return []*apiPath{
		// TODO: Research what happens if request Prometheus. It
		// may require support for OneOf or AnyOf.
		//s.mux.HandleFunc("/v1/metrics", s.wrap(s.MetricsRequest))
		{
			Template: "/metrics",
			Operations: []*operation{
				newOperation(http.MethodGet, "metricsSummaryRequest", tags, "GetMetricsSummary",
					nil,
					[]*parameter{&metricsSummaryFormatParam},
					newResponseConfig(200, objectSchema, api.MetricsSummary{}, nil, "GetMetricsSummaryResponse"),
				),
			},
		},
	}
}
