package main

import (
	"github.com/hashicorp/nomad/api"
	"net/http"
)

func (v *v1api) getMetricsPaths() []*Path {
	tags := []string{"Metrics"}

	return []*Path{
		// TODO: Research what happens if request Prometheus. It
		// may require support for OneOf or AnyOf.
		//s.mux.HandleFunc("/v1/metrics", s.wrap(s.MetricsRequest))
		{
			Template: "/metrics",
			Operations: []*Operation{
				newOperation(http.MethodGet, "metricsSummaryRequest", tags, "GetMetricsSummary",
					nil,
					[]*Parameter{&MetricsSummaryFormatParam},
					newResponseConfig(200, objectSchema, api.MetricsSummary{}, nil, "GetMetricsSummaryResponse"),
				),
			},
		},
	}
}
