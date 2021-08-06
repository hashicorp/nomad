package main

import (
	"net/http"
)

func (v *v1api) getRegionsPaths() []*Path {
	tags := []string{"Regions"}

	return []*Path{
		//s.mux.HandleFunc("/v1/regions", s.wrap(s.RegionListRequest))
		{
			Template: "/regions",
			Operations: []*Operation{
				newOperation(http.MethodGet, "regionListRequest", tags, "GetRegions",
					nil,
					queryOptions,
					newResponseConfig(200, arraySchema, "string", queryMeta, "GetJobsResponse"),
				),
			},
		},
	}
}
