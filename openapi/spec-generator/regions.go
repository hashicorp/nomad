package main

import (
	"net/http"
)

func (v *v1api) getRegionsPaths() []*apiPath {
	tags := []string{"Regions"}

	return []*apiPath{
		//s.mux.HandleFunc("/v1/regions", s.wrap(s.RegionListRequest))
		{
			Template: "/regions",
			Operations: []*operation{
				newOperation(http.MethodGet, "regionListRequest", tags, "GetRegions",
					nil,
					nil,
					newResponseConfig(200, arraySchema, "string", nil, "GetRegionsResponse"),
				),
			},
		},
	}
}
